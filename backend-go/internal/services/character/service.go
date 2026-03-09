package character

import (
	"errors"
	"log"
	"strings"
	"time"

	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/models"
)

const ahCut = 0.05

// ════════════════════════════════════════
// Character CRUD
// ════════════════════════════════════════

// Create registers a new character.
func Create(req models.CharacterAddRequest) (*models.Character, error) {
	db := database.DB
	c := models.Character{
		Name:      req.Name,
		Realm:     req.Realm,
		Class:     req.Class,
		Race:      req.Race,
		Level:     req.Level,
		AvatarURL: req.AvatarURL,
		Notes:     req.Notes,
		IsActive:  true,
	}
	if err := db.Create(&c).Error; err != nil {
		return nil, err
	}
	log.Printf("🧙 Character created: %s-%s", c.Name, c.Realm)
	return &c, nil
}

// List returns all characters, active ones first.
func List() ([]models.Character, error) {
	db := database.DB
	var chars []models.Character
	err := db.Order("is_active DESC, name ASC").Find(&chars).Error
	return chars, err
}

// Get returns a single character by ID.
func Get(id uint) (*models.Character, error) {
	db := database.DB
	var c models.Character
	if err := db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// Update applies partial updates to a character.
func Update(id uint, req models.CharacterUpdateRequest) (*models.Character, error) {
	db := database.DB
	c, err := Get(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Realm != "" {
		updates["realm"] = req.Realm
	}
	if req.Class != "" {
		updates["class"] = req.Class
	}
	if req.Race != "" {
		updates["race"] = req.Race
	}
	if req.Level > 0 {
		updates["level"] = req.Level
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := db.Model(c).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return Get(id)
}

// Delete removes a character and all associated data.
func Delete(id uint) error {
	db := database.DB
	if _, err := Get(id); err != nil {
		return err
	}
	// Cascade: delete transactions and snapshots first
	db.Where("character_id = ?", id).Delete(&models.CharacterTransaction{})
	db.Where("character_id = ?", id).Delete(&models.CharacterSnapshot{})
	return db.Delete(&models.Character{}, id).Error
}

// ════════════════════════════════════════
// Transactions
// ════════════════════════════════════════

// AddTransaction records a buy or sell for a character.
func AddTransaction(charID uint, req models.CharacterTransactionAddRequest) (*models.CharacterTransaction, error) {
	db := database.DB

	// Verify character exists
	if _, err := Get(charID); err != nil {
		return nil, errors.New("character not found")
	}

	t := models.CharacterTransaction{
		CharacterID:  charID,
		ItemID:       req.ItemID,
		ItemName:     req.ItemName,
		IconURL:      req.IconURL,
		Action:       strings.ToUpper(req.Action),
		Quantity:     req.Quantity,
		PricePerUnit: req.PricePerUnit,
		TotalPrice:   req.PricePerUnit * int64(req.Quantity),
		DealID:       req.DealID,
		Notes:        req.Notes,
		TransactedAt: time.Now().UTC(),
	}
	if err := db.Create(&t).Error; err != nil {
		return nil, err
	}

	// Recompute and store a snapshot after each transaction
	go recordSnapshot(charID)

	log.Printf("💸 CharTx: [%s] %s %dx %s @ %d copper each",
		getCharName(charID), t.Action, t.Quantity, t.ItemName, t.PricePerUnit)
	return &t, nil
}

// GetTransactions returns transactions for a character, newest first.
func GetTransactions(charID uint, limit int) ([]models.CharacterTransaction, error) {
	db := database.DB
	var txs []models.CharacterTransaction
	q := db.Where("character_id = ?", charID).Order("transacted_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&txs).Error
	return txs, err
}

// ════════════════════════════════════════
// P&L
// ════════════════════════════════════════

// GetPnl computes the full P&L breakdown for a character.
func GetPnl(charID uint) (*models.CharacterPnlDTO, error) {
	c, err := Get(charID)
	if err != nil {
		return nil, err
	}

	db := database.DB

	// Aggregate buys
	type agg struct {
		TotalCopper int64
		TotalQty    int
	}
	var buyAgg, sellAgg agg
	db.Model(&models.CharacterTransaction{}).
		Select("COALESCE(SUM(total_price),0) as total_copper, COALESCE(SUM(quantity),0) as total_qty").
		Where("character_id = ? AND action = ?", charID, "BUY").
		Scan(&buyAgg)
	db.Model(&models.CharacterTransaction{}).
		Select("COALESCE(SUM(total_price),0) as total_copper, COALESCE(SUM(quantity),0) as total_qty").
		Where("character_id = ? AND action = ?", charID, "SELL").
		Scan(&sellAgg)

	ahFees := int64(float64(sellAgg.TotalCopper) * ahCut)
	realizedPnl := sellAgg.TotalCopper - ahFees - buyAgg.TotalCopper

	// Open positions: BUY qty - SELL qty per item
	type openRow struct {
		ItemID    int
		NetQty    int
		AvgBuyPrice float64
	}
	var openRows []openRow
	db.Raw(`
		SELECT
			item_id,
			SUM(CASE WHEN action='BUY' THEN quantity ELSE -quantity END) AS net_qty,
			AVG(CASE WHEN action='BUY' THEN price_per_unit ELSE NULL END) AS avg_buy_price
		FROM character_transactions
		WHERE character_id = ?
		GROUP BY item_id
		HAVING net_qty > 0
	`, charID).Scan(&openRows)

	var openCopper int64
	for _, r := range openRows {
		openCopper += int64(float64(r.NetQty) * r.AvgBuyPrice)
	}

	// Transaction counts
	var totalTx int64
	db.Model(&models.CharacterTransaction{}).Where("character_id = ?", charID).Count(&totalTx)

	// Win rate: SELL transactions where total_price > matching avg_buy_price * qty
	// Simplified: % of SELL transactions with a positive realized margin
	type sellRow struct {
		ItemID       int
		SellTotal    int64
		SellQty      int
	}
	var sellRows []sellRow
	db.Model(&models.CharacterTransaction{}).
		Select("item_id, SUM(total_price) as sell_total, SUM(quantity) as sell_qty").
		Where("character_id = ? AND action = ?", charID, "SELL").
		Group("item_id").
		Scan(&sellRows)

	wins := 0
	for _, sr := range sellRows {
		var avgBuy float64
		db.Model(&models.CharacterTransaction{}).
			Select("AVG(price_per_unit)").
			Where("character_id = ? AND item_id = ? AND action = ?", charID, sr.ItemID, "BUY").
			Scan(&avgBuy)
		if avgBuy > 0 && float64(sr.SellTotal)/float64(sr.SellQty) > avgBuy {
			wins++
		}
	}
	winRate := 0.0
	if len(sellRows) > 0 {
		winRate = float64(wins) / float64(len(sellRows)) * 100
	}

	// Latest snapshot balance
	var latestSnap models.CharacterSnapshot
	var latestBalance int64
	if db.Where("character_id = ?", charID).Order("recorded_at DESC").First(&latestSnap).Error == nil {
		latestBalance = latestSnap.BalanceCopper
	}

	return &models.CharacterPnlDTO{
		CharacterID:        charID,
		CharacterDTO:       models.NewCharacterDTO(*c),
		TotalBuyCopper:     buyAgg.TotalCopper,
		TotalSellCopper:    sellAgg.TotalCopper,
		AHFeesCopper:       ahFees,
		RealizedPnlCopper:  realizedPnl,
		OpenPositionCopper: openCopper,
		TotalBuyGold:       models.CopperToGoldStr(buyAgg.TotalCopper),
		TotalSellGold:      models.CopperToGoldStr(sellAgg.TotalCopper),
		AHFeesGold:         models.CopperToGoldStr(ahFees),
		RealizedPnlGold:    models.CopperToGoldStr(realizedPnl),
		OpenPositionGold:   models.CopperToGoldStr(openCopper),
		TotalTransactions:  int(totalTx),
		TotalItemsBought:   buyAgg.TotalQty,
		TotalItemsSold:     sellAgg.TotalQty,
		WinRate:            models.RoundFloat(winRate, 1),
		LatestBalance:      latestBalance,
		LatestBalanceGold:  models.CopperToGoldStr(latestBalance),
	}, nil
}

// ════════════════════════════════════════
// Snapshots
// ════════════════════════════════════════

// AddSnapshot records a manual gold balance snapshot for a character.
func AddSnapshot(charID uint, balanceCopper int64) (*models.CharacterSnapshot, error) {
	if _, err := Get(charID); err != nil {
		return nil, errors.New("character not found")
	}
	return saveSnapshot(charID, balanceCopper)
}

// GetSnapshots returns historical snapshots for a character, newest first.
func GetSnapshots(charID uint, days int) ([]models.CharacterSnapshot, error) {
	db := database.DB
	var snaps []models.CharacterSnapshot
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	err := db.Where("character_id = ? AND recorded_at >= ?", charID, cutoff).
		Order("recorded_at ASC").
		Find(&snaps).Error
	return snaps, err
}

// ════════════════════════════════════════
// Internal helpers
// ════════════════════════════════════════

// recordSnapshot recomputes invested copper from open positions and saves a snapshot.
func recordSnapshot(charID uint) {
	db := database.DB

	// Compute open (invested) copper
	type openRow struct {
		NetQty      int
		AvgBuyPrice float64
	}
	var rows []openRow
	db.Raw(`
		SELECT
			SUM(CASE WHEN action='BUY' THEN quantity ELSE -quantity END) AS net_qty,
			AVG(CASE WHEN action='BUY' THEN price_per_unit ELSE NULL END) AS avg_buy_price
		FROM character_transactions
		WHERE character_id = ?
		GROUP BY item_id
		HAVING net_qty > 0
	`, charID).Scan(&rows)

	var investedCopper int64
	for _, r := range rows {
		investedCopper += int64(float64(r.NetQty) * r.AvgBuyPrice)
	}

	// Realized P&L
	type agg struct{ TotalCopper int64 }
	var buyAgg, sellAgg agg
	db.Model(&models.CharacterTransaction{}).
		Select("COALESCE(SUM(total_price),0) as total_copper").
		Where("character_id = ? AND action = ?", charID, "BUY").Scan(&buyAgg)
	db.Model(&models.CharacterTransaction{}).
		Select("COALESCE(SUM(total_price),0) as total_copper").
		Where("character_id = ? AND action = ?", charID, "SELL").Scan(&sellAgg)
	ahFees := int64(float64(sellAgg.TotalCopper) * ahCut)
	profit := sellAgg.TotalCopper - ahFees - buyAgg.TotalCopper

	// Use latest known balance (if any)
	var latestSnap models.CharacterSnapshot
	var balance int64
	if db.Where("character_id = ?", charID).Order("recorded_at DESC").First(&latestSnap).Error == nil {
		balance = latestSnap.BalanceCopper
	}

	snap := models.CharacterSnapshot{
		CharacterID:    charID,
		BalanceCopper:  balance,
		InvestedCopper: investedCopper,
		ProfitCopper:   profit,
	}
	if err := db.Create(&snap).Error; err != nil {
		log.Printf("⚠️  CharSnapshot save failed for char %d: %v", charID, err)
	}
}

func saveSnapshot(charID uint, balance int64) (*models.CharacterSnapshot, error) {
	db := database.DB

	type openRow struct {
		NetQty      int
		AvgBuyPrice float64
	}
	var rows []openRow
	db.Raw(`
		SELECT
			SUM(CASE WHEN action='BUY' THEN quantity ELSE -quantity END) AS net_qty,
			AVG(CASE WHEN action='BUY' THEN price_per_unit ELSE NULL END) AS avg_buy_price
		FROM character_transactions
		WHERE character_id = ?
		GROUP BY item_id
		HAVING net_qty > 0
	`, charID).Scan(&rows)

	var investedCopper int64
	for _, r := range rows {
		investedCopper += int64(float64(r.NetQty) * r.AvgBuyPrice)
	}

	type agg struct{ TotalCopper int64 }
	var buyAgg, sellAgg agg
	db.Model(&models.CharacterTransaction{}).
		Select("COALESCE(SUM(total_price),0) as total_copper").
		Where("character_id = ? AND action = ?", charID, "BUY").Scan(&buyAgg)
	db.Model(&models.CharacterTransaction{}).
		Select("COALESCE(SUM(total_price),0) as total_copper").
		Where("character_id = ? AND action = ?", charID, "SELL").Scan(&sellAgg)
	ahFees := int64(float64(sellAgg.TotalCopper) * ahCut)
	profit := sellAgg.TotalCopper - ahFees - buyAgg.TotalCopper

	snap := models.CharacterSnapshot{
		CharacterID:    charID,
		BalanceCopper:  balance,
		InvestedCopper: investedCopper,
		ProfitCopper:   profit,
	}
	if err := db.Create(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

func getCharName(charID uint) string {
	db := database.DB
	var name string
	db.Model(&models.Character{}).Where("id = ?", charID).Pluck("name", &name)
	if name == "" {
		return "Unknown"
	}
	return name
}
