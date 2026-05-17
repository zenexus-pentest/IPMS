package services

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"ipms/internal/models"
)

type InstagramService struct {
	rapidAPIKey string
	client      *http.Client
}

func NewInstagramService(rapidAPIKey string) *InstagramService {
	return &InstagramService{
		rapidAPIKey: rapidAPIKey,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *InstagramService) HasRapidAPI() bool {
	return s.rapidAPIKey != "" && s.rapidAPIKey != "your_rapidapi_key_here"
}

var igUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
}

func randomUserAgent() string {
	return igUserAgents[rand.Intn(len(igUserAgents))]
}

// FetchProfile tries multiple methods to get Instagram profile data
func (s *InstagramService) FetchProfile(username string) (*models.ScrapedProfile, error) {
	clean := strings.ToLower(strings.TrimPrefix(username, "@"))

	// Method 1: RapidAPI (best, if key configured)
	if s.HasRapidAPI() {
		if p, err := s.tryRapidAPI(clean); err == nil {
			return p, nil
		}
	}

	// Method 2: Official oEmbed (verifies account exists, limited data)
	if p, err := s.tryOEmbed(clean); err == nil {
		return p, nil
	}

	// Method 3: All automated methods failed — return manual entry signal
	return &models.ScrapedProfile{
		Platform:    "Instagram",
		Username:    clean,
		DisplayName: clean,
		NeedsManual: true,
		Note:        "Instagram blocks automated scraping. Please enter profile stats manually.",
		ScrapedAt:   time.Now().Format(time.RFC3339),
		Method:      "manual",
	}, nil
}

// tryRapidAPI calls a RapidAPI Instagram scraper
func (s *InstagramService) tryRapidAPI(username string) (*models.ScrapedProfile, error) {
	url := "https://instagram-scraper-api2.p.rapidapi.com/v1/info?username_or_id_or_url=" + username

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("x-rapidapi-key", s.rapidAPIKey)
	req.Header.Set("x-rapidapi-host", "instagram-scraper-api2.p.rapidapi.com")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("RapidAPI status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			Username   string `json:"username"`
			FullName   string `json:"full_name"`
			Biography  string `json:"biography"`
			Followers  int64  `json:"follower_count"`
			Following  int64  `json:"following_count"`
			MediaCount int64  `json:"media_count"`
			IsVerified bool   `json:"is_verified"`
			ProfilePic string `json:"profile_pic_url"`
			IsPrivate  bool   `json:"is_private"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	d := result.Data
	if d.Username == "" {
		return nil, fmt.Errorf("no user data in RapidAPI response")
	}

	initials := strings.ToUpper(d.FullName)
	if len(initials) > 2 {
		initials = initials[:2]
	}
	if initials == "" {
		initials = strings.ToUpper(username[:2])
	}

	verified := 0
	if d.IsVerified {
		verified = 1
	}

	return &models.ScrapedProfile{
		Platform:        "Instagram",
		Username:        d.Username,
		DisplayName:     d.FullName,
		AvatarInitials:  initials,
		Followers:       d.Followers,
		Following:       d.Following,
		Posts:           d.MediaCount,
		Bio:             d.Biography,
		Verified:        verified,
		ProfileImageURL: d.ProfilePic,
		ScrapedAt:       time.Now().Format(time.RFC3339),
		Method:          "rapidapi",
	}, nil
}

// tryOEmbed uses Instagram's official oEmbed endpoint (limited: name only, no follower counts)
func (s *InstagramService) tryOEmbed(username string) (*models.ScrapedProfile, error) {
	url := "https://api.instagram.com/oembed/?url=https://www.instagram.com/" + username + "/&format=json"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", randomUserAgent())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("oEmbed status %d", resp.StatusCode)
	}

	var result struct {
		AuthorName   string `json:"author_name"`
		ThumbnailURL string `json:"thumbnail_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.AuthorName == "" {
		return nil, fmt.Errorf("no author name in oEmbed response")
	}

	initials := strings.ToUpper(result.AuthorName)
	if len(initials) > 2 {
		initials = initials[:2]
	}

	return &models.ScrapedProfile{
		Platform:        "Instagram",
		Username:        username,
		DisplayName:     result.AuthorName,
		AvatarInitials:  initials,
		Followers:       0,
		Following:       0,
		Posts:           0,
		Verified:        0,
		ProfileImageURL: result.ThumbnailURL,
		ScrapedAt:       time.Now().Format(time.RFC3339),
		Method:          "oembed",
		Partial:         true,
		Note:            "Account verified. Add RAPIDAPI_KEY to .env for full follower stats.",
	}, nil
}

func (s *InstagramService) TestConnection() (bool, string) {
	if s.HasRapidAPI() {
		return true, "rapidapi configured"
	}
	return true, "oembed+manual mode"
}
