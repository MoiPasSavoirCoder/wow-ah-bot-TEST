package discord

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"wow-ah-bot/internal/config"
	"wow-ah-bot/internal/models"
)

// Bot manages the Discord connection and message sending.
type Bot struct {
	session   *discordgo.Session
	channelID string
	ready     chan struct{}
	once      sync.Once
}

// New creates a new Discord bot (does not connect yet).
func New() *Bot {
	return &Bot{ready: make(chan struct{})}
}

// Start connects the bot and waits for the Ready event.
func (b *Bot) Start() error {
	cfg := config.Cfg
	if cfg.DiscordBotToken == "" {
		log.Println("⚠️  Discord bot token not set, bot disabled")
		return nil
	}

	sess, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return fmt.Errorf("discord session: %w", err)
	}

	sess.Identify.Intents = discordgo.IntentsGuildMessages

	sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("🤖 Discord bot connected as %s", r.User.Username)
		b.once.Do(func() { close(b.ready) })
	})

	b.session = sess
	b.channelID = cfg.DiscordChannelID

	if err := sess.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}

	log.Println("📢 Discord bot started")
	return nil
}

// Stop gracefully closes the Discord session.
func (b *Bot) Stop() {
	if b.session != nil {
		b.session.Close()
	}
}

// waitReady blocks until the bot is connected (max 10s).
func (b *Bot) waitReady() bool {
	if b.session == nil || b.channelID == "" {
		return false
	}
	select {
	case <-b.ready:
		return true
	case <-time.After(10 * time.Second):
		log.Println("⚠️  Discord bot not ready after 10s")
		return false
	}
}

// ════════════════════════════════════════
// Public message methods
// ════════════════════════════════════════

// SendScanReport sends a scan summary embed.
func (b *Bot) SendScanReport(auctionsCount, itemsCount, newDealsCount int, durationSec float64) {
	if !b.waitReady() {
		return
	}

	color := 0x5865F2 // blurple
	title := "🔍 Scan terminé — Aucune nouvelle opportunité"
	if newDealsCount > 0 {
		color = 0x57F287 // green
		plural := ""
		if newDealsCount > 1 {
			plural = "s"
		}
		title = fmt.Sprintf("✅ Scan terminé — %d opportunité%s trouvée%s !", newDealsCount, plural, plural)
	}

	cfg := config.Cfg
	embed := &discordgo.MessageEmbed{
		Title:     title,
		Color:     color,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "📦 Enchères analysées", Value: fmt.Sprintf("**%d**", auctionsCount), Inline: true},
			{Name: "🧩 Items uniques", Value: fmt.Sprintf("**%d**", itemsCount), Inline: true},
			{Name: "💡 Nouvelles opportunités", Value: fmt.Sprintf("**%d**", newDealsCount), Inline: true},
			{Name: "⏱️ Durée du scan", Value: fmt.Sprintf("**%.1fs**", durationSec), Inline: true},
			{Name: "⏰ Prochain scan", Value: fmt.Sprintf("Dans **%d min**", cfg.ScanIntervalMinutes), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: "WoW AH Bot • Archimonde EU"},
	}

	if _, err := b.session.ChannelMessageSendEmbed(b.channelID, embed); err != nil {
		log.Printf("❌ Discord scan report failed: %v", err)
	}
}

// SendDealsSummary sends a summary header + individual deal embeds (max 10).
func (b *Bot) SendDealsSummary(deals []models.Deal) {
	if !b.waitReady() || len(deals) == 0 {
		return
	}

	// Header
	header := &discordgo.MessageEmbed{
		Title:       "🏦 WoW AH Trading Bot — Nouvelles Opportunités !",
		Description: fmt.Sprintf("**%d** deal(s) détecté(s) à %s", len(deals), time.Now().UTC().Format("15:04 UTC")),
		Color:       0xFEE75C, // gold
		Footer:      &discordgo.MessageEmbedFooter{Text: "Serveur Archimonde (EU) • Données mises à jour toutes les heures"},
	}
	if _, err := b.session.ChannelMessageSendEmbed(b.channelID, header); err != nil {
		log.Printf("❌ Discord header failed: %v", err)
		return
	}

	limit := 10
	if len(deals) < limit {
		limit = len(deals)
	}
	for _, d := range deals[:limit] {
		embed := buildDealEmbed(d)
		if _, err := b.session.ChannelMessageSendEmbed(b.channelID, embed); err != nil {
			log.Printf("❌ Discord deal embed failed: %v", err)
		}
		time.Sleep(500 * time.Millisecond) // rate-limit respect
	}

	if len(deals) > 10 {
		msg := fmt.Sprintf("... et **%d** autres deals. Consultez le dashboard pour la liste complète ! 📊", len(deals)-10)
		b.session.ChannelMessageSend(b.channelID, msg)
	}
}

// ════════════════════════════════════════
// Deal embed builder
// ════════════════════════════════════════

func buildDealEmbed(d models.Deal) *discordgo.MessageEmbed {
	// IR-based color: green gradient
	color := irColor(d.RentabilityIndex)
	irBar := irProgressBar(d.RentabilityIndex)

	name := d.ItemName
	if name == "" {
		name = fmt.Sprintf("Item #%d", d.ItemID)
	}

	totalCost := d.SuggestedBuyPrice * int64(d.SuggestedQuantity)
	profit := (d.SuggestedSellPrice - d.SuggestedBuyPrice) * int64(d.SuggestedQuantity) * 95 / 100
	if profit < 0 {
		profit = 0
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("🛒 ACHETER : %s", name),
		Color:     color,
		Timestamp: d.DetectedAt.Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "📊 Indice de Rentabilité", Value: fmt.Sprintf("%s **%.1f / 100**", irBar, d.RentabilityIndex), Inline: false},
			{Name: "💰 Prix actuel", Value: models.CopperToGoldStr(d.CurrentPrice), Inline: true},
			{Name: "📈 Prix médian (7j)", Value: models.CopperToGoldStr(d.AvgPrice), Inline: true},
			{Name: "📉 Marge estimée", Value: fmt.Sprintf("**%.1f%%**", d.ProfitMargin), Inline: true},
			{
				Name: "🛒 Action recommandée",
				Value: fmt.Sprintf("Acheter **%dx** à ≤ %s\nRevendre à ~ %s",
					d.SuggestedQuantity,
					models.CopperToGoldStr(d.SuggestedBuyPrice),
					models.CopperToGoldStr(d.SuggestedSellPrice)),
				Inline: false,
			},
			{Name: "💵 Investissement", Value: models.CopperToGoldStr(totalCost), Inline: true},
			{Name: "💎 Profit potentiel", Value: models.CopperToGoldStr(profit), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Item ID: %d • Deal #%d • IR basé sur 5 critères", d.ItemID, d.ID)},
	}

	return embed
}

// irColor returns a Discord embed color based on the IR score.
func irColor(ir float64) int {
	switch {
	case ir >= 80:
		return 0x57F287 // vivid green
	case ir >= 60:
		return 0xFEE75C // gold
	case ir >= 40:
		return 0xEB459E // pink
	default:
		return 0x5865F2 // blurple
	}
}

// irProgressBar returns a text progress bar for the IR score.
func irProgressBar(ir float64) string {
	filled := int(ir / 10)
	if filled > 10 {
		filled = 10
	}
	bar := ""
	for i := 0; i < 10; i++ {
		if i < filled {
			bar += "🟩"
		} else {
			bar += "⬜"
		}
	}
	return bar
}
