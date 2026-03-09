package models

import "time"

// Item — WoW item reference data (cached from Blizzard API).
type Item struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:255" json:"name"`
	Quality     string    `gorm:"size:50" json:"quality"`
	ItemClass   string    `gorm:"size:100" json:"item_class"`
	ItemSubclass string   `gorm:"size:100" json:"item_subclass"`
	Level       int       `json:"level"`
	IconURL     string    `gorm:"size:500" json:"icon_url"`
	IsCommodity bool      `gorm:"default:false" json:"is_commodity"`
	VendorPrice int64     `gorm:"default:0" json:"vendor_price"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// AuctionSnapshot — one AH scan run.
type AuctionSnapshot struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ScannedAt       time.Time `gorm:"index;autoCreateTime" json:"scanned_at"`
	TotalAuctions   int       `gorm:"default:0" json:"total_auctions"`
	TotalGoldVolume int64     `gorm:"default:0" json:"total_gold_volume"`
}

// AuctionEntry — individual auction from a snapshot.
type AuctionEntry struct {
	ID         uint  `gorm:"primaryKey;autoIncrement" json:"id"`
	SnapshotID uint  `gorm:"index" json:"snapshot_id"`
	AuctionID  int64 `json:"auction_id"`
	ItemID     int   `gorm:"index" json:"item_id"`
	Quantity   int   `gorm:"default:1" json:"quantity"`
	UnitPrice  int64 `json:"unit_price"`
	Buyout     int64 `json:"buyout"`
	Bid        int64 `json:"bid"`
	TimeLeft   string `gorm:"size:20" json:"time_left"`
}

func (AuctionEntry) TableName() string { return "auction_entries" }

// PriceHistory — aggregated price per item per scan.
type PriceHistory struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ItemID        int       `gorm:"index" json:"item_id"`
	ScannedAt     time.Time `gorm:"index;autoCreateTime" json:"scanned_at"`
	MinBuyout     int64     `json:"min_buyout"`
	AvgBuyout     int64     `json:"avg_buyout"`
	MedianBuyout  int64     `json:"median_buyout"`
	MaxBuyout     int64     `json:"max_buyout"`
	TotalQuantity int       `gorm:"default:0" json:"total_quantity"`
	NumAuctions   int       `gorm:"default:0" json:"num_auctions"`
}

func (PriceHistory) TableName() string { return "price_history" }

// Deal — detected trading opportunity.
type Deal struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ItemID             int       `gorm:"index" json:"item_id"`
	ItemName           string    `gorm:"size:255" json:"item_name"`
	DetectedAt         time.Time `gorm:"autoCreateTime" json:"detected_at"`
	CurrentPrice       int64     `json:"current_price"`
	AvgPrice           int64     `json:"avg_price"`
	SuggestedBuyPrice  int64     `json:"suggested_buy_price"`
	SuggestedSellPrice int64     `json:"suggested_sell_price"`
	SuggestedQuantity  int       `gorm:"default:1" json:"suggested_quantity"`
	ProfitMargin       float64   `json:"profit_margin"`
	RentabilityIndex   float64   `gorm:"default:0" json:"rentability_index"`
	Status             string    `gorm:"size:20;default:PENDING" json:"status"`
	Notified           bool      `gorm:"default:false" json:"notified"`
}

// ItemScore — detailed IR breakdown per item per scan.
// One row is inserted (or replaced) every time Analyze() runs.
type ItemScore struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ItemID         int       `gorm:"uniqueIndex:idx_item_scan" json:"item_id"`
	ScanID         uint      `gorm:"uniqueIndex:idx_item_scan" json:"scan_id"` // references AuctionSnapshot.ID
	ScoredAt       time.Time `gorm:"index;autoCreateTime" json:"scored_at"`

	// Raw component scores (0-100 each before weighting)
	ScoreUndervaluation float64 `gorm:"default:0" json:"score_undervaluation"` // 35 % — sous-évaluation vs médiane historique
	ScoreMomentum       float64 `gorm:"default:0" json:"score_momentum"`       // 20 % — pente baissière des prix
	ScoreLiquidity      float64 `gorm:"default:0" json:"score_liquidity"`      // 20 % — volume normalisé
	ScoreStability      float64 `gorm:"default:0" json:"score_stability"`      // 15 % — inverse CV
	ScoreNetProfit      float64 `gorm:"default:0" json:"score_net_profit"`     // 10 % — profit net après frais AH

	// Final weighted IR score 0-100
	RentabilityIndex float64 `gorm:"default:0" json:"rentability_index"`

	// Context values used in calculation
	CurrentMinPrice  int64   `json:"current_min_price"`
	HistMedianPrice  int64   `json:"hist_median_price"`
	AvgDailyVolume   float64 `json:"avg_daily_volume"`
	PriceSlope       float64 `json:"price_slope"`       // negative = falling = good
	CoeffVariation   float64 `json:"coeff_variation"`   // lower = more stable
	DataPoints       int     `json:"data_points"`
}

// Portfolio — actual buy/sell transaction.
type Portfolio struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ItemID       int       `gorm:"index" json:"item_id"`
	ItemName     string    `gorm:"size:255" json:"item_name"`
	Action       string    `gorm:"size:10" json:"action"` // BUY or SELL
	Quantity     int       `gorm:"default:1" json:"quantity"`
	PricePerUnit int64     `json:"price_per_unit"`
	TotalPrice   int64     `json:"total_price"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	Notes        string    `gorm:"size:500" json:"notes"`
}

// Character — a tracked WoW character.
type Character struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Realm     string    `gorm:"size:100;not null" json:"realm"`
	Class     string    `gorm:"size:50" json:"class"`
	Race      string    `gorm:"size:50" json:"race"`
	Level     int       `gorm:"default:0" json:"level"`
	AvatarURL string    `gorm:"size:500" json:"avatar_url"`
	Notes     string    `gorm:"size:500" json:"notes"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// CharacterTransaction — an AH transaction (buy or sell) linked to a character.
type CharacterTransaction struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CharacterID    uint      `gorm:"index;not null" json:"character_id"`
	ItemID         int       `gorm:"index" json:"item_id"`
	ItemName       string    `gorm:"size:255" json:"item_name"`
	IconURL        string    `gorm:"size:500" json:"icon_url"`
	Action         string    `gorm:"size:10;not null" json:"action"` // BUY or SELL
	Quantity       int       `gorm:"default:1" json:"quantity"`
	PricePerUnit   int64     `json:"price_per_unit"`
	TotalPrice     int64     `json:"total_price"`
	DealID         *uint     `gorm:"index" json:"deal_id"`    // optional link to a Deal
	TransactedAt   time.Time `gorm:"index;autoCreateTime" json:"transacted_at"`
	Notes          string    `gorm:"size:500" json:"notes"`
}

func (CharacterTransaction) TableName() string { return "character_transactions" }

// CharacterSnapshot — daily wealth snapshot for a character (for charting).
type CharacterSnapshot struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CharacterID    uint      `gorm:"index;not null" json:"character_id"`
	RecordedAt     time.Time `gorm:"index;autoCreateTime" json:"recorded_at"`
	BalanceCopper  int64     `gorm:"default:0" json:"balance_copper"`  // total gold on hand (manual input)
	InvestedCopper int64     `gorm:"default:0" json:"invested_copper"` // computed from open BUY positions
	ProfitCopper   int64     `gorm:"default:0" json:"profit_copper"`   // realized P&L
}

func (CharacterSnapshot) TableName() string { return "character_snapshots" }

// GoldBalance — daily gold balance snapshot for P&L chart.
type GoldBalance struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RecordedAt     time.Time `gorm:"index;autoCreateTime" json:"recorded_at"`
	BalanceCopper  int64     `gorm:"default:0" json:"balance_copper"`
	InvestedCopper int64     `gorm:"default:0" json:"invested_copper"`
	ProfitCopper   int64     `gorm:"default:0" json:"profit_copper"`
}

func (GoldBalance) TableName() string { return "gold_balance" }
