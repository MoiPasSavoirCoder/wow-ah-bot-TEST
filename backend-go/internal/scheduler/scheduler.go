package scheduler

import (
	"log"
	"time"

	"github.com/go-co-op/gocron"

	"wow-ah-bot/internal/config"
	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/discord"
	"wow-ah-bot/internal/models"
	"wow-ah-bot/internal/services/ai"
	"wow-ah-bot/internal/services/scanner"
	"wow-ah-bot/internal/services/trading"
)

var sched *gocron.Scheduler

// Start launches the periodic AH scan + analyze job.
func Start(bot *discord.Bot) {
	cfg := config.Cfg
	sched = gocron.NewScheduler(time.UTC)

	sched.Every(cfg.ScanIntervalMinutes).Minutes().Do(func() {
		scanAndAnalyze(bot)
	})

	sched.StartAsync()
	log.Printf("📅 Scheduler started: scanning every %d minutes", cfg.ScanIntervalMinutes)
}

// Stop shuts down the scheduler.
func Stop() {
	if sched != nil {
		sched.Stop()
		log.Println("Scheduler stopped")
	}
}

func scanAndAnalyze(bot *discord.Bot) {
	log.Println("⏰ Scheduled scan starting...")

	start := time.Now()

	result, err := scanner.Scan()
	if err != nil || result == nil {
		log.Printf("⚠️  Scheduled scan failed: %v", err)
		return
	}

	deals, err := trading.Analyze()
	if err != nil {
		log.Printf("⚠️  Scheduled analysis failed: %v", err)
	}

	// Run AI simulator after analysis
	if err := ai.SimulateTrades(); err != nil {
		log.Printf("⚠️  AI simulation failed: %v", err)
	}

	duration := time.Since(start).Seconds()

	// Filter unnotified deals
	var unnotified []models.Deal
	for _, d := range deals {
		if !d.Notified {
			unnotified = append(unnotified, d)
		}
	}

	// Discord notifications
	bot.SendScanReport(result.TotalAuctions, result.UniqueItems, len(unnotified), duration)
	if len(unnotified) > 0 {
		bot.SendDealsSummary(unnotified)

		// Mark as notified
		db := database.DB
		for _, d := range unnotified {
			if d.ID > 0 {
				db.Model(&models.Deal{}).Where("id = ?", d.ID).Update("notified", true)
			}
		}
	}

	log.Printf("✅ Scheduled scan complete: %d auctions, %d items, %d new deals in %.1fs",
		result.TotalAuctions, result.UniqueItems, len(unnotified), duration)
}
