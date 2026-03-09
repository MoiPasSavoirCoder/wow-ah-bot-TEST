"""
SQLAlchemy models for the WoW AH Trading Bot.
"""

from sqlalchemy import Column, Integer, Float, String, DateTime, Boolean, BigInteger, Index
from sqlalchemy.sql import func
from backend.core.database import Base


class Item(Base):
    """WoW Item reference data."""
    __tablename__ = "items"

    id = Column(Integer, primary_key=True, doc="Blizzard item ID")
    name = Column(String(255), nullable=True, doc="Item name (fr_FR)")
    quality = Column(String(50), nullable=True, doc="COMMON, UNCOMMON, RARE, EPIC, LEGENDARY")
    item_class = Column(String(100), nullable=True, doc="Item class (Weapon, Armor, etc.)")
    item_subclass = Column(String(100), nullable=True, doc="Item subclass")
    level = Column(Integer, nullable=True)
    icon_url = Column(String(500), nullable=True)
    is_commodity = Column(Boolean, default=False, doc="True if tradeable on commodities AH")
    vendor_price = Column(BigInteger, default=0, doc="Vendor sell price in copper")
    updated_at = Column(DateTime, server_default=func.now(), onupdate=func.now())


class AuctionSnapshot(Base):
    """A single AH scan snapshot."""
    __tablename__ = "auction_snapshots"

    id = Column(Integer, primary_key=True, autoincrement=True)
    scanned_at = Column(DateTime, server_default=func.now(), index=True)
    total_auctions = Column(Integer, default=0)
    total_gold_volume = Column(BigInteger, default=0, doc="Total gold volume in copper")


class AuctionEntry(Base):
    """Individual auction entry from a snapshot."""
    __tablename__ = "auction_entries"

    id = Column(Integer, primary_key=True, autoincrement=True)
    snapshot_id = Column(Integer, nullable=False, index=True)
    auction_id = Column(BigInteger, nullable=False)
    item_id = Column(Integer, nullable=False, index=True)
    quantity = Column(Integer, default=1)
    unit_price = Column(BigInteger, nullable=True, doc="Price per unit in copper")
    buyout = Column(BigInteger, nullable=True, doc="Buyout price in copper")
    bid = Column(BigInteger, nullable=True, doc="Current bid in copper")
    time_left = Column(String(20), nullable=True, doc="SHORT, MEDIUM, LONG, VERY_LONG")

    __table_args__ = (
        Index("idx_auction_item_snapshot", "snapshot_id", "item_id"),
    )


class PriceHistory(Base):
    """Aggregated price history per item (1 entry per scan per item)."""
    __tablename__ = "price_history"

    id = Column(Integer, primary_key=True, autoincrement=True)
    item_id = Column(Integer, nullable=False, index=True)
    scanned_at = Column(DateTime, server_default=func.now(), index=True)
    min_buyout = Column(BigInteger, nullable=True, doc="Minimum buyout in copper")
    avg_buyout = Column(BigInteger, nullable=True, doc="Average buyout in copper")
    median_buyout = Column(BigInteger, nullable=True, doc="Median buyout in copper")
    max_buyout = Column(BigInteger, nullable=True)
    total_quantity = Column(Integer, default=0)
    num_auctions = Column(Integer, default=0)

    __table_args__ = (
        Index("idx_price_item_date", "item_id", "scanned_at"),
    )


class Deal(Base):
    """Detected deal / opportunity."""
    __tablename__ = "deals"

    id = Column(Integer, primary_key=True, autoincrement=True)
    item_id = Column(Integer, nullable=False, index=True)
    item_name = Column(String(255), nullable=True)
    detected_at = Column(DateTime, server_default=func.now())
    deal_type = Column(String(20), default="BUY", doc="BUY or SELL")
    current_price = Column(BigInteger, doc="Current min price in copper")
    avg_price = Column(BigInteger, doc="Historical avg price in copper")
    suggested_buy_price = Column(BigInteger, nullable=True)
    suggested_sell_price = Column(BigInteger, nullable=True)
    suggested_quantity = Column(Integer, default=1)
    profit_margin = Column(Float, doc="Expected profit margin %")
    confidence_score = Column(Float, doc="0-100 confidence score")
    status = Column(String(20), default="PENDING", doc="PENDING, EXECUTED, EXPIRED, SKIPPED")
    notified = Column(Boolean, default=False, doc="Discord notification sent")


class Portfolio(Base):
    """Track actual buys and sells."""
    __tablename__ = "portfolio"

    id = Column(Integer, primary_key=True, autoincrement=True)
    item_id = Column(Integer, nullable=False, index=True)
    item_name = Column(String(255), nullable=True)
    action = Column(String(10), nullable=False, doc="BUY or SELL")
    quantity = Column(Integer, default=1)
    price_per_unit = Column(BigInteger, doc="Price per unit in copper")
    total_price = Column(BigInteger, doc="Total price in copper")
    created_at = Column(DateTime, server_default=func.now())
    notes = Column(String(500), nullable=True)


class GoldBalance(Base):
    """Daily gold balance tracking for P&L chart."""
    __tablename__ = "gold_balance"

    id = Column(Integer, primary_key=True, autoincrement=True)
    recorded_at = Column(DateTime, server_default=func.now(), index=True)
    balance_copper = Column(BigInteger, default=0, doc="Total gold balance in copper")
    invested_copper = Column(BigInteger, default=0, doc="Gold invested in items")
    profit_copper = Column(BigInteger, default=0, doc="Realized profit in copper")
