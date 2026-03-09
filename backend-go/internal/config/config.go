package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
)

// Settings holds all application configuration.
type Settings struct {
	// Blizzard API
	BlizzardClientID        string
	BlizzardClientSecret    string
	BlizzardRegion          string
	BlizzardRealmSlug       string
	BlizzardConnectedRealmID int
	BlizzardLocale          string

	// Discord
	DiscordBotToken  string
	DiscordChannelID string

	// Database
	DatabasePath string

	// Server
	BackendHost string
	BackendPort int
	FrontendURL string

	// Trading
	MinProfitMargin      float64
	MaxTrackedItems      int
	MaxBudgetGold        int
	ScanIntervalMinutes  int
}

// Computed URLs
func (s *Settings) BlizzardAPIBaseURL() string {
	return "https://" + s.BlizzardRegion + ".api.blizzard.com"
}

func (s *Settings) BlizzardTokenURL() string {
	return "https://oauth.battle.net/token"
}

func (s *Settings) BlizzardAHURL() string {
	return s.BlizzardAPIBaseURL() + "/data/wow/connected-realm/" +
		strconv.Itoa(s.BlizzardConnectedRealmID) + "/auctions"
}

func (s *Settings) BlizzardCommoditiesURL() string {
	return s.BlizzardAPIBaseURL() + "/data/wow/auctions/commodities"
}

// Global singleton
var Cfg *Settings

func Init() {
	// Find .env: try backend-go root first, then parent (WOW/) directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	envPath := filepath.Join(projectRoot, ".env")
	if err := godotenv.Load(envPath); err != nil {
		// Fallback: parent directory (e.g., WOW/.env)
		parentEnv := filepath.Join(projectRoot, "..", ".env")
		_ = godotenv.Load(parentEnv)
	}

	Cfg = &Settings{
		BlizzardClientID:        getEnv("BLIZZARD_CLIENT_ID", ""),
		BlizzardClientSecret:    getEnv("BLIZZARD_CLIENT_SECRET", ""),
		BlizzardRegion:          getEnv("BLIZZARD_REGION", "eu"),
		BlizzardRealmSlug:       getEnv("BLIZZARD_REALM_SLUG", "archimonde"),
		BlizzardConnectedRealmID: getEnvInt("BLIZZARD_CONNECTED_REALM_ID", 1302),
		BlizzardLocale:          getEnv("BLIZZARD_LOCALE", "fr_FR"),

		DiscordBotToken:  getEnv("DISCORD_BOT_TOKEN", ""),
		DiscordChannelID: getEnv("DISCORD_CHANNEL_ID", ""),

		DatabasePath: getEnv("DATABASE_PATH", filepath.Join(projectRoot, "data", "wow_ah.db")),

		BackendHost: getEnv("BACKEND_HOST", "0.0.0.0"),
		BackendPort: getEnvInt("BACKEND_PORT", 8000),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:4200"),

		MinProfitMargin:     getEnvFloat("MIN_PROFIT_MARGIN", 20),
		MaxTrackedItems:     getEnvInt("MAX_TRACKED_ITEMS", 500),
		MaxBudgetGold:       getEnvInt("MAX_BUDGET_GOLD", 500000),
		ScanIntervalMinutes: getEnvInt("SCAN_INTERVAL_MINUTES", 60),
	}

	log.Printf("⚙️  Config loaded: realm=%s region=%s scan_interval=%dm",
		Cfg.BlizzardRealmSlug, Cfg.BlizzardRegion, Cfg.ScanIntervalMinutes)
}

// ── helpers ──

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
