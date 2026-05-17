package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ipms/internal/models"
)

const xAPIBase = "https://api.twitter.com/2"

type XService struct {
	bearerToken string
	client      *http.Client
}

func NewXService(bearerToken string) *XService {
	return &XService{
		bearerToken: bearerToken,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *XService) IsConfigured() bool {
	return s.bearerToken != "" && s.bearerToken != "your_x_bearer_token_here"
}

// FetchProfile tries official API first, then falls back to free syndication scraper
func (s *XService) FetchProfile(username string) (*models.ScrapedProfile, error) {
	clean := strings.TrimPrefix(username, "@")

	// Method 1: Official X API v2 (if configured and credits available)
	if s.IsConfigured() {
		profile, err := s.fetchViaOfficialAPI(clean)
		if err == nil {
			return profile, nil
		}
		log.Printf("[X] Official API failed for @%s: %v — trying syndication fallback", clean, err)
	}

	// Method 2: Free syndication scraper (no API key needed)
	profile, err := s.FetchProfileViaSyndication(clean)
	if err != nil {
		return nil, fmt.Errorf("all X scraping methods failed for @%s: %v", clean, err)
	}
	return profile, nil
}

// fetchViaOfficialAPI uses the X API v2 with bearer token
func (s *XService) fetchViaOfficialAPI(username string) (*models.ScrapedProfile, error) {
	url := fmt.Sprintf("%s/users/by/username/%s?user.fields=id,name,username,description,public_metrics,verified,created_at,profile_image_url,location", xAPIBase, username)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.bearerToken)
	req.Header.Set("User-Agent", "IPMS/2.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("X API rate limit reached")
	}
	if resp.StatusCode == 402 {
		return nil, fmt.Errorf("X API credits depleted")
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("X API access forbidden")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("X API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Username    string `json:"username"`
			Description string `json:"description"`
			Verified    bool   `json:"verified"`
			CreatedAt   string `json:"created_at"`
			ProfilePic  string `json:"profile_image_url"`
			Location    string `json:"location"`
			Metrics     struct {
				Followers int64 `json:"followers_count"`
				Following int64 `json:"following_count"`
				Tweets    int64 `json:"tweet_count"`
				Listed    int64 `json:"listed_count"`
				Likes     int64 `json:"like_count"`
			} `json:"public_metrics"`
		} `json:"data"`
		Errors []struct{ Message string } `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("X API: %s", result.Errors[0].Message)
	}
	if result.Data.ID == "" {
		return nil, fmt.Errorf("user @%s not found on X", username)
	}

	u := result.Data
	joinedYear := ""
	if u.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, u.CreatedAt)
		if err == nil {
			joinedYear = fmt.Sprintf("%d", t.Year())
		}
	}

	initials := u.Name
	if len(initials) > 2 {
		initials = initials[:2]
	}
	initials = strings.ToUpper(initials)

	verified := 0
	if u.Verified {
		verified = 1
	}

	return &models.ScrapedProfile{
		Platform:        "X",
		Username:        u.Username,
		DisplayName:     u.Name,
		AvatarInitials:  initials,
		Followers:       u.Metrics.Followers,
		Following:       u.Metrics.Following,
		Posts:           u.Metrics.Tweets,
		Bio:             u.Description,
		Verified:        verified,
		JoinedYear:      joinedYear,
		ProfileImageURL: u.ProfilePic,
		ScrapedAt:       time.Now().Format(time.RFC3339),
		Method:          "x_api_v2",
	}, nil
}

// FetchProfileViaSyndication scrapes profile data from Twitter's free syndication endpoint
// No API key required — works even when official API credits are depleted
func (s *XService) FetchProfileViaSyndication(username string) (*models.ScrapedProfile, error) {
	url := "https://syndication.twitter.com/srv/timeline-profile/screen-name/" + username

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("syndication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("syndication returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Extract the JSON data from __NEXT_DATA__ script tag
	re := regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">(.+?)</script>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find profile data in syndication response")
	}

	var nextData struct {
		Props struct {
			PageProps struct {
				Timeline struct {
					Entries []struct {
						Content struct {
							Tweet struct {
								User struct {
									Name              string `json:"name"`
									ScreenName        string `json:"screen_name"`
									Description       string `json:"description"`
									FollowersCount    int64  `json:"followers_count"`
									FriendsCount      int64  `json:"friends_count"`
									StatusesCount     int64  `json:"statuses_count"`
									CreatedAt         string `json:"created_at"`
									ProfileImageURL   string `json:"profile_image_url_https"`
									ProfileBannerURL  string `json:"profile_banner_url"`
									IsBlueVerified    bool   `json:"is_blue_verified"`
									Verified          bool   `json:"verified"`
								} `json:"user"`
							} `json:"tweet"`
						} `json:"content"`
					} `json:"entries"`
				} `json:"timeline"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(matches[1]), &nextData); err != nil {
		return nil, fmt.Errorf("parse syndication JSON: %w", err)
	}

	entries := nextData.Props.PageProps.Timeline.Entries
	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries in syndication response for @%s", username)
	}

	// Get user data from the first tweet's user field
	u := entries[0].Content.Tweet.User
	if u.ScreenName == "" {
		return nil, fmt.Errorf("no user data found in syndication for @%s", username)
	}

	// Parse join date
	joinedYear := ""
	if u.CreatedAt != "" {
		// Twitter uses format: "Mon Jan 02 15:04:05 +0000 2006"
		t, err := time.Parse("Mon Jan 02 15:04:05 +0000 2006", u.CreatedAt)
		if err == nil {
			joinedYear = fmt.Sprintf("%d", t.Year())
		}
	}

	initials := u.Name
	if len(initials) > 2 {
		initials = initials[:2]
	}
	initials = strings.ToUpper(initials)

	verified := 0
	if u.IsBlueVerified || u.Verified {
		verified = 1
	}

	log.Printf("[X] ✅ Syndication scrape successful for @%s — %d followers", u.ScreenName, u.FollowersCount)

	return &models.ScrapedProfile{
		Platform:        "X",
		Username:        u.ScreenName,
		DisplayName:     u.Name,
		AvatarInitials:  initials,
		Followers:       u.FollowersCount,
		Following:       u.FriendsCount,
		Posts:           u.StatusesCount,
		Bio:             u.Description,
		Verified:        verified,
		JoinedYear:      joinedYear,
		ProfileImageURL: u.ProfileImageURL,
		ScrapedAt:       time.Now().Format(time.RFC3339),
		Method:          "syndication",
	}, nil
}

// FetchRecentTweets fetches last N tweets for the user
func (s *XService) FetchRecentTweets(username string, maxResults int) ([]*models.Tweet, error) {
	if !s.IsConfigured() {
		return nil, nil
	}

	clean := strings.TrimPrefix(username, "@")

	// First get user ID
	url := fmt.Sprintf("%s/users/by/username/%s?user.fields=id", xAPIBase, clean)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.bearerToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userResp struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&userResp)
	if userResp.Data.ID == "" {
		return nil, nil
	}

	if maxResults > 100 {
		maxResults = 100
	}
	if maxResults < 5 {
		maxResults = 5
	}

	// Fetch timeline
	turl := fmt.Sprintf("%s/users/%s/tweets?max_results=%d&tweet.fields=created_at,public_metrics,lang",
		xAPIBase, userResp.Data.ID, maxResults)

	treq, _ := http.NewRequest("GET", turl, nil)
	treq.Header.Set("Authorization", "Bearer "+s.bearerToken)

	tresp, err := s.client.Do(treq)
	if err != nil {
		return nil, err
	}
	defer tresp.Body.Close()

	var tweetsResp struct {
		Data []struct {
			ID      string `json:"id"`
			Text    string `json:"text"`
			Created string `json:"created_at"`
			Lang    string `json:"lang"`
			Metrics struct {
				Likes      int `json:"like_count"`
				Retweets   int `json:"retweet_count"`
				Replies    int `json:"reply_count"`
			} `json:"public_metrics"`
		} `json:"data"`
	}

	if err := json.NewDecoder(tresp.Body).Decode(&tweetsResp); err != nil {
		return nil, err
	}

	var tweets []*models.Tweet
	for _, t := range tweetsResp.Data {
		tweets = append(tweets, &models.Tweet{
			TweetID:   t.ID,
			Text:      t.Text,
			Likes:     t.Metrics.Likes,
			Retweets:  t.Metrics.Retweets,
			Replies:   t.Metrics.Replies,
			CreatedAt: t.Created,
		})
	}
	return tweets, nil
}

// TestConnection verifies the X API is working
func (s *XService) TestConnection() (bool, string) {
	if !s.IsConfigured() {
		return false, "Bearer token not configured"
	}
	_, err := s.FetchProfile("twitter")
	if err != nil {
		return false, err.Error()
	}
	return true, "ok"
}
