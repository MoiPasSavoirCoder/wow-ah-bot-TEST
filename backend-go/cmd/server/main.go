package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"wow-ah-bot/internal/api"
	"wow-ah-bot/internal/config"
	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/discord"
	"wow-ah-bot/internal/models"
	"wow-ah-bot/internal/scheduler"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("")

	log.Println("🚀 WoW AH Trading Bot (Go) starting...")

	// 1. Load config
	config.Init()
	cfg := config.Cfg

	// 2. Init database
	database.Init()

	// 3. Start Discord bot
	bot := discord.New()
	go func() {
		if err := bot.Start(); err != nil {
			log.Printf("❌ Discord bot error: %v", err)
		}
	}()

	// 4. Start scheduler
	scheduler.Start(bot)

	// 5. Wire Discord notify into API routes (avoids circular imports)
	api.NotifyFunc = func(totalAuctions, uniqueItems, newDeals int, durationSec float64, deals []models.Deal) {
		bot.SendScanReport(totalAuctions, uniqueItems, newDeals, durationSec)
		if len(deals) > 0 {
			bot.SendDealsSummary(deals)
		}
	}

	// 6. Setup Gin HTTP server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// CORS for Angular frontend
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL, "http://localhost:4200"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Root
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":    "WoW AH Trading Bot",
			"version": "2.0.0",
			"runtime": "Go",
			"realm":   fmt.Sprintf("%s (%s)", cfg.BlizzardRealmSlug, cfg.BlizzardRegion),
			"docs":    "/api",
		})
	})

	// API routes
	apiGroup := r.Group("/api")
	api.RegisterRoutes(apiGroup)

	// 7. Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("⏹️  Shutting down...")
		scheduler.Stop()
		bot.Stop()
		os.Exit(0)
	}()

	// 8. Start listening
	addr := fmt.Sprintf("%s:%d", cfg.BackendHost, cfg.BackendPort)
	log.Printf("🌐 API available at http://localhost:%d", cfg.BackendPort)
	log.Printf("🎮 Realm: %s (%s)", cfg.BlizzardRealmSlug, cfg.BlizzardRegion)

	if err := r.Run(addr); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
