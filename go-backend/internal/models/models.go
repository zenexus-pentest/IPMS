package models

import "time"

// ─── Profile ──────────────────────────────────────────────────────────────────
type Profile struct {
	ID           int64     `json:"id"            db:"id"`
	Username     string    `json:"username"      db:"username"`
	DisplayName  string    `json:"display_name"  db:"display_name"`
	Platform     string    `json:"platform"      db:"platform"`
	AvatarInit   string    `json:"avatar_initials" db:"avatar_initials"`
	Followers    int64     `json:"followers"     db:"followers"`
	Following    int64     `json:"following"     db:"following"`
	Posts        int64     `json:"posts"         db:"posts"`
	Bio          string    `json:"bio"           db:"bio"`
	Verified     int       `json:"verified"      db:"verified"`
	RiskLevel    string    `json:"risk_level"    db:"risk_level"`
	JoinedYear   string    `json:"joined_year"   db:"joined_year"`
	IsActive     int       `json:"is_active"     db:"is_active"`
	CreatedAt    time.Time `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"    db:"updated_at"`
}

// ─── Snapshot ─────────────────────────────────────────────────────────────────
type Snapshot struct {
	ID            int64     `json:"id"              db:"id"`
	ProfileID     int64     `json:"profile_id"      db:"profile_id"`
	Followers     int64     `json:"followers"       db:"followers"`
	Following     int64     `json:"following"       db:"following"`
	Posts         int64     `json:"posts"           db:"posts"`
	FollowerDelta int64     `json:"follower_delta"  db:"follower_delta"`
	RecordedAt    time.Time `json:"recorded_at"     db:"recorded_at"`
}

// ─── ActivityLog ──────────────────────────────────────────────────────────────
type ActivityLog struct {
	ID          int64     `json:"id"          db:"id"`
	ProfileID   *int64    `json:"profile_id"  db:"profile_id"`
	Username    string    `json:"username"    db:"username"`
	Platform    string    `json:"platform"    db:"platform"`
	EventType   string    `json:"event_type"  db:"event_type"`
	Description string    `json:"description" db:"description"`
	Severity    string    `json:"severity"    db:"severity"`
	CreatedAt   time.Time `json:"created_at"  db:"created_at"`
}

// ─── AnalysisReport ───────────────────────────────────────────────────────────
type AnalysisReport struct {
	ID            int64     `json:"id"              db:"id"`
	ProfileID     int64     `json:"profile_id"      db:"profile_id"`
	ReportText    string    `json:"report_text"     db:"report_text"`
	BotProbability int      `json:"bot_probability" db:"bot_probability"`
	RiskVerdict   string    `json:"risk_verdict"    db:"risk_verdict"`
	GeneratedAt   time.Time `json:"generated_at"    db:"generated_at"`
	// Joined fields
	Username    string `json:"username,omitempty"`
	Platform    string `json:"platform,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	RiskLevel   string `json:"risk_level,omitempty"`
}

// ─── DailyMetric ──────────────────────────────────────────────────────────────
type DailyMetric struct {
	ID                int64  `json:"id"                  db:"id"`
	Date              string `json:"date"                db:"date"`
	TotalProfiles     int    `json:"total_profiles"      db:"total_profiles"`
	HighRiskCount     int    `json:"high_risk_count"     db:"high_risk_count"`
	AlertsCount       int    `json:"alerts_count"        db:"alerts_count"`
	NewFollowersTotal int64  `json:"new_followers_total" db:"new_followers_total"`
}

// ─── Tweet ────────────────────────────────────────────────────────────────────
type Tweet struct {
	ID        int64  `json:"id"         db:"id"`
	ProfileID int64  `json:"profile_id" db:"profile_id"`
	TweetID   string `json:"tweet_id"   db:"tweet_id"`
	Text      string `json:"text"       db:"text"`
	Likes     int    `json:"likes"      db:"likes"`
	Retweets  int    `json:"retweets"   db:"retweets"`
	Replies   int    `json:"replies"    db:"replies"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

// ─── API Request/Response types ───────────────────────────────────────────────
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Total   int         `json:"total,omitempty"`
	Cached  bool        `json:"cached,omitempty"`
}

type ScrapeRequest struct {
	Username string `json:"username" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

type ManualProfileRequest struct {
	Username    string `json:"username"     binding:"required"`
	Platform    string `json:"platform"     binding:"required"`
	DisplayName string `json:"display_name"`
	Followers   int64  `json:"followers"`
	Following   int64  `json:"following"`
	Posts       int64  `json:"posts"`
	Bio         string `json:"bio"`
	Verified    bool   `json:"verified"`
	JoinedYear  string `json:"joined_year"`
	RiskLevel   string `json:"risk_level"`
}

type RiskUpdateRequest struct {
	RiskLevel string `json:"risk_level" binding:"required"`
}

// ─── Scraped profile data (internal) ─────────────────────────────────────────
type ScrapedProfile struct {
	Platform        string `json:"platform"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	AvatarInitials  string `json:"avatar_initials"`
	Followers       int64  `json:"followers"`
	Following       int64  `json:"following"`
	Posts           int64  `json:"posts"`
	Bio             string `json:"bio"`
	Verified        int    `json:"verified"`
	JoinedYear      string `json:"joined_year"`
	ProfileImageURL string `json:"profile_image_url"`
	ScrapedAt       string `json:"scraped_at"`
	Method          string `json:"method"`
	NeedsManual     bool   `json:"needs_manual,omitempty"`
	Note            string `json:"note,omitempty"`
	Partial         bool   `json:"partial,omitempty"`
}

// ─── Risk analysis result ─────────────────────────────────────────────────────
type RiskResult struct {
	Score     int      `json:"score"`
	RiskLevel string   `json:"risk_level"`
	Flags     []string `json:"flags"`
}

// ─── AI Analysis result ───────────────────────────────────────────────────────
type AIResult struct {
	ReportText     string `json:"report_text"`
	BotProbability int    `json:"bot_probability"`
	RiskVerdict    string `json:"risk_verdict"`
	RiskScore      int    `json:"risk_score"`
	RiskFlags      []string `json:"risk_flags"`
	Simulated      bool   `json:"simulated"`
}

// ─── Stats for dashboard ──────────────────────────────────────────────────────
type DashboardStats struct {
	TotalProfiles   int              `json:"totalProfiles"`
	HighRisk        int              `json:"highRisk"`
	CriticalCount   int              `json:"criticalCount"`
	TodayAlerts     int              `json:"todayAlerts"`
	TotalFollowers  int64            `json:"totalFollowers"`
	RecentReports   int              `json:"recentReports"`
	RiskBreakdown   []RiskBreakdown  `json:"riskBreakdown"`
}

type RiskBreakdown struct {
	RiskLevel string `json:"risk_level" db:"risk_level"`
	Count     int    `json:"count"      db:"count"`
}

// ─── WebSocket message ────────────────────────────────────────────────────────
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	TS   string      `json:"ts"`
}
