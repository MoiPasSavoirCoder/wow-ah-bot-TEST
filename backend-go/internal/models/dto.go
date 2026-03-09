package models

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// RoundFloat rounds a float64 to n decimal places.
func RoundFloat(v float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(v*pow) / pow
}

func CopperToGoldStr(copper int64) string {
	if copper == 0 {
		return "0g"
	}
	negative := copper < 0
	if negative {
		copper = -copper
	}
	gold := copper / 10000
	silver := (copper % 10000) / 100
	cop := copper % 100
	var parts []string
	if gold > 0 {
		parts = append(parts, fmt.Sprintf("%dg", gold))
	}
	if silver > 0 {
		parts = append(parts, fmt.Sprintf("%ds", silver))
	}
	if cop > 0 {
		parts = append(parts, fmt.Sprintf("%dc", cop))
	}
	result := strings.Join(parts, " ")
	if result == "" {
		return "0g"
	}
	if negative {
		return "-" + result
	}
	return result
}

// ════════════════════════════════════════
// DTOs (JSON response schemas)
// ════════════════════════════════════════

// DealDTO — enriched deal for API response.
type DealDTO struct {
	ID                 uint      `json:"id"`
	ItemID             int       `json:"item_id"`
	ItemName           string    `json:"item_name"`
	IconURL            string    `json:"icon_url"`
	DetectedAt         time.Time `json:"detected_at"`
	CurrentPrice       int64     `json:"current_price"`
	AvgPrice           int64     `json:"avg_price"`
	SuggestedBuyPrice  int64     `json:"suggested_buy_price"`
	SuggestedSellPrice int64     `json:"suggested_sell_price"`
	SuggestedQuantity  int       `json:"suggested_quantity"`
	ProfitMargin       float64   `json:"profit_margin"`
	RentabilityIndex   float64   `json:"rentability_index"`
	Status             string    `json:"status"`
	Notified           bool      `json:"notified"`

	// Computed gold strings
	CurrentPriceGold       string `json:"current_price_gold"`
	AvgPriceGold           string `json:"avg_price_gold"`
	SuggestedBuyPriceGold  string `json:"suggested_buy_price_gold"`
	SuggestedSellPriceGold string `json:"suggested_sell_price_gold"`
	PotentialProfitGold    string `json:"potential_profit_gold"`
}

// NewDealDTO builds a DealDTO from a Deal + optional item data.
func NewDealDTO(d Deal, itemName, iconURL string) DealDTO {
	name := itemName
	if name == "" {
		name = d.ItemName
	}
	if name == "" {
		name = fmt.Sprintf("Item #%d", d.ItemID)
	}

	potentialProfit := "N/A"
	if d.SuggestedSellPrice > 0 && d.SuggestedBuyPrice > 0 {
		profit := (d.SuggestedSellPrice - d.SuggestedBuyPrice) * int64(d.SuggestedQuantity) * 95 / 100
		if profit < 0 {
			profit = 0
		}
		potentialProfit = CopperToGoldStr(profit)
	}

	return DealDTO{
		ID:                 d.ID,
		ItemID:             d.ItemID,
		ItemName:           name,
		IconURL:            iconURL,
		DetectedAt:         d.DetectedAt,
		CurrentPrice:       d.CurrentPrice,
		AvgPrice:           d.AvgPrice,
		SuggestedBuyPrice:  d.SuggestedBuyPrice,
		SuggestedSellPrice: d.SuggestedSellPrice,
		SuggestedQuantity:  d.SuggestedQuantity,
		ProfitMargin:       d.ProfitMargin,
		RentabilityIndex:   d.RentabilityIndex,
		Status:             d.Status,
		Notified:           d.Notified,

		CurrentPriceGold:       CopperToGoldStr(d.CurrentPrice),
		AvgPriceGold:           CopperToGoldStr(d.AvgPrice),
		SuggestedBuyPriceGold:  CopperToGoldStr(d.SuggestedBuyPrice),
		SuggestedSellPriceGold: CopperToGoldStr(d.SuggestedSellPrice),
		PotentialProfitGold:    potentialProfit,
	}
}

// ItemScoreDTO — detailed IR breakdown for one item, for the /api/scores endpoint.
type ItemScoreDTO struct {
	ID             uint      `json:"id"`
	ItemID         int       `json:"item_id"`
	ItemName       string    `json:"item_name"`
	IconURL        string    `json:"icon_url"`
	ScanID         uint      `json:"scan_id"`
	ScoredAt       time.Time `json:"scored_at"`

	// Individual components (0-100)
	ScoreUndervaluation float64 `json:"score_undervaluation"`
	ScoreMomentum       float64 `json:"score_momentum"`
	ScoreLiquidity      float64 `json:"score_liquidity"`
	ScoreStability      float64 `json:"score_stability"`
	ScoreNetProfit      float64 `json:"score_net_profit"`

	// Weighted IR
	RentabilityIndex float64 `json:"rentability_index"`

	// Context
	CurrentMinPrice     int64   `json:"current_min_price"`
	CurrentMinPriceGold string  `json:"current_min_price_gold"`
	HistMedianPrice     int64   `json:"hist_median_price"`
	HistMedianPriceGold string  `json:"hist_median_price_gold"`
	AvgDailyVolume      float64 `json:"avg_daily_volume"`
	PriceSlope          float64 `json:"price_slope"`
	CoeffVariation      float64 `json:"coeff_variation"`
	DataPoints          int     `json:"data_points"`

	// Weight breakdown for display
	Weights IRWeights `json:"weights"`
}

// IRWeights shows the weight of each component in the final IR score.
type IRWeights struct {
	Undervaluation float64 `json:"undervaluation"`
	Momentum       float64 `json:"momentum"`
	Liquidity      float64 `json:"liquidity"`
	Stability      float64 `json:"stability"`
	NetProfit      float64 `json:"net_profit"`
}

// DefaultIRWeights returns the canonical component weights (must sum to 1.0).
func DefaultIRWeights() IRWeights {
	return IRWeights{
		Undervaluation: 0.35,
		Momentum:       0.20,
		Liquidity:      0.20,
		Stability:      0.15,
		NetProfit:      0.10,
	}
}

func NewItemScoreDTO(s ItemScore, itemName, iconURL string) ItemScoreDTO {
	name := itemName
	if name == "" {
		name = fmt.Sprintf("Item #%d", s.ItemID)
	}
	return ItemScoreDTO{
		ID:                  s.ID,
		ItemID:              s.ItemID,
		ItemName:            name,
		IconURL:             iconURL,
		ScanID:              s.ScanID,
		ScoredAt:            s.ScoredAt,
		ScoreUndervaluation: s.ScoreUndervaluation,
		ScoreMomentum:       s.ScoreMomentum,
		ScoreLiquidity:      s.ScoreLiquidity,
		ScoreStability:      s.ScoreStability,
		ScoreNetProfit:      s.ScoreNetProfit,
		RentabilityIndex:    s.RentabilityIndex,
		CurrentMinPrice:     s.CurrentMinPrice,
		CurrentMinPriceGold: CopperToGoldStr(s.CurrentMinPrice),
		HistMedianPrice:     s.HistMedianPrice,
		HistMedianPriceGold: CopperToGoldStr(s.HistMedianPrice),
		AvgDailyVolume:      s.AvgDailyVolume,
		PriceSlope:          s.PriceSlope,
		CoeffVariation:      s.CoeffVariation,
		DataPoints:          s.DataPoints,
		Weights:             DefaultIRWeights(),
	}
}

// PortfolioDTO — portfolio entry for API response.
type PortfolioDTO struct {
	ID           uint      `json:"id"`
	ItemID       int       `json:"item_id"`
	ItemName     string    `json:"item_name"`
	Action       string    `json:"action"`
	Quantity     int       `json:"quantity"`
	PricePerUnit int64     `json:"price_per_unit"`
	TotalPrice   int64     `json:"total_price"`
	CreatedAt    time.Time `json:"created_at"`
	Notes        string    `json:"notes"`
	TotalPriceGold string `json:"total_price_gold"`
}

func NewPortfolioDTO(p Portfolio) PortfolioDTO {
	return PortfolioDTO{
		ID:             p.ID,
		ItemID:         p.ItemID,
		ItemName:       p.ItemName,
		Action:         p.Action,
		Quantity:        p.Quantity,
		PricePerUnit:   p.PricePerUnit,
		TotalPrice:     p.TotalPrice,
		CreatedAt:      p.CreatedAt,
		Notes:          p.Notes,
		TotalPriceGold: CopperToGoldStr(p.TotalPrice),
	}
}

// GoldBalanceDTO — gold balance for API response.
type GoldBalanceDTO struct {
	RecordedAt     time.Time `json:"recorded_at"`
	BalanceCopper  int64     `json:"balance_copper"`
	InvestedCopper int64     `json:"invested_copper"`
	ProfitCopper   int64     `json:"profit_copper"`
	BalanceGold    string    `json:"balance_gold"`
	ProfitGold     string    `json:"profit_gold"`
}

func NewGoldBalanceDTO(g GoldBalance) GoldBalanceDTO {
	return GoldBalanceDTO{
		RecordedAt:     g.RecordedAt,
		BalanceCopper:  g.BalanceCopper,
		InvestedCopper: g.InvestedCopper,
		ProfitCopper:   g.ProfitCopper,
		BalanceGold:    CopperToGoldStr(g.BalanceCopper),
		ProfitGold:     CopperToGoldStr(g.ProfitCopper),
	}
}

// PriceHistoryDTO — price history entry for API response.
type PriceHistoryDTO struct {
	ItemID        int       `json:"item_id"`
	ScannedAt     time.Time `json:"scanned_at"`
	MinBuyout     int64     `json:"min_buyout"`
	AvgBuyout     int64     `json:"avg_buyout"`
	MedianBuyout  int64     `json:"median_buyout"`
	TotalQuantity int       `json:"total_quantity"`
	NumAuctions   int       `json:"num_auctions"`
	MinBuyoutGold string    `json:"min_buyout_gold"`
	AvgBuyoutGold string    `json:"avg_buyout_gold"`
}

func NewPriceHistoryDTO(p PriceHistory) PriceHistoryDTO {
	return PriceHistoryDTO{
		ItemID:        p.ItemID,
		ScannedAt:     p.ScannedAt,
		MinBuyout:     p.MinBuyout,
		AvgBuyout:     p.AvgBuyout,
		MedianBuyout:  p.MedianBuyout,
		TotalQuantity: p.TotalQuantity,
		NumAuctions:   p.NumAuctions,
		MinBuyoutGold: CopperToGoldStr(p.MinBuyout),
		AvgBuyoutGold: CopperToGoldStr(p.AvgBuyout),
	}
}

// PriceHistoryListDTO — price history list for an item.
type PriceHistoryListDTO struct {
	ItemID   int               `json:"item_id"`
	ItemName string            `json:"item_name"`
	History  []PriceHistoryDTO `json:"history"`
}

// DashboardSummaryDTO — dashboard overview.
type DashboardSummaryDTO struct {
	TotalInvestedGold  string           `json:"total_invested_gold"`
	TotalProfitGold    string           `json:"total_profit_gold"`
	CurrentBalanceGold string           `json:"current_balance_gold"`
	ActiveDeals        int              `json:"active_deals"`
	TotalItemsTracked  int              `json:"total_items_tracked"`
	LastScan           *time.Time       `json:"last_scan"`
	GoldHistory        []GoldBalanceDTO `json:"gold_history"`
	RecentDeals        []DealDTO        `json:"recent_deals"`
}

// ItemDTO — item for search results.
type ItemDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Quality     string `json:"quality"`
	ItemClass   string `json:"item_class"`
	ItemSubclass string `json:"item_subclass"`
	Level       int    `json:"level"`
	IconURL     string `json:"icon_url"`
	IsCommodity bool   `json:"is_commodity"`
}

func NewItemDTO(i Item) ItemDTO {
	return ItemDTO{
		ID: i.ID, Name: i.Name, Quality: i.Quality,
		ItemClass: i.ItemClass, ItemSubclass: i.ItemSubclass,
		Level: i.Level, IconURL: i.IconURL, IsCommodity: i.IsCommodity,
	}
}

// PortfolioAddRequest — request body for adding a transaction.
type PortfolioAddRequest struct {
	ItemID       int    `json:"item_id" binding:"required"`
	ItemName     string `json:"item_name"`
	Action       string `json:"action" binding:"required"`
	Quantity     int    `json:"quantity" binding:"required,min=1"`
	PricePerUnit int64  `json:"price_per_unit" binding:"required"`
	Notes        string `json:"notes"`
}

// PnlSummaryDTO — P&L summary for API response.
type PnlSummaryDTO struct {
	TotalInvested       string `json:"total_invested"`
	TotalRevenue        string `json:"total_revenue"`
	AHFees              string `json:"ah_fees"`
	RealizedProfit      string `json:"realized_profit"`
	TotalInvestedCopper int64  `json:"total_invested_copper"`
	TotalRevenueCopper  int64  `json:"total_revenue_copper"`
	RealizedProfitCopper int64 `json:"realized_profit_copper"`
}

// ════════════════════════════════════════
// Character DTOs
// ════════════════════════════════════════

// CharacterDTO — character for API response.
type CharacterDTO struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Realm     string    `json:"realm"`
	Class     string    `json:"class"`
	Race      string    `json:"race"`
	Level     int       `json:"level"`
	AvatarURL string    `json:"avatar_url"`
	Notes     string    `json:"notes"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewCharacterDTO(c Character) CharacterDTO {
	return CharacterDTO{
		ID:        c.ID,
		Name:      c.Name,
		Realm:     c.Realm,
		Class:     c.Class,
		Race:      c.Race,
		Level:     c.Level,
		AvatarURL: c.AvatarURL,
		Notes:     c.Notes,
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// CharacterTransactionDTO — enriched character transaction for API response.
type CharacterTransactionDTO struct {
	ID           uint      `json:"id"`
	CharacterID  uint      `json:"character_id"`
	ItemID       int       `json:"item_id"`
	ItemName     string    `json:"item_name"`
	IconURL      string    `json:"icon_url"`
	Action       string    `json:"action"`
	Quantity     int       `json:"quantity"`
	PricePerUnit int64     `json:"price_per_unit"`
	TotalPrice   int64     `json:"total_price"`
	DealID       *uint     `json:"deal_id"`
	TransactedAt time.Time `json:"transacted_at"`
	Notes        string    `json:"notes"`

	// Gold strings
	PricePerUnitGold string `json:"price_per_unit_gold"`
	TotalPriceGold   string `json:"total_price_gold"`
}

func NewCharacterTransactionDTO(t CharacterTransaction) CharacterTransactionDTO {
	return CharacterTransactionDTO{
		ID:               t.ID,
		CharacterID:      t.CharacterID,
		ItemID:           t.ItemID,
		ItemName:         t.ItemName,
		IconURL:          t.IconURL,
		Action:           t.Action,
		Quantity:         t.Quantity,
		PricePerUnit:     t.PricePerUnit,
		TotalPrice:       t.TotalPrice,
		DealID:           t.DealID,
		TransactedAt:     t.TransactedAt,
		Notes:            t.Notes,
		PricePerUnitGold: CopperToGoldStr(t.PricePerUnit),
		TotalPriceGold:   CopperToGoldStr(t.TotalPrice),
	}
}

// CharacterSnapshotDTO — wealth snapshot for API response.
type CharacterSnapshotDTO struct {
	CharacterID    uint      `json:"character_id"`
	RecordedAt     time.Time `json:"recorded_at"`
	BalanceCopper  int64     `json:"balance_copper"`
	InvestedCopper int64     `json:"invested_copper"`
	ProfitCopper   int64     `json:"profit_copper"`
	BalanceGold    string    `json:"balance_gold"`
	InvestedGold   string    `json:"invested_gold"`
	ProfitGold     string    `json:"profit_gold"`
}

func NewCharacterSnapshotDTO(s CharacterSnapshot) CharacterSnapshotDTO {
	return CharacterSnapshotDTO{
		CharacterID:    s.CharacterID,
		RecordedAt:     s.RecordedAt,
		BalanceCopper:  s.BalanceCopper,
		InvestedCopper: s.InvestedCopper,
		ProfitCopper:   s.ProfitCopper,
		BalanceGold:    CopperToGoldStr(s.BalanceCopper),
		InvestedGold:   CopperToGoldStr(s.InvestedCopper),
		ProfitGold:     CopperToGoldStr(s.ProfitCopper),
	}
}

// CharacterPnlDTO — full P&L summary for a character.
type CharacterPnlDTO struct {
	CharacterID  uint `json:"character_id"`
	CharacterDTO      // embedded: name, realm, class, etc.

	// Aggregated from character_transactions
	TotalBuyCopper    int64 `json:"total_buy_copper"`
	TotalSellCopper   int64 `json:"total_sell_copper"`
	AHFeesCopper      int64 `json:"ah_fees_copper"`
	RealizedPnlCopper int64 `json:"realized_pnl_copper"`
	OpenPositionCopper int64 `json:"open_position_copper"` // invested but not yet sold

	// Gold strings
	TotalBuyGold    string `json:"total_buy_gold"`
	TotalSellGold   string `json:"total_sell_gold"`
	AHFeesGold      string `json:"ah_fees_gold"`
	RealizedPnlGold string `json:"realized_pnl_gold"`
	OpenPositionGold string `json:"open_position_gold"`

	// Stats
	TotalTransactions int     `json:"total_transactions"`
	TotalItemsBought  int     `json:"total_items_bought"`
	TotalItemsSold    int     `json:"total_items_sold"`
	WinRate           float64 `json:"win_rate"` // % of sell transactions that were profitable

	// Latest snapshot (current gold)
	LatestBalance     int64  `json:"latest_balance_copper"`
	LatestBalanceGold string `json:"latest_balance_gold"`
}

// CharacterAddRequest — request body for creating a character.
type CharacterAddRequest struct {
	Name      string `json:"name"      binding:"required"`
	Realm     string `json:"realm"     binding:"required"`
	Class     string `json:"class"`
	Race      string `json:"race"`
	Level     int    `json:"level"`
	AvatarURL string `json:"avatar_url"`
	Notes     string `json:"notes"`
}

// CharacterUpdateRequest — request body for updating a character.
type CharacterUpdateRequest struct {
	Name      string `json:"name"`
	Realm     string `json:"realm"`
	Class     string `json:"class"`
	Race      string `json:"race"`
	Level     int    `json:"level"`
	AvatarURL string `json:"avatar_url"`
	Notes     string `json:"notes"`
	IsActive  *bool  `json:"is_active"`
}

// CharacterTransactionAddRequest — request body for adding a character transaction.
type CharacterTransactionAddRequest struct {
	ItemID       int    `json:"item_id"        binding:"required"`
	ItemName     string `json:"item_name"`
	IconURL      string `json:"icon_url"`
	Action       string `json:"action"         binding:"required"` // BUY or SELL
	Quantity     int    `json:"quantity"       binding:"required,min=1"`
	PricePerUnit int64  `json:"price_per_unit" binding:"required"`
	DealID       *uint  `json:"deal_id"`
	Notes        string `json:"notes"`
}

// CharacterSnapshotAddRequest — request body for recording a character's gold balance.
type CharacterSnapshotAddRequest struct {
	BalanceCopper int64 `json:"balance_copper" binding:"required"`
}
