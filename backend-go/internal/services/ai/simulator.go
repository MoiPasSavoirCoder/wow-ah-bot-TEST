package ai

import (
	"log"
	"math"
	"sort"
	"time"

	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/models"
)

// ════════════════════════════════════════
// Constants
// ════════════════════════════════════════

const (
	initialBudgetGold   = 100000               // 100 000 gold de départ
	initialBudgetCopper = initialBudgetGold * 10000
	maxHoldingDays      = 7                     // expire un holding après 7 jours
	minIRForBuy         = 30.0                  // IR minimum pour acheter
	sellProfitTarget    = 0.15                  // vend quand +15% de profit
	stopLossThreshold   = -0.20                 // stop-loss à -20%
	maxPositions        = 20                    // max positions ouvertes simultanées
	maxPositionPct      = 0.15                  // max 15% du budget par position
	ahFeeCopper         = 20000                 // frais AH fixes = 2 PO (2g)
)

// ════════════════════════════════════════
// SimulateTrades — called after each Analyze()
// ════════════════════════════════════════

// SimulateTrades examines current deals/scores and simulates virtual buy/sell decisions.
func SimulateTrades() error {
	log.Println("🤖 AI Simulator: running trade simulation...")

	// ── 1. Ensure we have an initial state ──
	ensureInitialBalance()

	// ── 2. Get current virtual balance ──
	balance := getCurrentBalance()
	if balance == 0 {
		balance = initialBudgetCopper
	}

	// ── 3. Try to sell existing holdings ──
	sellCount := tryAutoSells(balance)

	// ── 4. Try to buy new positions from latest deals ──
	buyCount := tryAutoBuys(balance)

	// ── 5. Expire old holdings ──
	expireCount := expireOldHoldings()

	// ── 6. Record portfolio snapshot ──
	takeSnapshot()

	log.Printf("🤖 AI Simulator done: %d buys, %d sells, %d expired", buyCount, sellCount, expireCount)
	return nil
}

// ════════════════════════════════════════
// Balance management
// ════════════════════════════════════════

func ensureInitialBalance() {
	db := database.DB
	var count int64
	db.Model(&models.AITrade{}).Count(&count)
	if count == 0 {
		// Also ensure a snapshot exists
		var snapCount int64
		db.Model(&models.AIPortfolioSnapshot{}).Count(&snapCount)
		if snapCount == 0 {
			db.Create(&models.AIPortfolioSnapshot{
				RecordedAt:       time.Now().UTC(),
				CashCopper:       initialBudgetCopper,
				InvestedCopper:   0,
				TotalValueCopper: initialBudgetCopper,
				RealizedPnl:      0,
				UnrealizedPnl:    0,
				OpenPositions:    0,
				TotalTrades:      0,
			})
		}
	}
}

// getCurrentBalance returns the available cash in copper.
func getCurrentBalance() int64 {
	db := database.DB
	var snap models.AIPortfolioSnapshot
	if err := db.Order("recorded_at DESC").First(&snap).Error; err != nil {
		return initialBudgetCopper
	}
	return snap.CashCopper
}

// ════════════════════════════════════════
// Auto-sell logic
// ════════════════════════════════════════

func tryAutoSells(balance int64) int {
	db := database.DB
	var holdings []models.AITrade
	db.Where("status = ?", "HOLDING").Find(&holdings)

	sellCount := 0
	for _, h := range holdings {
		currentPrice := getCurrentMinPrice(h.ItemID)
		if currentPrice <= 0 {
			continue
		}

		// Calculate profit percentage
		buyPrice := float64(h.PricePerUnit)
		sellPrice := float64(currentPrice)
		netSellTotal := sellPrice*float64(h.Quantity) - float64(ahFeeCopper) // flat 2g AH fee
		netSellPrice := netSellTotal / float64(h.Quantity)
		profitPct := (netSellPrice - buyPrice) / buyPrice

		reason := ""

		// ── Sell condition 1: target profit reached ──
		if profitPct >= sellProfitTarget {
			reason = "TARGET_PROFIT"
		}

		// ── Sell condition 2: stop-loss triggered ──
		if profitPct <= stopLossThreshold {
			reason = "STOP_LOSS"
		}

		// ── Sell condition 3: price is above historical median (original deal target) ──
		if reason == "" {
			histMedian := getHistMedianPrice(h.ItemID)
			if histMedian > 0 && currentPrice >= histMedian {
				reason = "ABOVE_MEDIAN"
			}
		}

		if reason != "" {
			now := time.Now().UTC()
			netTotal := int64(netSellTotal)
			profitCopper := netTotal - h.TotalCost

			h.Status = "SOLD"
			h.SoldAt = &now
			h.SellPrice = currentPrice
			h.SellTotal = netTotal
			h.ProfitCopper = profitCopper
			h.ProfitPct = math.Round(profitPct*10000) / 100 // e.g. 15.23%
			h.SellReason = reason
			db.Save(&h)

			sellCount++
			log.Printf("🤖 AI SELL: %s x%d @ %s (profit: %s, reason: %s)",
				h.ItemName, h.Quantity,
				models.CopperToGoldStr(currentPrice),
				models.CopperToGoldStr(profitCopper),
				reason)
		}
	}
	return sellCount
}

// ════════════════════════════════════════
// Auto-buy logic
// ════════════════════════════════════════

func tryAutoBuys(cashBalance int64) int {
	db := database.DB

	// Count open positions
	var openCount int64
	db.Model(&models.AITrade{}).Where("status = ?", "HOLDING").Count(&openCount)
	if openCount >= maxPositions {
		log.Println("🤖 AI: max positions reached, skipping buys")
		return 0
	}

	// Recalculate available cash from snapshot
	var snap models.AIPortfolioSnapshot
	if err := db.Order("recorded_at DESC").First(&snap).Error; err == nil {
		cashBalance = snap.CashCopper
	}

	// ── Strategy 1: Use existing IR scores if available ──
	var latestSnap models.AuctionSnapshot
	if err := db.Order("scanned_at DESC").First(&latestSnap).Error; err != nil {
		return 0
	}

	var scores []models.ItemScore
	db.Where("scan_id = ? AND rentability_index >= ?", latestSnap.ID, minIRForBuy).
		Order("rentability_index DESC").
		Limit(50).
		Find(&scores)

	// ── Strategy 2: If no IR scores, use direct price analysis ──
	type directCandidate struct {
		ItemID      int
		CurrentMin  int64
		HistMedian  int64
		AvgVolume   float64
		Discount    float64 // % below median
	}
	var directCandidates []directCandidate

	if len(scores) == 0 {
		log.Println("🤖 AI: no IR scores found, using direct price analysis")
		cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)

		// Find items where current price is significantly below historical median
		type rawCandidate struct {
			ItemID    int
			MinBuyout int64
		}

		// Get latest price per item
		var latest []rawCandidate
		db.Raw(`
			SELECT ph.item_id, ph.min_buyout 
			FROM price_history ph
			INNER JOIN (
				SELECT item_id, MAX(scanned_at) as max_scan 
				FROM price_history 
				WHERE scanned_at >= ?
				GROUP BY item_id 
				HAVING COUNT(*) >= 3
			) sub ON ph.item_id = sub.item_id AND ph.scanned_at = sub.max_scan
			WHERE ph.min_buyout > 0
			ORDER BY ph.item_id
			LIMIT 5000
		`, cutoff).Scan(&latest)

		for _, l := range latest {
			// Get historical median for this item
			var medians []int64
			db.Model(&models.PriceHistory{}).
				Where("item_id = ? AND scanned_at >= ? AND median_buyout > 0", l.ItemID, cutoff).
				Pluck("median_buyout", &medians)

			if len(medians) < 3 {
				continue
			}

			sort.Slice(medians, func(i, j int) bool { return medians[i] < medians[j] })
			histMedian := medians[len(medians)/2]

			if histMedian <= 0 || l.MinBuyout >= histMedian {
				continue
			}

			// Get average volume
			var avgQty float64
			db.Model(&models.PriceHistory{}).
				Where("item_id = ? AND scanned_at >= ?", l.ItemID, cutoff).
				Select("AVG(total_quantity)").
				Scan(&avgQty)

			if avgQty < 5 {
				continue
			}

			discount := float64(histMedian-l.MinBuyout) / float64(histMedian) * 100
			if discount < 10 { // at least 10% below median
				continue
			}

			directCandidates = append(directCandidates, directCandidate{
				ItemID:     l.ItemID,
				CurrentMin: l.MinBuyout,
				HistMedian: histMedian,
				AvgVolume:  avgQty,
				Discount:   discount,
			})
		}

		// Sort by discount descending
		sort.Slice(directCandidates, func(i, j int) bool {
			return directCandidates[i].Discount > directCandidates[j].Discount
		})

		// Keep top 30
		if len(directCandidates) > 30 {
			directCandidates = directCandidates[:30]
		}

		log.Printf("🤖 AI: found %d direct candidates (10%%+ below median)", len(directCandidates))
	}

	buyCount := 0
	slotsAvailable := int(maxPositions - openCount)

	// ── Buy from IR scores ──
	for _, score := range scores {
		if buyCount >= slotsAvailable {
			break
		}
		if !canBuyItem(score.ItemID) {
			continue
		}

		currentPrice := score.CurrentMinPrice
		if currentPrice <= 0 {
			continue
		}

		qty, totalCost := calculatePosition(currentPrice, cashBalance, score.AvgDailyVolume)
		if qty < 1 || totalCost > cashBalance {
			continue
		}

		itemName, iconURL := resolveItem(score.ItemID)

		trade := models.AITrade{
			ItemID:           score.ItemID,
			ItemName:         itemName,
			IconURL:          iconURL,
			Action:           "BUY",
			Quantity:         int(qty),
			PricePerUnit:     currentPrice,
			TotalCost:        totalCost,
			TargetSellPrice:  score.HistMedianPrice,
			RentabilityIndex: score.RentabilityIndex,
			Status:           "HOLDING",
			ScanID:           score.ScanID,
		}

		db.Create(&trade)
		cashBalance -= totalCost
		buyCount++

		log.Printf("🤖 AI BUY (IR): %s x%d @ %s (IR: %.1f, target: %s)",
			itemName, qty,
			models.CopperToGoldStr(currentPrice),
			score.RentabilityIndex,
			models.CopperToGoldStr(score.HistMedianPrice))
	}

	// ── Buy from direct candidates (fallback) ──
	for _, dc := range directCandidates {
		if buyCount >= slotsAvailable {
			break
		}
		if !canBuyItem(dc.ItemID) {
			continue
		}

		qty, totalCost := calculatePosition(dc.CurrentMin, cashBalance, dc.AvgVolume)
		if qty < 1 || totalCost > cashBalance {
			continue
		}

		itemName, iconURL := resolveItem(dc.ItemID)

		// Synthetic IR based on discount
		syntheticIR := dc.Discount * 1.5 // e.g. 20% discount = IR 30
		if syntheticIR > 100 {
			syntheticIR = 100
		}

		trade := models.AITrade{
			ItemID:           dc.ItemID,
			ItemName:         itemName,
			IconURL:          iconURL,
			Action:           "BUY",
			Quantity:         int(qty),
			PricePerUnit:     dc.CurrentMin,
			TotalCost:        totalCost,
			TargetSellPrice:  dc.HistMedian,
			RentabilityIndex: syntheticIR,
			Status:           "HOLDING",
			ScanID:           latestSnap.ID,
		}

		db.Create(&trade)
		cashBalance -= totalCost
		buyCount++

		log.Printf("🤖 AI BUY (DIRECT): %s x%d @ %s (discount: %.1f%%, target: %s)",
			itemName, qty,
			models.CopperToGoldStr(dc.CurrentMin),
			dc.Discount,
			models.CopperToGoldStr(dc.HistMedian))
	}

	return buyCount
}

// ── Buy helpers ──

func canBuyItem(itemID int) bool {
	db := database.DB

	// Skip if we already hold this item
	var existingCount int64
	db.Model(&models.AITrade{}).
		Where("item_id = ? AND status = ?", itemID, "HOLDING").
		Count(&existingCount)
	if existingCount > 0 {
		return false
	}

	// Skip if we bought this item in the last 2 hours (cooldown)
	cooldownCutoff := time.Now().UTC().Add(-2 * time.Hour)
	var recentCount int64
	db.Model(&models.AITrade{}).
		Where("item_id = ? AND created_at >= ?", itemID, cooldownCutoff).
		Count(&recentCount)
	return recentCount == 0
}

func calculatePosition(currentPrice, cashBalance int64, avgDailyVolume float64) (int64, int64) {
	maxPerPosition := int64(float64(initialBudgetCopper) * maxPositionPct)

	maxByBudget := cashBalance / currentPrice
	maxByPosition := maxPerPosition / currentPrice
	maxByVolume := int64(math.Max(1, avgDailyVolume*0.2))

	qty := minInt64(maxByBudget, minInt64(maxByPosition, maxByVolume))
	if qty > 200 {
		qty = 200
	}
	if qty < 1 {
		return 0, 0
	}

	totalCost := currentPrice * qty
	return qty, totalCost
}

func resolveItem(itemID int) (string, string) {
	db := database.DB
	var item models.Item
	if db.Where("id = ?", itemID).First(&item).Error == nil {
		name := item.Name
		if name == "" {
			name = "Item inconnu"
		}
		return name, item.IconURL
	}
	return "Item inconnu", ""
}

// ════════════════════════════════════════
// Expire old holdings
// ════════════════════════════════════════

func expireOldHoldings() int {
	db := database.DB
	cutoff := time.Now().UTC().Add(-time.Duration(maxHoldingDays) * 24 * time.Hour)

	var expired []models.AITrade
	db.Where("status = ? AND created_at < ?", "HOLDING", cutoff).Find(&expired)

	for _, h := range expired {
		currentPrice := getCurrentMinPrice(h.ItemID)
		if currentPrice <= 0 {
			currentPrice = h.PricePerUnit // fallback
		}

		now := time.Now().UTC()
		netTotal := int64(currentPrice)*int64(h.Quantity) - int64(ahFeeCopper)
		profitCopper := netTotal - h.TotalCost
		profitPct := float64(profitCopper) / float64(h.TotalCost) * 100

		h.Status = "EXPIRED"
		h.SoldAt = &now
		h.SellPrice = currentPrice
		h.SellTotal = netTotal
		h.ProfitCopper = profitCopper
		h.ProfitPct = math.Round(profitPct*100) / 100
		h.SellReason = "EXPIRED"
		db.Save(&h)

		log.Printf("🤖 AI EXPIRED: %s x%d (P&L: %s)",
			h.ItemName, h.Quantity, models.CopperToGoldStr(profitCopper))
	}

	return len(expired)
}

// ════════════════════════════════════════
// Snapshot — record daily portfolio state
// ════════════════════════════════════════

func takeSnapshot() {
	db := database.DB

	// Cash = initial budget - total invested in open positions + total realized from closed positions
	var totalInvested int64
	db.Model(&models.AITrade{}).
		Where("status = ?", "HOLDING").
		Select("COALESCE(SUM(total_cost), 0)").
		Scan(&totalInvested)

	var realizedPnl int64
	db.Model(&models.AITrade{}).
		Where("status IN ?", []string{"SOLD", "EXPIRED"}).
		Select("COALESCE(SUM(profit_copper), 0)").
		Scan(&realizedPnl)

	var totalSellRevenue int64
	db.Model(&models.AITrade{}).
		Where("status IN ?", []string{"SOLD", "EXPIRED"}).
		Select("COALESCE(SUM(sell_total), 0)").
		Scan(&totalSellRevenue)

	var totalSpentAll int64
	db.Model(&models.AITrade{}).
		Select("COALESCE(SUM(total_cost), 0)").
		Scan(&totalSpentAll)

	cashCopper := initialBudgetCopper - totalSpentAll + totalSellRevenue

	// Unrealized P&L: for each holding, compare current price to buy price
	var holdings []models.AITrade
	db.Where("status = ?", "HOLDING").Find(&holdings)

	var unrealizedPnl int64
	for _, h := range holdings {
		cp := getCurrentMinPrice(h.ItemID)
		if cp <= 0 {
			continue
		}
		netTotal := cp*int64(h.Quantity) - int64(ahFeeCopper)
		unrealizedPnl += netTotal - h.PricePerUnit*int64(h.Quantity)
	}

	var openPositions int64
	db.Model(&models.AITrade{}).Where("status = ?", "HOLDING").Count(&openPositions)

	var totalTrades int64
	db.Model(&models.AITrade{}).Count(&totalTrades)

	totalValue := cashCopper + totalInvested + unrealizedPnl

	snap := models.AIPortfolioSnapshot{
		RecordedAt:       time.Now().UTC(),
		CashCopper:       cashCopper,
		InvestedCopper:   totalInvested,
		TotalValueCopper: totalValue,
		RealizedPnl:      realizedPnl,
		UnrealizedPnl:    unrealizedPnl,
		OpenPositions:    int(openPositions),
		TotalTrades:      int(totalTrades),
	}
	db.Create(&snap)
}

// ════════════════════════════════════════
// Helpers
// ════════════════════════════════════════

// GetCurrentMinPrice returns the current min buyout for an item from the latest snapshot.
// Exported for use by API handlers.
func GetCurrentMinPrice(itemID int) int64 {
	return getCurrentMinPrice(itemID)
}

// getCurrentMinPrice returns the current min buyout for an item from the latest snapshot.
func getCurrentMinPrice(itemID int) int64 {
	db := database.DB
	var ph models.PriceHistory
	if err := db.Where("item_id = ?", itemID).Order("scanned_at DESC").First(&ph).Error; err != nil {
		return 0
	}
	return ph.MinBuyout
}

// getHistMedianPrice returns the median of historical median buyouts over lookback window.
func getHistMedianPrice(itemID int) int64 {
	db := database.DB
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)

	var prices []int64
	db.Model(&models.PriceHistory{}).
		Where("item_id = ? AND scanned_at >= ? AND median_buyout > 0", itemID, cutoff).
		Pluck("median_buyout", &prices)

	if len(prices) == 0 {
		return 0
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	return prices[len(prices)/2]
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
