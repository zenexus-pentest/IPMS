package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	GrokAPIKey          string
	XBearerToken       string
	XAPIKey            string
	XAPISecret         string
	XAccessToken       string
	XAccessSecret      string
	RapidAPIKey        string
	DBPath             string
	DistPath           string
	Env                string
	DemoMode           bool
}

func Load() *Config {
	// Load .env file (ignore error if not present)
	_ = godotenv.Load()

	return &Config{
		Port:            getEnv("PORT", "5000"),
		GrokAPIKey:      getEnv("GROK_API_KEY", ""),
		XBearerToken:    getEnv("X_BEARER_TOKEN", ""),
		XAPIKey:         getEnv("X_API_KEY", ""),
		XAPISecret:      getEnv("X_API_SECRET", ""),
		XAccessToken:    getEnv("X_ACCESS_TOKEN", ""),
		XAccessSecret:   getEnv("X_ACCESS_SECRET", ""),
		RapidAPIKey:     getEnv("RAPIDAPI_KEY", ""),
		DBPath:          getEnv("DB_PATH", "./ipms.db"),
		DistPath:        getEnv("DIST_PATH", "../frontend/dist"),
		Env:             getEnv("NODE_ENV", "development"),
		DemoMode:        getEnv("DEMO_MODE", "false") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *Config) HasXAPI() bool {
	return c.XBearerToken != "" && c.XBearerToken != "your_x_bearer_token_here"
}

func (c *Config) HasGrokAPI() bool {
	return c.GrokAPIKey != "" && c.GrokAPIKey != "your_grok_api_key_here"
}

func (c *Config) HasRapidAPI() bool {
	return c.RapidAPIKey != "" && c.RapidAPIKey != "your_rapidapi_key_here"
}
