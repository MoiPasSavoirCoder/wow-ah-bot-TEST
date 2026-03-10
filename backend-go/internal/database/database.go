package database

import (
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wow-ah-bot/internal/config"
	"wow-ah-bot/internal/models"
)

// DB is the global database connection.
var DB *gorm.DB

// Init opens the SQLite database and auto-migrates all models.
func Init() {
	// Ensure data directory exists
	dir := filepath.Dir(config.Cfg.DatabasePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("❌ Cannot create data dir %s: %v", dir, err)
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(config.Cfg.DatabasePath+"?_busy_timeout=30000&_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Database open failed: %v", err)
	}

	// Auto-migrate all tables
	if err := DB.AutoMigrate(
		&models.Item{},
		&models.AuctionSnapshot{},
		&models.AuctionEntry{},
		&models.PriceHistory{},
		&models.Deal{},
		&models.ItemScore{},
		&models.Portfolio{},
		&models.GoldBalance{},
		&models.Character{},
		&models.CharacterTransaction{},
		&models.CharacterSnapshot{},
		&models.AITrade{},
		&models.AIPortfolioSnapshot{},
	); err != nil {
		log.Fatalf("❌ Auto-migrate failed: %v", err)
	}

	log.Println("✅ Database initialized (SQLite + WAL)")
}
