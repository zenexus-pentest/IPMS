package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ipms/internal/models"
)

// Grok API is OpenAI-compatible — same format, different URL + model name
const grokURL = "https://api.x.ai/v1/chat/completions"

type AIService struct {
	apiKey string
	client *http.Client
}

func NewAIService(apiKey string) *AIService {
	return &AIService{
		apiKey: apiKey,
		client: &http.Client{Timeout: 40 * time.Second},
	}
}

func (s *AIService) HasKey() bool {
	return s.apiKey != "" &&
		s.apiKey != "your_grok_api_key_here" &&
		s.apiKey != "your_anthropic_api_key_here"
}

// GenerateReport calls Grok API (or falls back to rule-based offline report)
func (s *AIService) GenerateReport(p *models.ScrapedProfile, risk *models.RiskResult) *models.AIResult {
	var followRatio string
	if p.Following > 0 {
		followRatio = fmt.Sprintf("%.2f", float64(p.Followers)/float64(p.Following))
	} else {
		followRatio = "∞"
	}

	if !s.HasKey() || p.Method == "demo_mode" {
		return s.offlineReport(p, risk, followRatio)
	}

	prompt := buildPrompt(p, risk, followRatio)
	text, err := s.callGrok(prompt)
	if err != nil {
		result := s.offlineReport(p, risk, followRatio)
		result.ReportText += "\n\n⚠ Grok API unavailable: " + err.Error() + ". This is a rule-based report."
		return result
	}

	botProb := extractBotProb(text, risk.Score)
	verdict := extractVerdict(text, risk.RiskLevel)

	return &models.AIResult{
		ReportText:     text,
		BotProbability: botProb,
		RiskVerdict:    strings.ToUpper(verdict),
		RiskScore:      risk.Score,
		RiskFlags:      risk.Flags,
		Simulated:      false,
	}
}

// callGrok uses the xAI Grok API (OpenAI-compatible format)
func (s *AIService) callGrok(prompt string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model": "grok-3-mini",   // free tier model
		"messages": []map[string]string{
			{"role": "system", "content": "You are IPMS, a professional cybersecurity threat intelligence analyst specializing in social media bot detection."},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  1200,
		"temperature": 0.3, // lower = more consistent structured output
	})

	req, _ := http.NewRequest("POST", grokURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(raw))
	}

	// OpenAI-compatible response format
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("Grok error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from Grok")
	}
	return result.Choices[0].Message.Content, nil
}

func buildPrompt(p *models.ScrapedProfile, risk *models.RiskResult, followRatio string) string {
	flagsStr := "None detected"
	if len(risk.Flags) > 0 {
		flagsStr = strings.Join(risk.Flags, "; ")
	}
	return fmt.Sprintf(`Analyze the following social media profile data and generate a professional threat intelligence report.

PROFILE DATA
Platform:        %s
Username:        @%s
Display Name:    %s
Followers:       %d
Following:       %d
Follow Ratio:    %s
Total Posts:     %d
Account Since:   %s
Verified:        %s
Bio:             "%s"
Auto Risk Score: %d/100
Detected Flags:  %s

Generate a structured report with EXACTLY these 7 sections.
Use ** around section headers. Reference the real numbers above.

**EXECUTIVE SUMMARY**
**BEHAVIOUR ANALYSIS**
**THREAT INDICATORS**
**BOT PROBABILITY SCORE**
Bot Probability: XX%%
**SENTIMENT ANALYSIS**
**RECOMMENDATIONS**
**FINAL RISK VERDICT**
%s — one sentence justification.`,
		p.Platform, p.Username, p.DisplayName,
		p.Followers, p.Following, followRatio, p.Posts,
		p.JoinedYear, map[int]string{1: "Yes ✓", 0: "No"}[p.Verified],
		p.Bio, risk.Score, flagsStr, strings.ToUpper(risk.RiskLevel),
	)
}

func extractBotProb(text string, fallback int) int {
	re := regexp.MustCompile(`Bot Probability:\s*(\d+)%`)
	m := re.FindStringSubmatch(text)
	if len(m) >= 2 {
		var v int
		fmt.Sscanf(m[1], "%d", &v)
		return v
	}
	return fallback
}

func extractVerdict(text, fallback string) string {
	re := regexp.MustCompile(`(?i)FINAL RISK VERDICT[:\s*\n]+([A-Z]+)`)
	m := re.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return strings.ToUpper(fallback)
}

func (s *AIService) offlineReport(p *models.ScrapedProfile, risk *models.RiskResult, followRatio string) *models.AIResult {
	var riskAdj string
	if risk.Score >= 45 {
		riskAdj = "multiple indicators of inauthentic or suspicious behavior"
	} else {
		riskAdj = "patterns generally consistent with a legitimate account"
	}

	flagLines := "• No significant threat indicators detected\n• Account metrics appear within normal parameters"
	if len(risk.Flags) > 0 {
		var sb strings.Builder
		for _, f := range risk.Flags {
			sb.WriteString("• " + f + "\n")
		}
		flagLines = sb.String()
	}

	var ratioCmt string
	if followRatio != "∞" {
		var r float64
		fmt.Sscanf(followRatio, "%f", &r)
		if r < 0.2 {
			ratioCmt = "critically low, strong bot/spam indicator"
		} else if r < 1.0 {
			ratioCmt = "below average"
		} else {
			ratioCmt = "healthy"
		}
	}

	var recLines string
	switch risk.RiskLevel {
	case "critical":
		recLines = "• IMMEDIATE ACTION: Flag for platform abuse reporting\n• Block and restrict account interactions\n• Document evidence for escalation"
	case "high":
		recLines = "• Schedule increased monitoring (every 30 min)\n• Cross-reference with known bot networks\n• Review recent post content for policy violations"
	default:
		recLines = "• Continue routine monitoring (daily snapshots)\n• No immediate action required\n• Review in 30 days"
	}

	report := fmt.Sprintf(`**EXECUTIVE SUMMARY**
Analysis of @%s on %s reveals %s. The account has %d followers with a follow ratio of %s. Risk score: %d/100.

**BEHAVIOUR ANALYSIS**
• Follow ratio: %s — %s
• Total posts: %d — %s
• Account age: %s
• Verification: %s
• Bio: %s

**THREAT INDICATORS**
%s
**BOT PROBABILITY SCORE**
Bot Probability: %d%%
Score based on: follow ratio, bio signals, account age, post activity.

**SENTIMENT ANALYSIS**
Bio: "%s"
%s

**RECOMMENDATIONS**
%s

**FINAL RISK VERDICT**
%s — %s`,
		p.Username, p.Platform, riskAdj, p.Followers, followRatio, risk.Score,
		followRatio, ratioCmt,
		p.Posts, map[bool]string{true: "unusually low activity", false: "normal activity level"}[p.Posts < 20],
		map[bool]string{true: "Since " + p.JoinedYear, false: "Unknown"}[p.JoinedYear != ""],
		map[int]string{1: "Verified ✓", 0: "Not verified"}[p.Verified],
		map[bool]string{true: p.Bio, false: "(empty)"}[p.Bio != ""],
		flagLines,
		risk.Score,
		map[bool]string{true: p.Bio, false: "(empty)"}[p.Bio != ""],
		map[bool]string{true: "Promotional/potentially spam content detected in bio.", false: "No immediate red flags in bio content."}[strings.ContainsAny(strings.ToLower(p.Bio), "crypto invest profit dm forex")],
		recLines,
		strings.ToUpper(risk.RiskLevel),
		map[string]string{
			"critical": "Multiple high-confidence automated behavior signals detected.",
			"high":     "Suspicious patterns warrant increased monitoring and review.",
			"medium":   "Some risk indicators present — monitor closely.",
			"low":      "Account displays characteristics consistent with authentic human operation.",
		}[risk.RiskLevel],
	)

	return &models.AIResult{
		ReportText:     report,
		BotProbability: risk.Score,
		RiskVerdict:    strings.ToUpper(risk.RiskLevel),
		RiskScore:      risk.Score,
		RiskFlags:      risk.Flags,
		Simulated:      true,
	}
}
