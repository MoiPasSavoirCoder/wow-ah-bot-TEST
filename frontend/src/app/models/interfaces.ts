/**
 * TypeScript interfaces matching the backend Pydantic schemas.
 */

export interface DashboardSummary {
  total_invested_gold: string;
  total_profit_gold: string;
  current_balance_gold: string;
  active_deals: number;
  total_items_tracked: number;
  last_scan: string | null;
  gold_history: GoldBalance[];
  recent_deals: Deal[];
}

export interface Deal {
  id: number;
  item_id: number;
  item_name: string | null;
  icon_url: string | null;
  detected_at: string;
  current_price: number;
  avg_price: number;
  suggested_buy_price: number;
  suggested_sell_price: number;
  suggested_quantity: number;
  profit_margin: number;
  rentability_index: number;
  status: 'PENDING' | 'EXECUTED' | 'EXPIRED' | 'SKIPPED';
  notified: boolean;
  current_price_gold: string;
  avg_price_gold: string;
  suggested_buy_price_gold: string;
  suggested_sell_price_gold: string;
  potential_profit_gold: string;
}

export interface ItemScore {
  id: number;
  item_id: number;
  item_name: string;
  icon_url: string;
  scan_id: number;
  scored_at: string;
  score_undervaluation: number;
  score_momentum: number;
  score_liquidity: number;
  score_stability: number;
  score_net_profit: number;
  rentability_index: number;
  current_min_price: number;
  current_min_price_gold: string;
  hist_median_price: number;
  hist_median_price_gold: string;
  avg_daily_volume: number;
  price_slope: number;
  coeff_variation: number;
  data_points: number;
  weights: {
    undervaluation: number;
    momentum: number;
    liquidity: number;
    stability: number;
    net_profit: number;
  };
}

export interface PortfolioEntry {
  id: number;
  item_id: number;
  item_name: string | null;
  action: 'BUY' | 'SELL';
  quantity: number;
  price_per_unit: number;
  total_price: number;
  created_at: string;
  notes: string | null;
  total_price_gold: string;
}

export interface GoldBalance {
  recorded_at: string;
  balance_copper: number;
  invested_copper: number;
  profit_copper: number;
  balance_gold: string;
  profit_gold: string;
}

export interface PriceHistoryEntry {
  item_id: number;
  scanned_at: string;
  min_buyout: number | null;
  avg_buyout: number | null;
  median_buyout: number | null;
  total_quantity: number;
  num_auctions: number;
  min_buyout_gold: string;
  avg_buyout_gold: string;
}

export interface PriceHistoryList {
  item_id: number;
  item_name: string | null;
  history: PriceHistoryEntry[];
}

export interface InventoryItem {
  item_id: number;
  item_name: string | null;
  quantity: number;
  avg_buy_price: number;
  total_invested: number;
  total_revenue: number;
}

export interface PnlSummary {
  total_invested: string;
  total_revenue: string;
  ah_fees: string;
  realized_profit: string;
  total_invested_copper: number;
  total_revenue_copper: number;
  realized_profit_copper: number;
}

export interface WowItem {
  id: number;
  name: string | null;
  quality: string | null;
  item_class: string | null;
  item_subclass: string | null;
  level: number | null;
  icon_url: string | null;
  is_commodity: boolean;
}

// ─── Auction House ───

export interface AuctionHouseItem {
  item_id: number;
  item_name: string;
  icon_url: string;
  quality: string;
  item_class: string;
  item_subclass: string;
  level: number;
  min_buyout: number;
  min_buyout_gold: string;
  avg_buyout: number;
  avg_buyout_gold: string;
  market_price: number;
  market_price_gold: string;
  total_quantity: number;
  num_auctions: number;
  time_left: string;
}

export interface AuctionHouseResponse {
  items: AuctionHouseItem[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
  snapshot_id: number;
  scanned_at: string;
}

export interface AuctionHouseCategory {
  name: string;
  subcategories: { name: string; count: number }[];
  total: number;
}
