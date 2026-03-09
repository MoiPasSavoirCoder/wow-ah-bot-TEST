package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/models"
	"wow-ah-bot/internal/services/blizzard"
	"wow-ah-bot/internal/services/character"
	"wow-ah-bot/internal/services/portfolio"
	"wow-ah-bot/internal/services/scanner"
	"wow-ah-bot/internal/services/trading"
)

// RegisterRoutes sets up all API endpoints on the given router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/dashboard", getDashboard)
	rg.GET("/deals", getDeals)
	rg.POST("/deals/:id/execute", executeDeal)
	rg.POST("/deals/:id/skip", skipDeal)
	rg.GET("/portfolio", getPortfolio)
	rg.POST("/portfolio", addPortfolioEntry)
	rg.GET("/portfolio/inventory", getInventory)
	rg.GET("/portfolio/pnl", getPnl)
	rg.GET("/gold-history", getGoldHistory)
	rg.GET("/prices/:itemId", getPriceHistory)
	rg.GET("/items/search", searchItems)
	rg.GET("/scores", getItemScores)
	rg.GET("/scores/:itemId", getItemScoreHistory)
	// Characters
	rg.GET("/characters", listCharacters)
	rg.POST("/characters", createCharacter)
	rg.GET("/characters/:id", getCharacter)
	rg.PUT("/characters/:id", updateCharacter)
	rg.DELETE("/characters/:id", deleteCharacter)
	rg.GET("/characters/:id/pnl", getCharacterPnl)
	rg.GET("/characters/:id/transactions", getCharacterTransactions)
	rg.POST("/characters/:id/transactions", addCharacterTransaction)
	rg.GET("/characters/:id/snapshots", getCharacterSnapshots)
	rg.POST("/characters/:id/snapshots", addCharacterSnapshot)
	// Actions
	rg.POST("/scan", triggerScan)
	rg.POST("/analyze", triggerAnalyze)
	rg.POST("/refresh", refreshAll)
}

// ════════════════════════════════════════
// Dashboard
// ════════════════════════════════════════

func getDashboard(c *gin.Context) {
	db := database.DB
	pnl := portfolio.GetPnlSummary()

	var activeDeals int64
	db.Model(&models.Deal{}).Where("status = ?", "PENDING").Count(&activeDeals)

	var totalItems int64
	db.Model(&models.Item{}).Count(&totalItems)

	var lastScan *time.Time
	var snap models.AuctionSnapshot
	if db.Order("scanned_at DESC").First(&snap).Error == nil {
		lastScan = &snap.ScannedAt
	}

	goldHistory, _ := portfolio.GetGoldHistory(30)
	ghDTOs := make([]models.GoldBalanceDTO, len(goldHistory))
	for i, g := range goldHistory {
		ghDTOs[i] = models.NewGoldBalanceDTO(g)
	}

	var recentDeals []models.Deal
	db.Order("detected_at DESC").Limit(10).Find(&recentDeals)
	rdDTOs := make([]models.DealDTO, len(recentDeals))
	for i, d := range recentDeals {
		name, icon := resolveItem(d.ItemID, d.ItemName)
		rdDTOs[i] = models.NewDealDTO(d, name, icon)
	}

	inv, _ := portfolio.GetInventory()
	var invested int64
	for _, item := range inv {
		invested += item.AvgBuyPrice * int64(item.Quantity)
	}

	c.JSON(http.StatusOK, models.DashboardSummaryDTO{
		TotalInvestedGold:  models.CopperToGoldStr(invested),
		TotalProfitGold:    models.CopperToGoldStr(pnl.RealizedProfitCopper),
		CurrentBalanceGold: models.CopperToGoldStr(pnl.RealizedProfitCopper + invested),
		ActiveDeals:        int(activeDeals),
		TotalItemsTracked:  int(totalItems),
		LastScan:           lastScan,
		GoldHistory:        ghDTOs,
		RecentDeals:        rdDTOs,
	})
}

// ════════════════════════════════════════
// Deals
// ════════════════════════════════════════

func getDeals(c *gin.Context) {
	db := database.DB
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	// Query deals joined with items for name + icon, sorted by IR
	type row struct {
		models.Deal
		JoinedItemName string
		JoinedIconURL  string
	}
	var rows []row
	q := db.Table("deals").
		Select("deals.*, items.name as joined_item_name, items.icon_url as joined_icon_url").
		Joins("LEFT JOIN items ON items.id = deals.item_id").
		Order("deals.rentability_index DESC").
		Limit(limit)

	if status != "" {
		q = q.Where("deals.status = ?", strings.ToUpper(status))
	}
	q.Scan(&rows)

	dtos := make([]models.DealDTO, len(rows))
	for i, r := range rows {
		name := r.JoinedItemName
		if name == "" {
			name = r.Deal.ItemName
		}
		dtos[i] = models.NewDealDTO(r.Deal, name, r.JoinedIconURL)
	}
	c.JSON(http.StatusOK, dtos)
}

// ════════════════════════════════════════
// Item Scores (IR breakdown)
// ════════════════════════════════════════

// getItemScores returns the latest IR score for every item, sorted by rentability_index DESC.
func getItemScores(c *gin.Context) {
	db := database.DB
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	// Keep only the most recent score per item (MAX id trick)
	type row struct {
		models.ItemScore
		JoinedItemName string
		JoinedIconURL  string
	}
	var rows []row
	db.Table("item_scores").
		Select("item_scores.*, items.name as joined_item_name, items.icon_url as joined_icon_url").
		Joins("LEFT JOIN items ON items.id = item_scores.item_id").
		Where("item_scores.id IN (SELECT MAX(id) FROM item_scores GROUP BY item_id)").
		Order("item_scores.rentability_index DESC").
		Limit(limit).
		Scan(&rows)

	dtos := make([]models.ItemScoreDTO, len(rows))
	for i, r := range rows {
		name := r.JoinedItemName
		if name == "" {
			name = fmt.Sprintf("Item #%d", r.ItemScore.ItemID)
		}
		dtos[i] = models.NewItemScoreDTO(r.ItemScore, name, r.JoinedIconURL)
	}
	c.JSON(http.StatusOK, dtos)
}

// getItemScoreHistory returns all IR scores for a specific item (latest first).
func getItemScoreHistory(c *gin.Context) {
	db := database.DB
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item ID"})
		return
	}
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var scores []models.ItemScore
	db.Where("item_id = ?", itemID).
		Order("scored_at DESC").
		Limit(limit).
		Find(&scores)

	name, iconURL := resolveItem(itemID, "")

	dtos := make([]models.ItemScoreDTO, len(scores))
	for i, s := range scores {
		dtos[i] = models.NewItemScoreDTO(s, name, iconURL)
	}
	c.JSON(http.StatusOK, gin.H{
		"item_id":   itemID,
		"item_name": name,
		"icon_url":  iconURL,
		"history":   dtos,
		"weights":   models.DefaultIRWeights(),
	})
}

func executeDeal(c *gin.Context) {
	db := database.DB
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deal ID"})
		return
	}

	var deal models.Deal
	if db.First(&deal, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deal not found"})
		return
	}

	price := deal.SuggestedBuyPrice
	if price == 0 {
		price = deal.CurrentPrice
	}
	_, err = portfolio.AddTransaction(
		deal.ItemID, deal.ItemName, "BUY",
		deal.SuggestedQuantity, price,
		fmt.Sprintf("Deal #%d - IR: %.1f/100", deal.ID, deal.RentabilityIndex),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	db.Model(&deal).Update("status", "EXECUTED")
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Deal #%d executed and added to portfolio", id)})
}

func skipDeal(c *gin.Context) {
	db := database.DB
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deal ID"})
		return
	}
	result := db.Model(&models.Deal{}).Where("id = ?", id).Update("status", "SKIPPED")
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deal not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Deal #%d skipped", id)})
}

// ════════════════════════════════════════
// Portfolio
// ════════════════════════════════════════

func getPortfolio(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	txs, err := portfolio.GetTransactions(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dtos := make([]models.PortfolioDTO, len(txs))
	for i, t := range txs {
		dtos[i] = models.NewPortfolioDTO(t)
	}
	c.JSON(http.StatusOK, dtos)
}

func addPortfolioEntry(c *gin.Context) {
	var req models.PortfolioAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry, err := portfolio.AddTransaction(
		req.ItemID, req.ItemName, req.Action,
		req.Quantity, req.PricePerUnit, req.Notes,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.NewPortfolioDTO(*entry))
}

func getInventory(c *gin.Context) {
	inv, err := portfolio.GetInventory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, inv)
}

func getPnl(c *gin.Context) {
	pnl := portfolio.GetPnlSummary()
	c.JSON(http.StatusOK, models.PnlSummaryDTO{
		TotalInvested:        models.CopperToGoldStr(pnl.TotalInvestedCopper),
		TotalRevenue:         models.CopperToGoldStr(pnl.TotalRevenueCopper),
		AHFees:               models.CopperToGoldStr(pnl.AHFeesCopper),
		RealizedProfit:       models.CopperToGoldStr(pnl.RealizedProfitCopper),
		TotalInvestedCopper:  pnl.TotalInvestedCopper,
		TotalRevenueCopper:   pnl.TotalRevenueCopper,
		RealizedProfitCopper: pnl.RealizedProfitCopper,
	})
}

// ════════════════════════════════════════
// Gold History
// ════════════════════════════════════════

func getGoldHistory(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 || days > 365 {
		days = 30
	}
	history, err := portfolio.GetGoldHistory(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dtos := make([]models.GoldBalanceDTO, len(history))
	for i, g := range history {
		dtos[i] = models.NewGoldBalanceDTO(g)
	}
	c.JSON(http.StatusOK, dtos)
}

// ════════════════════════════════════════
// Price History
// ════════════════════════════════════════

func getPriceHistory(c *gin.Context) {
	db := database.DB
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item ID"})
		return
	}
	daysStr := c.DefaultQuery("days", "7")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 || days > 90 {
		days = 7
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	var item models.Item
	db.First(&item, itemID)

	var records []models.PriceHistory
	db.Where("item_id = ? AND scanned_at >= ?", itemID, cutoff).
		Order("scanned_at ASC").Find(&records)

	dtos := make([]models.PriceHistoryDTO, len(records))
	for i, r := range records {
		dtos[i] = models.NewPriceHistoryDTO(r)
	}

	c.JSON(http.StatusOK, models.PriceHistoryListDTO{
		ItemID:   itemID,
		ItemName: item.Name,
		History:  dtos,
	})
}

// ════════════════════════════════════════
// Items
// ════════════════════════════════════════

func searchItems(c *gin.Context) {
	db := database.DB
	q := c.Query("q")
	if len(q) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query must be at least 2 characters"})
		return
	}
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	var items []models.Item
	db.Where("name LIKE ?", "%"+q+"%").Limit(limit).Find(&items)

	dtos := make([]models.ItemDTO, len(items))
	for i, item := range items {
		dtos[i] = models.NewItemDTO(item)
	}
	c.JSON(http.StatusOK, dtos)
}

// ════════════════════════════════════════
// Actions
// ════════════════════════════════════════

// NotifyFunc is called after a refresh to notify Discord.
// Set by main.go to avoid circular imports.
var NotifyFunc func(totalAuctions, uniqueItems, newDeals int, durationSec float64, deals []models.Deal)

func triggerScan(c *gin.Context) {
	result, err := scanner.Scan()
	if err != nil || result == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":        "Scan completed",
		"total_auctions": result.TotalAuctions,
		"scanned_at":     result.ScannedAt.Format(time.RFC3339),
	})
}

func triggerAnalyze(c *gin.Context) {
	deals, err := trading.Analyze()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Analysis complete: %d deals found", len(deals)),
		"deals_count": len(deals),
	})
}

func refreshAll(c *gin.Context) {
	start := time.Now()

	result, err := scanner.Scan()
	if err != nil || result == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed"})
		return
	}

	deals, err := trading.Analyze()
	if err != nil {
		log.Printf("⚠️  Analysis error during refresh: %v", err)
	}
	duration := time.Since(start).Seconds()

	// Count unnotified deals
	var newDeals []models.Deal
	for _, d := range deals {
		if !d.Notified {
			newDeals = append(newDeals, d)
		}
	}

	// Notify Discord (non-blocking)
	if NotifyFunc != nil {
		go NotifyFunc(result.TotalAuctions, result.UniqueItems, len(newDeals), duration, newDeals)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Refresh complete",
		"total_auctions": result.TotalAuctions,
		"unique_items":   result.UniqueItems,
		"deals_count":    len(deals),
		"scanned_at":     result.ScannedAt.Format(time.RFC3339),
	})
}

// ── helper ──

func resolveItem(itemID int, fallbackName string) (name, iconURL string) {
	db := database.DB
	var item models.Item
	if db.First(&item, itemID).Error == nil && item.Name != "" {
		return item.Name, item.IconURL
	}
	// Not in cache — fetch from Blizzard API
	details, err := blizzard.GetItemWithDetails(itemID)
	if err == nil && details.Name != "" {
		// Cache for next time
		newItem := models.Item{
			ID: details.ID, Name: details.Name, Quality: details.Quality,
			ItemClass: details.ItemClass, ItemSubclass: details.ItemSubclass,
			Level: details.Level, IconURL: details.IconURL, VendorPrice: details.VendorPrice,
		}
		db.Save(&newItem)
		return details.Name, details.IconURL
	}
	if fallbackName != "" {
		return fallbackName, ""
	}
	return fmt.Sprintf("Item #%d", itemID), ""
}

// ════════════════════════════════════════
// Characters
// ════════════════════════════════════════

func listCharacters(c *gin.Context) {
	chars, err := character.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dtos := make([]models.CharacterDTO, len(chars))
	for i, ch := range chars {
		dtos[i] = models.NewCharacterDTO(ch)
	}
	c.JSON(http.StatusOK, dtos)
}

func createCharacter(c *gin.Context) {
	var req models.CharacterAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ch, err := character.Create(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.NewCharacterDTO(*ch))
}

func getCharacter(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	ch, err := character.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Character not found"})
		return
	}
	c.JSON(http.StatusOK, models.NewCharacterDTO(*ch))
}

func updateCharacter(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	var req models.CharacterUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ch, err := character.Update(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.NewCharacterDTO(*ch))
}

func deleteCharacter(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	if err := character.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Character not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Character #%d deleted", id)})
}

func getCharacterPnl(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	pnl, err := character.GetPnl(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Character not found"})
		return
	}
	c.JSON(http.StatusOK, pnl)
}

func getCharacterTransactions(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	txs, err := character.GetTransactions(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dtos := make([]models.CharacterTransactionDTO, len(txs))
	for i, t := range txs {
		dtos[i] = models.NewCharacterTransactionDTO(t)
	}
	c.JSON(http.StatusOK, dtos)
}

func addCharacterTransaction(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	var req models.CharacterTransactionAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Resolve icon_url if not provided
	if req.IconURL == "" && req.ItemID > 0 {
		_, req.IconURL = resolveItem(req.ItemID, req.ItemName)
		if req.ItemName == "" {
			req.ItemName, _ = resolveItem(req.ItemID, "")
		}
	}
	tx, err := character.AddTransaction(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.NewCharacterTransactionDTO(*tx))
}

func getCharacterSnapshots(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	daysStr := c.DefaultQuery("days", "30")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 || days > 365 {
		days = 30
	}
	snaps, err := character.GetSnapshots(id, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dtos := make([]models.CharacterSnapshotDTO, len(snaps))
	for i, s := range snaps {
		dtos[i] = models.NewCharacterSnapshotDTO(s)
	}
	c.JSON(http.StatusOK, dtos)
}

func addCharacterSnapshot(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}
	var req models.CharacterSnapshotAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	snap, err := character.AddSnapshot(id, req.BalanceCopper)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.NewCharacterSnapshotDTO(*snap))
}

// parseUintParam parses a named URL param as uint.
func parseUintParam(c *gin.Context, name string) (uint, error) {
	v, err := strconv.Atoi(c.Param(name))
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return uint(v), nil
}
