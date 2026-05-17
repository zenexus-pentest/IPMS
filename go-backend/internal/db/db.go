package db

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"ipms/internal/models"
)

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	d.seedMetrics()
	return d, nil
}

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
	CREATE TABLE IF NOT EXISTS profiles (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		username       TEXT NOT NULL,
		display_name   TEXT,
		platform       TEXT NOT NULL,
		avatar_initials TEXT,
		followers      INTEGER DEFAULT 0,
		following      INTEGER DEFAULT 0,
		posts          INTEGER DEFAULT 0,
		bio            TEXT DEFAULT '',
		verified       INTEGER DEFAULT 0,
		risk_level     TEXT DEFAULT 'low',
		joined_year    TEXT,
		is_active      INTEGER DEFAULT 1,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS snapshots (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_id     INTEGER NOT NULL,
		followers      INTEGER,
		following      INTEGER,
		posts          INTEGER,
		follower_delta INTEGER DEFAULT 0,
		recorded_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (profile_id) REFERENCES profiles(id)
	);
	CREATE TABLE IF NOT EXISTS tweets (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_id INTEGER NOT NULL,
		tweet_id   TEXT UNIQUE,
		text       TEXT,
		likes      INTEGER DEFAULT 0,
		retweets   INTEGER DEFAULT 0,
		replies    INTEGER DEFAULT 0,
		created_at TEXT,
		FOREIGN KEY (profile_id) REFERENCES profiles(id)
	);
	CREATE TABLE IF NOT EXISTS activity_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_id  INTEGER,
		username    TEXT,
		platform    TEXT,
		event_type  TEXT,
		description TEXT,
		severity    TEXT DEFAULT 'info',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS analysis_reports (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		profile_id      INTEGER NOT NULL,
		report_text     TEXT,
		bot_probability INTEGER DEFAULT 0,
		risk_verdict    TEXT,
		generated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (profile_id) REFERENCES profiles(id)
	);
	CREATE TABLE IF NOT EXISTS daily_metrics (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		date                 TEXT NOT NULL,
		total_profiles       INTEGER DEFAULT 0,
		high_risk_count      INTEGER DEFAULT 0,
		alerts_count         INTEGER DEFAULT 0,
		new_followers_total  INTEGER DEFAULT 0
	);`)
	return err
}

func (d *DB) seedMetrics() {
	var count int
	d.conn.QueryRow("SELECT COUNT(*) FROM daily_metrics").Scan(&count)
	if count > 0 {
		return
	}
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, day := range days {
		d.conn.Exec(
			"INSERT INTO daily_metrics (date,total_profiles,high_risk_count,alerts_count,new_followers_total) VALUES (?,?,?,?,?)",
			day, r.Intn(4)+2, r.Intn(3)+1, r.Intn(5)+1, r.Intn(9000)+1000,
		)
	}
	log.Println("✅ Daily metrics seeded")
}

// ─── Profiles ─────────────────────────────────────────────────────────────────
func (d *DB) GetProfiles(platform, risk, search string) ([]models.Profile, error) {
	q := "SELECT * FROM profiles WHERE is_active=1"
	args := []interface{}{}

	if platform != "" && platform != "all" {
		q += " AND LOWER(platform) LIKE ?"
		args = append(args, "%"+platform+"%")
	}
	if risk != "" && risk != "all" {
		q += " AND risk_level=?"
		args = append(args, risk)
	}
	if search != "" {
		q += " AND (LOWER(username) LIKE ? OR LOWER(display_name) LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}
	q += " ORDER BY CASE risk_level WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END"

	rows, err := d.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProfiles(rows)
}

func (d *DB) GetProfile(id int64) (*models.Profile, error) {
	row := d.conn.QueryRow("SELECT * FROM profiles WHERE id=?", id)
	p := &models.Profile{}
	err := row.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Platform, &p.AvatarInit,
		&p.Followers, &p.Following, &p.Posts, &p.Bio, &p.Verified, &p.RiskLevel,
		&p.JoinedYear, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (d *DB) GetProfileByUsername(username, platform string) (*models.Profile, error) {
	row := d.conn.QueryRow(
		"SELECT * FROM profiles WHERE LOWER(username)=LOWER(?) AND platform=?",
		username, platform,
	)
	p := &models.Profile{}
	err := row.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Platform, &p.AvatarInit,
		&p.Followers, &p.Following, &p.Posts, &p.Bio, &p.Verified, &p.RiskLevel,
		&p.JoinedYear, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (d *DB) CreateProfile(p *models.ScrapedProfile, riskLevel string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO profiles (username,display_name,platform,avatar_initials,followers,following,posts,bio,verified,risk_level,joined_year)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		p.Username, p.DisplayName, p.Platform, p.AvatarInitials,
		p.Followers, p.Following, p.Posts, p.Bio, p.Verified, riskLevel, p.JoinedYear,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateProfile(id, followers, following, posts int64, bio, riskLevel string) error {
	_, err := d.conn.Exec(
		"UPDATE profiles SET followers=?,following=?,posts=?,bio=?,risk_level=?,updated_at=CURRENT_TIMESTAMP WHERE id=?",
		followers, following, posts, bio, riskLevel, id,
	)
	return err
}

func (d *DB) UpdateRiskLevel(id int64, riskLevel string) error {
	_, err := d.conn.Exec(
		"UPDATE profiles SET risk_level=?,updated_at=CURRENT_TIMESTAMP WHERE id=?",
		riskLevel, id,
	)
	return err
}

func (d *DB) DeactivateProfile(id int64) error {
	_, err := d.conn.Exec("UPDATE profiles SET is_active=0 WHERE id=?", id)
	return err
}

// ─── Snapshots ────────────────────────────────────────────────────────────────
func (d *DB) InsertSnapshot(profileID, followers, following, posts, delta int64) error {
	_, err := d.conn.Exec(
		"INSERT INTO snapshots (profile_id,followers,following,posts,follower_delta) VALUES (?,?,?,?,?)",
		profileID, followers, following, posts, delta,
	)
	return err
}

func (d *DB) GetSnapshotsByProfile(profileID int64) ([]map[string]interface{}, error) {
	rows, err := d.conn.Query(
		`SELECT DATE(recorded_at) as day, AVG(followers) as avg_followers, SUM(follower_delta) as net_change
		 FROM snapshots WHERE profile_id=? GROUP BY DATE(recorded_at) ORDER BY day ASC LIMIT 30`,
		profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var day string
		var avg float64
		var net int64
		rows.Scan(&day, &avg, &net)
		result = append(result, map[string]interface{}{"day": day, "avg_followers": avg, "net_change": net})
	}
	return result, nil
}

// ─── Activity Log ─────────────────────────────────────────────────────────────
func (d *DB) InsertActivity(username, platform, eventType, description, severity string) error {
	_, err := d.conn.Exec(
		"INSERT INTO activity_log (username,platform,event_type,description,severity) VALUES (?,?,?,?,?)",
		username, platform, eventType, description, severity,
	)
	return err
}

func (d *DB) GetActivity(severity string, limit, offset int) ([]models.ActivityLog, int, error) {
	countQ := "SELECT COUNT(*) FROM activity_log"
	q := "SELECT id,profile_id,username,platform,event_type,description,severity,created_at FROM activity_log"
	args := []interface{}{}

	if severity != "" && severity != "all" {
		countQ += " WHERE severity=?"
		q += " WHERE severity=?"
		args = append(args, severity)
	}
	q += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var total int
	if severity != "" && severity != "all" {
		d.conn.QueryRow(countQ, severity).Scan(&total)
	} else {
		d.conn.QueryRow(countQ).Scan(&total)
	}

	rows, err := d.conn.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.ActivityLog
	for rows.Next() {
		var a models.ActivityLog
		var pid sql.NullInt64
		rows.Scan(&a.ID, &pid, &a.Username, &a.Platform, &a.EventType, &a.Description, &a.Severity, &a.CreatedAt)
		if pid.Valid {
			a.ProfileID = &pid.Int64
		}
		logs = append(logs, a)
	}
	return logs, total, nil
}

// ─── Analytics ────────────────────────────────────────────────────────────────
func (d *DB) GetDashboardStats() (*models.DashboardStats, error) {
	s := &models.DashboardStats{}

	d.conn.QueryRow("SELECT COUNT(*) FROM profiles WHERE is_active=1").Scan(&s.TotalProfiles)
	d.conn.QueryRow("SELECT COUNT(*) FROM profiles WHERE risk_level IN ('high','critical') AND is_active=1").Scan(&s.HighRisk)
	d.conn.QueryRow("SELECT COUNT(*) FROM profiles WHERE risk_level='critical' AND is_active=1").Scan(&s.CriticalCount)
	d.conn.QueryRow("SELECT COUNT(*) FROM activity_log WHERE severity IN ('alert','critical') AND DATE(created_at)=DATE('now')").Scan(&s.TodayAlerts)
	d.conn.QueryRow("SELECT COALESCE(SUM(followers),0) FROM profiles WHERE is_active=1").Scan(&s.TotalFollowers)
	d.conn.QueryRow("SELECT COUNT(*) FROM analysis_reports WHERE DATE(generated_at)=DATE('now')").Scan(&s.RecentReports)

	rows, err := d.conn.Query("SELECT risk_level, COUNT(*) as count FROM profiles WHERE is_active=1 GROUP BY risk_level")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rb models.RiskBreakdown
			rows.Scan(&rb.RiskLevel, &rb.Count)
			s.RiskBreakdown = append(s.RiskBreakdown, rb)
		}
	}
	return s, nil
}

func (d *DB) GetChartData() ([]models.DailyMetric, []map[string]interface{}, error) {
	rows, err := d.conn.Query("SELECT id,date,total_profiles,high_risk_count,alerts_count,new_followers_total FROM daily_metrics ORDER BY id ASC LIMIT 7")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var metrics []models.DailyMetric
	for rows.Next() {
		var m models.DailyMetric
		rows.Scan(&m.ID, &m.Date, &m.TotalProfiles, &m.HighRiskCount, &m.AlertsCount, &m.NewFollowersTotal)
		metrics = append(metrics, m)
	}

	pRows, err := d.conn.Query("SELECT platform, COUNT(*) as count, SUM(followers) as total_followers FROM profiles WHERE is_active=1 GROUP BY platform")
	if err != nil {
		return metrics, nil, nil
	}
	defer pRows.Close()
	var byPlatform []map[string]interface{}
	for pRows.Next() {
		var platform string
		var count int
		var total int64
		pRows.Scan(&platform, &count, &total)
		byPlatform = append(byPlatform, map[string]interface{}{"platform": platform, "count": count, "total_followers": total})
	}
	return metrics, byPlatform, nil
}

// ─── Analysis Reports ─────────────────────────────────────────────────────────
func (d *DB) InsertReport(profileID int64, reportText string, botProb int, riskVerdict string) (int64, error) {
	res, err := d.conn.Exec(
		"INSERT INTO analysis_reports (profile_id,report_text,bot_probability,risk_verdict) VALUES (?,?,?,?)",
		profileID, reportText, botProb, riskVerdict,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetReport(id int64) (*models.AnalysisReport, error) {
	row := d.conn.QueryRow("SELECT id,profile_id,report_text,bot_probability,risk_verdict,generated_at FROM analysis_reports WHERE id=?", id)
	r := &models.AnalysisReport{}
	err := row.Scan(&r.ID, &r.ProfileID, &r.ReportText, &r.BotProbability, &r.RiskVerdict, &r.GeneratedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (d *DB) GetReports() ([]models.AnalysisReport, error) {
	rows, err := d.conn.Query(
		`SELECT ar.id,ar.profile_id,ar.report_text,ar.bot_probability,ar.risk_verdict,ar.generated_at,
		        p.username,p.platform,p.display_name,p.risk_level
		 FROM analysis_reports ar JOIN profiles p ON ar.profile_id=p.id
		 ORDER BY ar.generated_at DESC LIMIT 20`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []models.AnalysisReport
	for rows.Next() {
		var r models.AnalysisReport
		rows.Scan(&r.ID, &r.ProfileID, &r.ReportText, &r.BotProbability, &r.RiskVerdict, &r.GeneratedAt,
			&r.Username, &r.Platform, &r.DisplayName, &r.RiskLevel)
		reports = append(reports, r)
	}
	return reports, nil
}

// ─── Tweets ───────────────────────────────────────────────────────────────────
func (d *DB) InsertTweet(t *models.Tweet) error {
	_, err := d.conn.Exec(
		"INSERT OR REPLACE INTO tweets (profile_id,tweet_id,text,likes,retweets,replies,created_at) VALUES (?,?,?,?,?,?,?)",
		t.ProfileID, t.TweetID, t.Text, t.Likes, t.Retweets, t.Replies, t.CreatedAt,
	)
	return err
}

// ─── Demo Data Seeding ────────────────────────────────────────────────────────
type demoProfile struct {
	Username    string
	DisplayName string
	Platform    string
	Followers   int64
	Following   int64
	Posts       int64
	Bio         string
	Verified    int
	RiskLevel   string
	JoinedYear  string
	BotProb     int
}

func (d *DB) SeedDemoData() {
	var count int
	d.conn.QueryRow("SELECT COUNT(*) FROM profiles").Scan(&count)
	if count > 0 {
		return // already seeded
	}

	profiles := []demoProfile{
		// ─── LOW RISK ─────────────────────────────────────────
		{"techguru_pk", "Tech Guru Pakistan", "X", 124500, 892, 3420,
			"🇵🇰 Tech blogger | AI & cybersecurity | Speaker | DMs open for collabs",
			1, "low", "2018", 5},
		{"sarahcodes", "Sarah Ahmed", "X", 45200, 1230, 890,
			"Software engineer @Google | Python & Go | Women in Tech 🚀",
			1, "low", "2019", 8},
		{"travel.pakistan", "Pakistan Travels", "Instagram", 89000, 450, 2100,
			"📸 Showcasing the beauty of Pakistan | Hunza | Swat | K2",
			1, "low", "2020", 3},
		{"dev_abdullah", "Abdullah Dev", "X", 18700, 620, 445,
			"Full-stack developer | Open source contributor | UAF alumni 🎓",
			0, "low", "2021", 12},

		// ─── MEDIUM RISK ──────────────────────────────────────
		{"crypto_alerts99", "Crypto Alerts", "X", 8900, 4200, 120,
			"📊 Real-time crypto signals | BTC ETH SOL | Join our Telegram!",
			0, "medium", "2023", 38},
		{"fashionista.pk", "Fashion Hub PK", "Instagram", 34500, 2800, 670,
			"Latest fashion trends 🛍️ | DM for promotions | Karachi based",
			0, "medium", "2022", 28},
		{"newz_bot_feed", "News Feed Auto", "X", 5200, 1800, 15200,
			"Automated news aggregator | Breaking headlines 24/7",
			0, "medium", "2022", 42},

		// ─── HIGH RISK ────────────────────────────────────────
		{"follow4follow_king", "Follow King", "X", 320, 5800, 12,
			"Follow me I follow back! 💯 DM for shoutouts",
			0, "high", "2024", 65},
		{"insta.growth.hacks", "Growth Hacks", "Instagram", 1200, 6500, 45,
			"🚀 Get 10K followers FAST! DM 'GROW' | Forex | Investment tips",
			0, "high", "2023", 72},

		// ─── CRITICAL RISK ────────────────────────────────────
		{"x_promo_deal2024", "PROMO DEALS", "X", 45, 7200, 3,
			"💰 Crypto investment guaranteed 500% returns! DM NOW! BTC ETH NFT forex",
			0, "critical", "2024", 95},
		{"spam_follower_net", "Free Followers", "Instagram", 88, 9100, 7,
			"Get FREE followers instantly! Click link in bio 🔗 crypto nft profit dm",
			0, "critical", "2025", 98},
		{"bot_network_node7", "Node Seven", "X", 15, 6300, 1,
			"",
			0, "critical", "2025", 99},
	}

	r := rand.New(rand.NewSource(42)) // fixed seed for reproducibility

	for _, p := range profiles {
		initials := ""
		if len(p.DisplayName) >= 2 {
			initials = strings.ToUpper(p.DisplayName[:2])
		}

		res, err := d.conn.Exec(
			`INSERT INTO profiles (username,display_name,platform,avatar_initials,followers,following,posts,bio,verified,risk_level,joined_year,created_at,updated_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,datetime('now',?),datetime('now',?))`,
			p.Username, p.DisplayName, p.Platform, initials,
			p.Followers, p.Following, p.Posts, p.Bio, p.Verified, p.RiskLevel, p.JoinedYear,
			fmt.Sprintf("-%d hours", r.Intn(168)+24),
			fmt.Sprintf("-%d hours", r.Intn(12)),
		)
		if err != nil {
			continue
		}
		profileID, _ := res.LastInsertId()

		// ─── 7 days of historical snapshots ──────────────────
		for day := 6; day >= 0; day-- {
			jitter := int64(r.Intn(800)) - 200
			delta := int64(0)
			if day < 6 {
				delta = jitter
			}
			d.conn.Exec(
				`INSERT INTO snapshots (profile_id,followers,following,posts,follower_delta,recorded_at)
				 VALUES (?,?,?,?,?,datetime('now',?))`,
				profileID,
				p.Followers+jitter,
				p.Following+int64(r.Intn(20)-10),
				p.Posts+int64(day),
				delta,
				fmt.Sprintf("-%d days", day),
			)
		}

		// ─── Activity log entries ────────────────────────────
		d.conn.Exec(
			`INSERT INTO activity_log (username,platform,event_type,description,severity,created_at) VALUES (?,?,?,?,?,datetime('now',?))`,
			p.Username, p.Platform, "added",
			fmt.Sprintf("@%s added via live scrape — %d followers", p.Username, p.Followers),
			"info", fmt.Sprintf("-%d hours", r.Intn(120)+48),
		)
		if p.RiskLevel == "critical" || p.RiskLevel == "high" {
			d.conn.Exec(
				`INSERT INTO activity_log (username,platform,event_type,description,severity,created_at) VALUES (?,?,?,?,?,datetime('now',?))`,
				p.Username, p.Platform, "risk_flag",
				fmt.Sprintf("%s risk detected for @%s", strings.ToUpper(p.RiskLevel), p.Username),
				p.RiskLevel, fmt.Sprintf("-%d hours", r.Intn(48)),
			)
		}
		d.conn.Exec(
			`INSERT INTO activity_log (username,platform,event_type,description,severity,created_at) VALUES (?,?,?,?,?,datetime('now',?))`,
			p.Username, p.Platform, "analysis",
			fmt.Sprintf("AI analysis complete — Bot probability: %d%%", p.BotProb),
			"info", fmt.Sprintf("-%d hours", r.Intn(24)),
		)

		// ─── Pre-generated AI report ─────────────────────────
		report := generateDemoReport(p)
		d.conn.Exec(
			`INSERT INTO analysis_reports (profile_id,report_text,bot_probability,risk_verdict,generated_at) VALUES (?,?,?,?,datetime('now',?))`,
			profileID, report, p.BotProb, strings.ToUpper(p.RiskLevel),
			fmt.Sprintf("-%d hours", r.Intn(24)),
		)
	}

	// ─── Extra activity events for realism ───────────────
	extraEvents := []struct{ user, platform, evType, desc, sev, ago string }{
		{"techguru_pk", "X", "follower_change", "Follower change: +1,240 for @techguru_pk", "info", "-2 hours"},
		{"travel.pakistan", "Instagram", "follower_change", "Follower change: +890 for @travel.pakistan", "info", "-5 hours"},
		{"x_promo_deal2024", "X", "follower_change", "Follower spike: +3,200 for @x_promo_deal2024 — possible bot network", "alert", "-1 hours"},
		{"spam_follower_net", "Instagram", "risk_flag", "CRITICAL risk escalated for @spam_follower_net — spam keywords detected", "critical", "-3 hours"},
		{"sarahcodes", "X", "refresh", "Live data refreshed for @sarahcodes — 45,200 followers", "info", "-8 hours"},
		{"crypto_alerts99", "X", "analysis", "Re-analysis triggered — risk level changed to MEDIUM", "info", "-6 hours"},
	}
	for _, e := range extraEvents {
		d.conn.Exec(
			`INSERT INTO activity_log (username,platform,event_type,description,severity,created_at) VALUES (?,?,?,?,?,datetime('now',?))`,
			e.user, e.platform, e.evType, e.desc, e.sev, e.ago,
		)
	}

	log.Println("✅ Demo data seeded — 12 profiles, snapshots, reports, and activity logs")
}

func generateDemoReport(p demoProfile) string {
	var followRatio string
	if p.Following > 0 {
		followRatio = fmt.Sprintf("%.2f", float64(p.Followers)/float64(p.Following))
	} else {
		followRatio = "∞"
	}

	var riskAdj string
	if p.RiskLevel == "critical" || p.RiskLevel == "high" {
		riskAdj = "multiple indicators of inauthentic or suspicious behavior"
	} else if p.RiskLevel == "medium" {
		riskAdj = "some patterns that warrant closer monitoring"
	} else {
		riskAdj = "patterns generally consistent with a legitimate account"
	}

	var threatLines string
	switch p.RiskLevel {
	case "critical":
		threatLines = "• Extreme follow/follower imbalance\n• Very few posts with high following count\n• Spam keywords detected in bio\n• Very new account (2024+)\n• No verification badge"
	case "high":
		threatLines = "• Significant follow/follower imbalance\n• Low posts, high following\n• Spam keywords in bio\n• Recently created account"
	case "medium":
		threatLines = "• Moderate follow/follower imbalance\n• Some promotional content in bio\n• Account age within suspicious range"
	default:
		threatLines = "• No significant threat indicators detected\n• Account metrics appear within normal parameters"
	}

	var recLines string
	switch p.RiskLevel {
	case "critical":
		recLines = "• IMMEDIATE ACTION: Flag for platform abuse reporting\n• Block and restrict account interactions\n• Document evidence for escalation"
	case "high":
		recLines = "• Schedule increased monitoring (every 30 min)\n• Cross-reference with known bot networks\n• Review recent post content for policy violations"
	case "medium":
		recLines = "• Monitor weekly for behavioral changes\n• Flag for review if follow ratio worsens\n• Check posting patterns over next 30 days"
	default:
		recLines = "• Continue routine monitoring (daily snapshots)\n• No immediate action required\n• Review in 30 days"
	}

	verdictDesc := map[string]string{
		"critical": "Multiple high-confidence automated behavior signals detected.",
		"high":     "Suspicious patterns warrant increased monitoring and review.",
		"medium":   "Some risk indicators present — monitor closely.",
		"low":      "Account displays characteristics consistent with authentic human operation.",
	}

	return fmt.Sprintf(`**EXECUTIVE SUMMARY**
Analysis of @%s on %s reveals %s. The account has %d followers with a follow ratio of %s. Risk score: %d/100.

**BEHAVIOUR ANALYSIS**
• Follow ratio: %s
• Total posts: %d
• Account age: Since %s
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
		p.Username, p.Platform, riskAdj, p.Followers, followRatio, p.BotProb,
		followRatio, p.Posts, p.JoinedYear,
		map[int]string{1: "Verified ✓", 0: "Not verified"}[p.Verified],
		map[bool]string{true: p.Bio, false: "(empty)"}[p.Bio != ""],
		threatLines, p.BotProb,
		map[bool]string{true: p.Bio, false: "(empty)"}[p.Bio != ""],
		map[bool]string{true: "Promotional/potentially spam content detected in bio.", false: "No immediate red flags in bio content."}[p.RiskLevel == "critical" || p.RiskLevel == "high"],
		recLines, strings.ToUpper(p.RiskLevel), verdictDesc[p.RiskLevel],
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────
func scanProfiles(rows *sql.Rows) ([]models.Profile, error) {
	var profiles []models.Profile
	for rows.Next() {
		var p models.Profile
		err := rows.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Platform, &p.AvatarInit,
			&p.Followers, &p.Following, &p.Posts, &p.Bio, &p.Verified, &p.RiskLevel,
			&p.JoinedYear, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}
