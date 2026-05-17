package services

import (
	"strings"

	"ipms/internal/models"
)

var spamKeywords = []string{
	"dm for", "crypto", "nft", "investment", "100x",
	"profit", "forex", "trading signal", "onlyfans",
}

// AutoDetectRisk scores a scraped profile 0-100 and returns risk level + flags
func AutoDetectRisk(p *models.ScrapedProfile) *models.RiskResult {
	score := 0
	flags := []string{}

	var followRatio float64
	if p.Following > 0 {
		followRatio = float64(p.Followers) / float64(p.Following)
	} else {
		followRatio = float64(p.Followers)
	}

	// Follow ratio
	if p.Following > 5000 && followRatio < 0.1 {
		score += 40
		flags = append(flags, "Extreme follow/follower imbalance")
	} else if p.Following > 2000 && followRatio < 0.2 {
		score += 25
		flags = append(flags, "High follow/follower imbalance")
	} else if p.Following > 500 && followRatio < 0.5 {
		score += 15
		flags = append(flags, "Low follower ratio")
	}

	// Empty bio
	if strings.TrimSpace(p.Bio) == "" {
		score += 15
		flags = append(flags, "Empty bio")
	}

	// Few posts vs many followers
	if p.Posts < 5 && p.Followers > 100 {
		score += 20
		flags = append(flags, "Very few posts vs follower count")
	} else if p.Posts < 20 && p.Following > 1000 {
		score += 15
		flags = append(flags, "Low posts, high following")
	}

	// Not verified
	if p.Verified == 0 {
		score += 5
	}

	// New account
	if p.JoinedYear >= "2024" && p.JoinedYear != "" {
		score += 20
		flags = append(flags, "Very new account (2024+)")
	} else if p.JoinedYear >= "2023" && p.JoinedYear != "" {
		score += 10
		flags = append(flags, "Recently created account")
	}

	// Spam keywords in bio
	bioLower := strings.ToLower(p.Bio)
	for _, kw := range spamKeywords {
		if strings.Contains(bioLower, kw) {
			score += 10
			flags = append(flags, "Spam keyword in bio: "+kw)
		}
	}

	// Determine level
	level := "low"
	switch {
	case score >= 70:
		level = "critical"
	case score >= 45:
		level = "high"
	case score >= 20:
		level = "medium"
	}

	if score > 100 {
		score = 100
	}

	return &models.RiskResult{
		Score:     score,
		RiskLevel: level,
		Flags:     flags,
	}
}
