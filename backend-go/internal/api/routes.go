package api

import (
	"fmt"
	"log"
	"math"
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
	"wow-ah-bot/internal/services/ai"
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
	// Auction House browse
	rg.GET("/auction-house", getAuctionHouse)
	rg.GET("/auction-house/categories", getAuctionHouseCategories)
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
	// AI Trading Simulator
	rg.GET("/ai/stats", getAIStats)
	rg.GET("/ai/holdings", getAIHoldings)
	rg.GET("/ai/trades", getAITrades)
	rg.GET("/ai/snapshots", getAISnapshots)
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

	// Run AI simulator
	if err := ai.SimulateTrades(); err != nil {
		log.Printf("⚠️  AI simulation error during refresh: %v", err)
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

// ════════════════════════════════════════
// Auction House Browse
// ════════════════════════════════════════

// AuctionHouseItemDTO represents one item listing on the AH (aggregated from latest snapshot).
type AuctionHouseItemDTO struct {
	ItemID       int    `json:"item_id"`
	ItemName     string `json:"item_name"`
	IconURL      string `json:"icon_url"`
	Quality      string `json:"quality"`
	ItemClass    string `json:"item_class"`
	ItemSubclass string `json:"item_subclass"`
	Level        int    `json:"level"`
	MinBuyout    int64  `json:"min_buyout"`
	MinBuyoutGold string `json:"min_buyout_gold"`
	AvgBuyout    int64  `json:"avg_buyout"`
	AvgBuyoutGold string `json:"avg_buyout_gold"`
	MarketPrice  int64  `json:"market_price"`
	MarketPriceGold string `json:"market_price_gold"`
	TotalQuantity int   `json:"total_quantity"`
	NumAuctions  int    `json:"num_auctions"`
	TimeLeft     string `json:"time_left"`
}

func getAuctionHouse(c *gin.Context) {
	db := database.DB

	// Get latest snapshot
	var snapshot models.AuctionSnapshot
	if db.Order("scanned_at DESC").First(&snapshot).Error != nil {
		c.JSON(http.StatusOK, []AuctionHouseItemDTO{})
		return
	}

	// Params
	search := c.Query("search")
	category := c.Query("category")
	subcategory := c.Query("subcategory")
	quality := c.Query("quality")
	sortBy := c.DefaultQuery("sort", "name")
	sortDir := c.DefaultQuery("dir", "asc")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "50")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 { page = 1 }
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 10 { pageSize = 10 }
	if pageSize > 200 { pageSize = 200 }

	// Aggregate from auction_entries for this snapshot, join with items table
	type row struct {
		ItemID       int
		ItemName     string
		IconURL      string
		Quality      string
		ItemClass    string
		ItemSubclass string
		Level        int
		MinPrice     int64
		AvgPrice     int64
		TotalQty     int
		NumAuctions  int
	}

	q := db.Table("auction_entries ae").
		Select(`ae.item_id,
			COALESCE(i.name, '') as item_name,
			COALESCE(i.icon_url, '') as icon_url,
			COALESCE(i.quality, '') as quality,
			COALESCE(i.item_class, '') as item_class,
			COALESCE(i.item_subclass, '') as item_subclass,
			COALESCE(i.level, 0) as level,
			MIN(COALESCE(NULLIF(ae.unit_price, 0), ae.buyout)) as min_price,
			CAST(AVG(COALESCE(NULLIF(ae.unit_price, 0), ae.buyout)) AS INTEGER) as avg_price,
			COALESCE(SUM(ae.quantity), 0) as total_qty,
			COUNT(*) as num_auctions`).
		Joins("LEFT JOIN items i ON i.id = ae.item_id").
		Where("ae.snapshot_id = ?", snapshot.ID).
		Group("ae.item_id")

	// Filters
	if search != "" {
		q = q.Where("i.name LIKE ?", "%"+search+"%")
	}
	if category != "" {
		q = q.Where("i.item_class = ?", category)
	}
	if subcategory != "" {
		q = q.Where("i.item_subclass = ?", subcategory)
	}
	if quality != "" {
		q = q.Where("i.quality = ?", quality)
	}

	// Count total for pagination
	var total int64
	countQ := db.Table("(?) as sub", q)
	countQ.Count(&total)

	// Sort
	orderClause := "item_name ASC"
	switch sortBy {
	case "price":
		orderClause = "min_price"
	case "quantity":
		orderClause = "total_qty"
	case "level":
		orderClause = "level"
	case "auctions":
		orderClause = "num_auctions"
	default:
		orderClause = "item_name"
	}
	if sortDir == "desc" {
		orderClause += " DESC"
	} else {
		orderClause += " ASC"
	}
	q = q.Order(orderClause)

	// Paginate
	offset := (page - 1) * pageSize
	q = q.Offset(offset).Limit(pageSize)

	var rows []row
	q.Scan(&rows)

	// Build DTOs
	items := make([]AuctionHouseItemDTO, len(rows))
	for i, r := range rows {
		name := r.ItemName
		if name == "" {
			name = fmt.Sprintf("Item #%d", r.ItemID)
		}
		items[i] = AuctionHouseItemDTO{
			ItemID:        r.ItemID,
			ItemName:      name,
			IconURL:       r.IconURL,
			Quality:       r.Quality,
			ItemClass:     r.ItemClass,
			ItemSubclass:  r.ItemSubclass,
			Level:         r.Level,
			MinBuyout:     r.MinPrice,
			MinBuyoutGold: models.CopperToGoldStr(r.MinPrice),
			AvgBuyout:     r.AvgPrice,
			AvgBuyoutGold: models.CopperToGoldStr(r.AvgPrice),
			MarketPrice:   r.MinPrice,
			MarketPriceGold: models.CopperToGoldStr(r.MinPrice),
			TotalQuantity: r.TotalQty,
			NumAuctions:   r.NumAuctions,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		"snapshot_id": snapshot.ID,
		"scanned_at":  snapshot.ScannedAt,
	})
}

func getAuctionHouseCategories(c *gin.Context) {
	db := database.DB

	// Get latest snapshot
	var snapshot models.AuctionSnapshot
	if db.Order("scanned_at DESC").First(&snapshot).Error != nil {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	type catRow struct {
		ItemClass    string
		ItemSubclass string
		Count        int64
	}
	var rows []catRow
	db.Table("auction_entries ae").
		Select("COALESCE(i.item_class, 'Inconnu') as item_class, COALESCE(i.item_subclass, 'Autre') as item_subclass, COUNT(DISTINCT ae.item_id) as count").
		Joins("LEFT JOIN items i ON i.id = ae.item_id").
		Where("ae.snapshot_id = ?", snapshot.ID).
		Group("i.item_class, i.item_subclass").
		Order("i.item_class ASC, i.item_subclass ASC").
		Scan(&rows)

	// Build tree: { category: string, subcategories: [{name, count}], total: int }
	type subcat struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	type category struct {
		Name          string   `json:"name"`
		Subcategories []subcat `json:"subcategories"`
		Total         int64    `json:"total"`
	}
	catMap := make(map[string]*category)
	var catOrder []string
	for _, r := range rows {
		cat, ok := catMap[r.ItemClass]
		if !ok {
			cat = &category{Name: r.ItemClass}
			catMap[r.ItemClass] = cat
			catOrder = append(catOrder, r.ItemClass)
		}
		cat.Subcategories = append(cat.Subcategories, subcat{Name: r.ItemSubclass, Count: r.Count})
		cat.Total += r.Count
	}

	result := make([]category, 0, len(catOrder))
	for _, name := range catOrder {
		result = append(result, *catMap[name])
	}
	c.JSON(http.StatusOK, result)
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

// ════════════════════════════════════════
// AI Trading Simulator
// ════════════════════════════════════════

func getAIStats(c *gin.Context) {
	db := database.DB

	// Latest snapshot
	var snap models.AIPortfolioSnapshot
	if err := db.Order("recorded_at DESC").First(&snap).Error; err != nil {
		// No data yet — return empty stats
		c.JSON(http.StatusOK, models.AIStatsDTO{
			InitialBudgetCopper: 1000000000, // 100k gold
			InitialBudgetGold:   models.CopperToGoldStr(1000000000),
		})
		return
	}

	// Closed trades stats
	var closedTrades []models.AITrade
	db.Where("status IN ?", []string{"SOLD", "EXPIRED"}).Find(&closedTrades)

	winning := 0
	losing := 0
	var bestPnl, worstPnl int64
	var totalProfitPct float64
	for _, t := range closedTrades {
		if t.ProfitCopper > 0 {
			winning++
		} else {
			losing++
		}
		if t.ProfitCopper > bestPnl {
			bestPnl = t.ProfitCopper
		}
		if t.ProfitCopper < worstPnl {
			worstPnl = t.ProfitCopper
		}
		totalProfitPct += t.ProfitPct
	}

	winRate := 0.0
	avgProfitPct := 0.0
	if len(closedTrades) > 0 {
		winRate = float64(winning) / float64(len(closedTrades)) * 100
		avgProfitPct = totalProfitPct / float64(len(closedTrades))
	}

	totalPnl := snap.RealizedPnl + snap.UnrealizedPnl
	roiPct := 0.0
	if snap.TotalValueCopper > 0 {
		roiPct = float64(totalPnl) / float64(1000000000) * 100 // vs initial budget
	}

	stats := models.AIStatsDTO{
		InitialBudgetCopper: 1000000000,
		InitialBudgetGold:   models.CopperToGoldStr(1000000000),
		CurrentCashCopper:   snap.CashCopper,
		CurrentCashGold:     models.CopperToGoldStr(snap.CashCopper),
		InvestedCopper:      snap.InvestedCopper,
		InvestedGold:        models.CopperToGoldStr(snap.InvestedCopper),
		TotalValueCopper:    snap.TotalValueCopper,
		TotalValueGold:      models.CopperToGoldStr(snap.TotalValueCopper),

		RealizedPnlCopper:   snap.RealizedPnl,
		RealizedPnlGold:     models.CopperToGoldStr(snap.RealizedPnl),
		UnrealizedPnlCopper: snap.UnrealizedPnl,
		UnrealizedPnlGold:   models.CopperToGoldStr(snap.UnrealizedPnl),
		TotalPnlCopper:      totalPnl,
		TotalPnlGold:        models.CopperToGoldStr(totalPnl),
		ROIPct:              math.Round(roiPct*100) / 100,

		TotalTrades:    snap.TotalTrades,
		OpenPositions:  snap.OpenPositions,
		ClosedTrades:   len(closedTrades),
		WinningTrades:  winning,
		LosingTrades:   losing,
		WinRate:        math.Round(winRate*100) / 100,
		AvgProfitPct:   math.Round(avgProfitPct*100) / 100,
		BestTradePnl:   bestPnl,
		BestTradeGold:  models.CopperToGoldStr(bestPnl),
		WorstTradePnl:  worstPnl,
		WorstTradeGold: models.CopperToGoldStr(worstPnl),
	}

	c.JSON(http.StatusOK, stats)
}

func getAIHoldings(c *gin.Context) {
	db := database.DB
	var holdings []models.AITrade
	db.Where("status = ?", "HOLDING").Order("created_at DESC").Find(&holdings)

	dtos := make([]models.AITradeDTO, len(holdings))
	for i, h := range holdings {
		currentPrice := ai.GetCurrentMinPrice(h.ItemID)
		dtos[i] = models.NewAITradeDTO(h, currentPrice)
	}
	c.JSON(http.StatusOK, dtos)
}

func getAITrades(c *gin.Context) {
	db := database.DB
	status := c.DefaultQuery("status", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	query := db.Model(&models.AITrade{}).Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", strings.ToUpper(status))
	}

	var trades []models.AITrade
	query.Limit(limit).Find(&trades)

	dtos := make([]models.AITradeDTO, len(trades))
	for i, t := range trades {
		cp := int64(0)
		if t.Status == "HOLDING" {
			cp = ai.GetCurrentMinPrice(t.ItemID)
		}
		dtos[i] = models.NewAITradeDTO(t, cp)
	}
	c.JSON(http.StatusOK, dtos)
}

func getAISnapshots(c *gin.Context) {
	db := database.DB
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	var snapshots []models.AIPortfolioSnapshot
	db.Where("recorded_at >= ?", cutoff).Order("recorded_at ASC").Find(&snapshots)

	dtos := make([]models.AIPortfolioSnapshotDTO, len(snapshots))
	for i, s := range snapshots {
		dtos[i] = models.NewAIPortfolioSnapshotDTO(s)
	}
	c.JSON(http.StatusOK, dtos)
}
