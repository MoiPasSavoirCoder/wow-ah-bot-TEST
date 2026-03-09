"""
Pydantic schemas for API responses.
"""

from pydantic import BaseModel, computed_field
from datetime import datetime


# ─── Utility ───

def copper_to_gold_str(copper: int | None) -> str:
    """Convert copper amount to human-readable gold string."""
    if copper is None:
        return "0g"
    gold = copper // 10000
    silver = (copper % 10000) // 100
    cop = copper % 100
    parts = []
    if gold:
        parts.append(f"{gold:,}g")
    if silver:
        parts.append(f"{silver}s")
    if cop:
        parts.append(f"{cop}c")
    return " ".join(parts) or "0g"


# ─── Items ───

class ItemSchema(BaseModel):
    id: int
    name: str | None = None
    quality: str | None = None
    item_class: str | None = None
    item_subclass: str | None = None
    level: int | None = None
    icon_url: str | None = None
    is_commodity: bool = False

    class Config:
        from_attributes = True


# ─── Price History ───

class PriceHistorySchema(BaseModel):
    item_id: int
    scanned_at: datetime
    min_buyout: int | None = None
    avg_buyout: int | None = None
    median_buyout: int | None = None
    total_quantity: int = 0
    num_auctions: int = 0

    @computed_field
    @property
    def min_buyout_gold(self) -> str:
        return copper_to_gold_str(self.min_buyout)

    @computed_field
    @property
    def avg_buyout_gold(self) -> str:
        return copper_to_gold_str(self.avg_buyout)

    class Config:
        from_attributes = True


class PriceHistoryListSchema(BaseModel):
    item_id: int
    item_name: str | None = None
    history: list[PriceHistorySchema] = []


# ─── Deals ───

class DealSchema(BaseModel):
    id: int
    item_id: int
    item_name: str | None = None
    icon_url: str | None = None
    detected_at: datetime
    deal_type: str = "BUY"
    current_price: int
    avg_price: int
    suggested_buy_price: int | None = None
    suggested_sell_price: int | None = None
    suggested_quantity: int = 1
    profit_margin: float
    confidence_score: float
    status: str = "PENDING"
    notified: bool = False

    @computed_field
    @property
    def current_price_gold(self) -> str:
        return copper_to_gold_str(self.current_price)

    @computed_field
    @property
    def avg_price_gold(self) -> str:
        return copper_to_gold_str(self.avg_price)

    @computed_field
    @property
    def suggested_buy_price_gold(self) -> str:
        return copper_to_gold_str(self.suggested_buy_price)

    @computed_field
    @property
    def suggested_sell_price_gold(self) -> str:
        return copper_to_gold_str(self.suggested_sell_price)

    @computed_field
    @property
    def potential_profit_gold(self) -> str:
        if self.suggested_sell_price and self.suggested_buy_price:
            profit = (self.suggested_sell_price - self.suggested_buy_price) * self.suggested_quantity * 95 // 100
            return copper_to_gold_str(max(0, profit))
        return "N/A"

    class Config:
        from_attributes = True


# ─── Portfolio ───

class PortfolioEntrySchema(BaseModel):
    id: int
    item_id: int
    item_name: str | None = None
    action: str
    quantity: int
    price_per_unit: int
    total_price: int
    created_at: datetime
    notes: str | None = None

    @computed_field
    @property
    def total_price_gold(self) -> str:
        return copper_to_gold_str(self.total_price)

    class Config:
        from_attributes = True


class PortfolioAddRequest(BaseModel):
    item_id: int
    item_name: str | None = None
    action: str  # BUY or SELL
    quantity: int
    price_per_unit: int
    notes: str | None = None


# ─── Gold Balance / P&L ───

class GoldBalanceSchema(BaseModel):
    recorded_at: datetime
    balance_copper: int = 0
    invested_copper: int = 0
    profit_copper: int = 0

    @computed_field
    @property
    def balance_gold(self) -> str:
        return copper_to_gold_str(self.balance_copper)

    @computed_field
    @property
    def profit_gold(self) -> str:
        return copper_to_gold_str(self.profit_copper)

    class Config:
        from_attributes = True


# ─── Dashboard Summary ───

class DashboardSummary(BaseModel):
    total_invested_gold: str
    total_profit_gold: str
    current_balance_gold: str
    active_deals: int
    total_items_tracked: int
    last_scan: datetime | None = None
    gold_history: list[GoldBalanceSchema] = []
    recent_deals: list[DealSchema] = []
