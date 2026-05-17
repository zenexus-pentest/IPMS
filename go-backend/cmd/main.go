package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"ipms/config"
	"ipms/internal/db"
	"ipms/internal/handlers"
	"ipms/internal/services"
)

func main() {
	// Load config
	cfg := config.Load()

	// Init database
	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("❌ Database init failed: %v", err)
	}
	log.Println("✅ SQLite database ready")

	// Seed demo data if DEMO_MODE is enabled
	if cfg.DemoMode {
		database.SeedDemoData()
	}

	// Init WebSocket hub
	hub := services.NewHub()

	// Init handlers
	h := handlers.New(database, cfg, hub)

	// Setup Gin router
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS — allow all origins (frontend served from same Go server)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
	}))

	// ─── Routes ───────────────────────────────────────────────────────────────
	api := r.Group("/api")
	{
		api.GET("/health", h.Health)

		// WebSocket
		r.GET("/ws", h.WebSocket)

		// Profiles
		p := api.Group("/profiles")
		{
			p.GET("",          h.GetProfiles)
			p.GET("/:id",      h.GetProfile)
			p.PATCH("/:id/risk", h.UpdateRisk)
			p.DELETE("/:id",   h.DeleteProfile)
		}

		// Analytics
		a := api.Group("/analytics")
		{
			a.GET("/stats",          h.GetStats)
			a.GET("/chart",          h.GetChart)
			a.GET("/activity",       h.GetActivity)
			a.GET("/snapshots/:id",  h.GetSnapshots)
		}

		// Scraping
		s := api.Group("/scrape")
		{
			s.POST("/fetch",      h.ScrapeFetch)
			s.POST("/analyze",    h.ScrapeAnalyze)
			s.POST("/refresh/:id",h.ScrapeRefresh)
			s.POST("/manual",     h.ScrapeManual)
			s.GET("/status",      h.ScrapeStatus)
		}

		// AI Analysis
		an := api.Group("/analysis")
		{
			an.POST("/profile/:id", h.AnalyzeProfile)
			an.GET("/reports",      h.GetReports)
		}
	}

	// ─── Auto-refresh cron every 10 minutes ───────────────────────────────────
	c := cron.New()
	c.AddFunc("*/10 * * * *", func() {
		if cfg.DemoMode {
			return // skip auto-refresh in demo mode — no API credits
		}

		profiles, err := database.GetProfiles("", "", "")
		if err != nil || len(profiles) == 0 {
			return
		}

		xSvc := services.NewXService(cfg.XBearerToken)
		igSvc := services.NewInstagramService(cfg.RapidAPIKey)

		for _, p := range profiles {
			platform := strings.ToUpper(p.Platform)
			var freshFollowers, freshFollowing, freshPosts int64
			var freshBio string

			if platform == "X" && xSvc.IsConfigured() {
				sp, err := xSvc.FetchProfile(p.Username)
				if err != nil {
					log.Printf("[CRON] Failed to refresh @%s (X): %v", p.Username, err)
					continue
				}
				freshFollowers = sp.Followers
				freshFollowing = sp.Following
				freshPosts     = sp.Posts
				freshBio       = sp.Bio
			} else if platform == "INSTAGRAM" {
				sp, err := igSvc.FetchProfile(p.Username)
				if err != nil || sp.NeedsManual {
					continue
				}
				freshFollowers = sp.Followers
				freshFollowing = sp.Following
				freshPosts     = sp.Posts
				freshBio       = sp.Bio
			} else {
				continue
			}

			delta := freshFollowers - p.Followers
			database.UpdateProfile(p.ID, freshFollowers, freshFollowing, freshPosts, freshBio, p.RiskLevel)
			database.InsertSnapshot(p.ID, freshFollowers, freshFollowing, freshPosts, delta)

			hub.Broadcast("profile_update", map[string]interface{}{
				"id": p.ID, "username": p.Username, "platform": p.Platform,
				"followers": freshFollowers, "delta": delta,
			})

			if absInt64(delta) > 500 {
				sign := "+"
				if delta < 0 {
					sign = ""
				}
				desc := fmt.Sprintf("Follower change: %s%d for @%s", sign, delta, p.Username)
				sev := "info"
				if absInt64(delta) > 5000 {
					sev = "alert"
				}
				database.InsertActivity(p.Username, p.Platform, "follower_change", desc, sev)
				hub.Broadcast("alert", map[string]interface{}{"username": p.Username, "description": desc})
			}
		}
		log.Printf("[CRON] Refreshed %d profiles", len(profiles))
	})
	c.Start()

	// ─── Serve pre-built React frontend ─────────────────────────────────────────
	distPath := cfg.DistPath
	r.Static("/assets", distPath+"/assets")
	r.StaticFile("/favicon.svg", distPath+"/favicon.svg")
	r.StaticFile("/icons.svg", distPath+"/icons.svg")
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if !strings.HasPrefix(p, "/api") && !strings.HasPrefix(p, "/ws") {
			c.File(distPath + "/index.html")
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	// ─── Print startup banner ─────────────────────────────────────────────────
	xStatus := "❌ Add X_BEARER_TOKEN to .env"
	if cfg.HasXAPI() {
		xStatus = "✅ Ready"
	}
	aiStatus := "❌ Add GROK_API_KEY to .env"
	if cfg.HasGrokAPI() {
		aiStatus = "✅ Grok Ready"
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║   IPMS — Intelligent Profile Monitoring System v2.0  ║")
	fmt.Println("║   Muhammad Abdullah Mujahid | 2022-AG-6620 | UAF      ║")
	fmt.Println("╠══════════════════════════════════════════════════════╣")
	fmt.Printf( "║  🟢 HTTP:      http://localhost:%s                   ║\n", cfg.Port)
	fmt.Printf( "║  🔌 WebSocket: ws://localhost:%s/ws                  ║\n", cfg.Port)
	fmt.Printf( "║  🐦 X API:     %-36s  ║\n", xStatus)
	fmt.Printf( "║  🤖 Claude AI: %-36s  ║\n", aiStatus)
	fmt.Println("║  📷 Instagram: ✅ oEmbed + Manual mode active         ║")
	fmt.Println("║  ⚡ Runtime:   Go (fast, single binary)               ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	// ─── Start server ──────────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	log.Printf("Server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
