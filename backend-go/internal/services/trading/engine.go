package trading

import (
	"log"
	"math"
	"sort"
	"time"

	"wow-ah-bot/internal/config"
	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/models"
	"wow-ah-bot/internal/services/blizzard"
)

// ════════════════════════════════════════
// Constants
// ════════════════════════════════════════

const (
	minHistoryPoints = 5  // minimum scans required to score an item
	lookbackDays     = 7  // history window
	minDailyVolume   = 10 // items with fewer average daily qty are ignored
	ahCut            = 0.05
	minIRForDeal     = 40.0 // IR threshold below which no deal is created
)

// IR component weights — must sum to 1.0.
const (
	wUndervaluation = 0.35
	wMomentum       = 0.20
	wLiquidity      = 0.20
	wStability      = 0.15
	wNetProfit      = 0.10
)

// ════════════════════════════════════════
// Public API
// ════════════════════════════════════════

// Analyze computes the Indice de Rentabilité (IR) for every eligible item,
// persists ItemScore rows, and creates Deal rows for items with IR >= minIRForDeal.
func Analyze() ([]models.Deal, error) {
	log.Println("🧠 Running IR-based trading analysis...")
	db := database.DB
	cfg := config.Cfg
	cutoff := time.Now().UTC().Add(-lookbackDays * 24 * time.Hour)

	// Get the latest scan ID (used to group ItemScore rows per scan)
	var latestSnap models.AuctionSnapshot
	if err := db.Order("scanned_at DESC").First(&latestSnap).Error; err != nil {
		log.Println("⚠️  No snapshots found, skipping analysis")
		return nil, nil
	}

	// Items that have enough price history in the window
	type itemCount struct {
		ItemID int
		Cnt    int
	}
	var candidates []itemCount
	db.Model(&models.PriceHistory{}).
		Select("item_id, COUNT(*) as cnt").
		Where("scanned_at >= ?", cutoff).
		Group("item_id").
		Having("COUNT(*) >= ?", minHistoryPoints).
		Scan(&candidates)

	log.Printf("📊 Scoring %d items with sufficient data (IR threshold: %.0f/100)", len(candidates), minIRForDeal)

	var scores []models.ItemScore
	var deals []models.Deal

	for i, c := range candidates {
		if i >= cfg.MaxTrackedItems {
			break
		}
		score := scoreItem(c.ItemID, cutoff, latestSnap.ID)
		if score == nil {
			continue
		}
		scores = append(scores, *score)

		if score.RentabilityIndex >= minIRForDeal {
			deal := buildDeal(score, cfg)
			if deal != nil {
				deals = append(deals, *deal)
			}
		}
	}

	// Sort deals by IR descending
	sort.Slice(deals, func(i, j int) bool {
		return deals[i].RentabilityIndex > deals[j].RentabilityIndex
	})

	// Persist ItemScores — delete previous scores for this scan to allow re-analysis
	if len(scores) > 0 {
		db.Where("scan_id = ?", latestSnap.ID).Delete(&models.ItemScore{})
		db.CreateInBatches(scores, 200)
		log.Printf("📈 Persisted %d item scores", len(scores))
	}

	// Persist Deals
	if len(deals) > 0 {
		db.CreateInBatches(deals, 100)
	}

	log.Printf("💰 Found %d deals with IR >= %.0f", len(deals), minIRForDeal)
	return deals, nil
}

// ════════════════════════════════════════
// Core IR computation
// ════════════════════════════════════════

// scoreItem fetches price history for one item and computes its full IR breakdown.
func scoreItem(itemID int, cutoff time.Time, scanID uint) *models.ItemScore {
	db := database.DB

	var records []models.PriceHistory
	db.Where("item_id = ? AND scanned_at >= ?", itemID, cutoff).
		Order("scanned_at ASC").
		Find(&records)

	if len(records) < minHistoryPoints {
		return nil
	}

	n := len(records)
	minPrices := make([]float64, 0, n)
	medianPrices := make([]float64, 0, n)
	avgPrices := make([]float64, 0, n)
	volumes := make([]float64, 0, n)

	for _, r := range records {
		if r.MinBuyout > 0 {
			minPrices = append(minPrices, float64(r.MinBuyout))
		}
		if r.MedianBuyout > 0 {
			medianPrices = append(medianPrices, float64(r.MedianBuyout))
		}
		if r.AvgBuyout > 0 {
			avgPrices = append(avgPrices, float64(r.AvgBuyout))
		}
		volumes = append(volumes, float64(r.TotalQuantity))
	}

	if len(minPrices) < 3 || len(avgPrices) < 3 {
		return nil
	}

	currentMin := minPrices[len(minPrices)-1]
	currentVolume := volumes[len(volumes)-1]
	_ = currentVolume // available for future use

	// Historical window = all points except the latest
	histMinSlice := minPrices[:len(minPrices)-1]
	histMedianSlice := medianPrices
	if len(medianPrices) > 1 {
		histMedianSlice = medianPrices[:len(medianPrices)-1]
	}
	histAvgSlice := avgPrices[:len(avgPrices)-1]
	histVols := volumes[:len(volumes)-1]

	histMedian := median(histMedianSlice)
	histAvg := mean(histAvgSlice)
	histStd := stddev(histAvgSlice)
	avgVolume := mean(histVols)

	if histMedian <= 0 || histAvg <= 0 || avgVolume < minDailyVolume {
		return nil
	}

	// ── Component 1: Sous-évaluation (35%) ──────────────────────────────────────
	// How far below the historical median is the current min price?
	// 0 = at/above median; 100 = 50% below median (cap).
	undervaluationRaw := (histMedian - currentMin) / histMedian
	scoreUndervaluation := clamp01(undervaluationRaw/0.5) * 100

	// ── Component 2: Momentum négatif (20%) ─────────────────────────────────────
	// Linear regression slope over historical min prices.
	// Falling prices → good entry point → higher score.
	slope := linearSlope(histMinSlice)
	slopeNorm := slope / histAvg            // normalise by price level
	scoreMomentum := clamp01(-slopeNorm/0.1) * 100 // slopeNorm in [-0.1, 0] → [100, 0]

	// ── Component 3: Liquidité (20%) ─────────────────────────────────────────────
	// Normalise against 200 units/day = perfect liquidity.
	scoreLiquidity := clamp01(avgVolume/200.0) * 100

	// ── Component 4: Stabilité (15%) ─────────────────────────────────────────────
	// Inverse of coefficient of variation. CV=0 → stable (100); CV≥0.5 → chaotic (0).
	cv := 0.0
	if histAvg > 0 {
		cv = histStd / histAvg
	}
	scoreStability := clamp01(1.0-cv/0.5) * 100

	// ── Component 5: Profit net AH (10%) ─────────────────────────────────────────
	// Net margin after 5% AH cut, capped at 50%.
	grossProfit := (histMedian - currentMin) * (1 - ahCut)
	netMarginFrac := grossProfit / currentMin
	scoreNetProfit := clamp01(netMarginFrac/0.5) * 100

	// ── Weighted IR ──────────────────────────────────────────────────────────────
	ir := scoreUndervaluation*wUndervaluation +
		scoreMomentum*wMomentum +
		scoreLiquidity*wLiquidity +
		scoreStability*wStability +
		scoreNetProfit*wNetProfit

	ir = math.Round(ir*10) / 10

	return &models.ItemScore{
		ItemID:              itemID,
		ScanID:              scanID,
		ScoreUndervaluation: math.Round(scoreUndervaluation*10) / 10,
		ScoreMomentum:       math.Round(scoreMomentum*10) / 10,
		ScoreLiquidity:      math.Round(scoreLiquidity*10) / 10,
		ScoreStability:      math.Round(scoreStability*10) / 10,
		ScoreNetProfit:      math.Round(scoreNetProfit*10) / 10,
		RentabilityIndex:    ir,
		CurrentMinPrice:     int64(currentMin),
		HistMedianPrice:     int64(histMedian),
		AvgDailyVolume:      math.Round(avgVolume*10) / 10,
		PriceSlope:          math.Round(slope*100) / 100,
		CoeffVariation:      math.Round(cv*1000) / 1000,
		DataPoints:          len(records),
	}
}

// buildDeal converts an ItemScore into a Deal if the item qualifies.
func buildDeal(s *models.ItemScore, cfg *config.Settings) *models.Deal {
	currentMin := float64(s.CurrentMinPrice)
	histMedian := float64(s.HistMedianPrice)

	// Margin after AH cut
	grossMarginPct := (histMedian-currentMin)*(1-ahCut)/currentMin*100
	if grossMarginPct < cfg.MinProfitMargin {
		return nil
	}

	suggestedBuy := s.CurrentMinPrice
	suggestedSell := s.HistMedianPrice

	// Suggested qty: bounded by budget and 20% of avg daily volume
	suggestedQty := 1
	maxByBudget := int(float64(cfg.MaxBudgetGold) * 10000 / math.Max(float64(suggestedBuy), 1))
	maxByVolume := int(math.Max(1, s.AvgDailyVolume*0.2))
	suggestedQty = min3(maxByBudget, maxByVolume, 200)
	if suggestedQty < 1 {
		suggestedQty = 1
	}

	// Resolve item name
	var itemName string
	database.DB.Model(&models.Item{}).Where("id = ?", s.ItemID).Pluck("name", &itemName)
	if itemName == "" {
		itemName = fetchAndCacheItemName(s.ItemID)
	}

	return &models.Deal{
		ItemID:             s.ItemID,
		ItemName:           itemName,
		DetectedAt:         time.Now().UTC(),
		CurrentPrice:       suggestedBuy,
		AvgPrice:           suggestedSell,
		SuggestedBuyPrice:  suggestedBuy,
		SuggestedSellPrice: suggestedSell,
		SuggestedQuantity:  suggestedQty,
		ProfitMargin:       math.Round(grossMarginPct*100) / 100,
		RentabilityIndex:   s.RentabilityIndex,
		Status:             "PENDING",
		Notified:           false,
	}
}

// ════════════════════════════════════════
// Helpers
// ════════════════════════════════════════

// fetchAndCacheItemName calls the Blizzard API and caches the result in the items table.
func fetchAndCacheItemName(itemID int) string {
	details, err := blizzard.GetItemWithDetails(itemID)
	if err != nil || details.Name == "" {
		return ""
	}
	db := database.DB
	item := models.Item{
		ID:           details.ID,
		Name:         details.Name,
		Quality:      details.Quality,
		ItemClass:    details.ItemClass,
		ItemSubclass: details.ItemSubclass,
		Level:        details.Level,
		IconURL:      details.IconURL,
		VendorPrice:  details.VendorPrice,
	}
	db.Save(&item)
	return details.Name
}

// clamp01 clamps v to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// linearSlope returns the OLS slope of data over implicit x = [0, 1, ..., n-1].
func linearSlope(data []float64) float64 {
	n := float64(len(data))
	if n < 2 {
		return 0
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	var s float64
	for _, v := range data {
		s += v
	}
	return s / float64(len(data))
}

func median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

func stddev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := mean(data)
	var sumSq float64
	for _, v := range data {
		d := v - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(data)))
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
