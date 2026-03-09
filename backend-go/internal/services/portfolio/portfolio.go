package portfolio

import (
	"log"
	"strings"
	"time"

	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/models"
)

// PnlData holds P&L summary values.
type PnlData struct {
	TotalInvestedCopper  int64
	TotalRevenueCopper   int64
	AHFeesCopper         int64
	RealizedProfitCopper int64
}

// InventoryItem holds a single item's inventory position.
type InventoryItem struct {
	ItemID        int    `json:"item_id"`
	ItemName      string `json:"item_name"`
	Quantity      int    `json:"quantity"`
	AvgBuyPrice   int64  `json:"avg_buy_price"`
	TotalInvested int64  `json:"total_invested"`
	TotalRevenue  int64  `json:"total_revenue"`
}

// AddTransaction records a buy/sell transaction.
func AddTransaction(itemID int, itemName, action string, quantity int, pricePerUnit int64, notes string) (*models.Portfolio, error) {
	db := database.DB
	entry := models.Portfolio{
		ItemID:       itemID,
		ItemName:     itemName,
		Action:       strings.ToUpper(action),
		Quantity:     quantity,
		PricePerUnit: pricePerUnit,
		TotalPrice:   pricePerUnit * int64(quantity),
		CreatedAt:    time.Now().UTC(),
		Notes:        notes,
	}
	if err := db.Create(&entry).Error; err != nil {
		return nil, err
	}

	// Update gold balance snapshot
	go updateBalance()

	log.Printf("📝 Portfolio: %s %dx %s @ %.1fg each",
		entry.Action, quantity, itemName, float64(pricePerUnit)/10000)
	return &entry, nil
}

// GetTransactions returns recent portfolio transactions.
func GetTransactions(limit int) ([]models.Portfolio, error) {
	db := database.DB
	var txs []models.Portfolio
	err := db.Order("created_at DESC").Limit(limit).Find(&txs).Error
	return txs, err
}

// GetInventory returns current items in stock (bought - sold).
func GetInventory() ([]InventoryItem, error) {
	db := database.DB

	type buyRow struct {
		ItemID   int
		ItemName string
		Qty      int
		AvgPrice float64
		Total    int64
	}
	var buys []buyRow
	db.Model(&models.Portfolio{}).
		Select("item_id, item_name, SUM(quantity) as qty, AVG(price_per_unit) as avg_price, SUM(total_price) as total").
		Where("action = ?", "BUY").
		Group("item_id, item_name").
		Scan(&buys)

	type sellRow struct {
		ItemID  int
		Qty     int
		Revenue int64
	}
	var sells []sellRow
	db.Model(&models.Portfolio{}).
		Select("item_id, SUM(quantity) as qty, SUM(total_price) as revenue").
		Where("action = ?", "SELL").
		Group("item_id").
		Scan(&sells)

	sellMap := make(map[int]sellRow, len(sells))
	for _, s := range sells {
		sellMap[s.ItemID] = s
	}

	var inventory []InventoryItem
	for _, b := range buys {
		s := sellMap[b.ItemID]
		remaining := b.Qty - s.Qty
		if remaining > 0 {
			inventory = append(inventory, InventoryItem{
				ItemID:        b.ItemID,
				ItemName:      b.ItemName,
				Quantity:      remaining,
				AvgBuyPrice:   int64(b.AvgPrice),
				TotalInvested: b.Total,
				TotalRevenue:  s.Revenue,
			})
		}
	}
	return inventory, nil
}

// GetPnlSummary returns the P&L summary.
func GetPnlSummary() PnlData {
	db := database.DB

	var totalBought, totalSold int64
	db.Model(&models.Portfolio{}).Where("action = ?", "BUY").
		Select("COALESCE(SUM(total_price), 0)").Scan(&totalBought)
	db.Model(&models.Portfolio{}).Where("action = ?", "SELL").
		Select("COALESCE(SUM(total_price), 0)").Scan(&totalSold)

	ahFees := int64(float64(totalSold) * 0.05)
	return PnlData{
		TotalInvestedCopper:  totalBought,
		TotalRevenueCopper:   totalSold,
		AHFeesCopper:         ahFees,
		RealizedProfitCopper: totalSold - ahFees - totalBought,
	}
}

// GetGoldHistory returns gold balance history for charts.
func GetGoldHistory(days int) ([]models.GoldBalance, error) {
	db := database.DB
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	var history []models.GoldBalance
	err := db.Where("recorded_at >= ?", cutoff).Order("recorded_at ASC").Find(&history).Error
	return history, err
}

func updateBalance() {
	db := database.DB
	pnl := GetPnlSummary()
	inv, _ := GetInventory()

	var invested int64
	for _, item := range inv {
		invested += item.AvgBuyPrice * int64(item.Quantity)
	}

	balance := models.GoldBalance{
		RecordedAt:     time.Now().UTC(),
		BalanceCopper:  pnl.RealizedProfitCopper,
		InvestedCopper: invested,
		ProfitCopper:   pnl.RealizedProfitCopper,
	}
	db.Create(&balance)
}
