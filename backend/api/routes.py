"""
FastAPI Routes - Dashboard & Trading API endpoints.
"""

import logging
from fastapi import APIRouter, HTTPException, Query
from datetime import datetime
from sqlalchemy import select, func, desc
from typing import Optional

logger = logging.getLogger(__name__)

from backend.core.database import async_session
from backend.models.models import (
    Deal, Portfolio, GoldBalance, PriceHistory, Item, AuctionSnapshot,
)
from backend.models.schemas import (
    DealSchema, PortfolioEntrySchema, PortfolioAddRequest,
    GoldBalanceSchema, PriceHistoryListSchema, PriceHistorySchema,
    DashboardSummary, ItemSchema, copper_to_gold_str,
)
from backend.services.ah_scanner import ah_scanner
from backend.services.trading_engine import trading_engine
from backend.services.portfolio_service import portfolio_service
from backend.bot.discord_bot import discord_bot

router = APIRouter()


# ════════════════════════════════════════
# Dashboard
# ════════════════════════════════════════

@router.get("/dashboard", response_model=DashboardSummary)
async def get_dashboard():
    """Get dashboard summary data."""
    async with async_session() as session:
        # P&L
        pnl = await portfolio_service.get_pnl_summary()

        # Active deals count
        deal_count = await session.execute(
            select(func.count(Deal.id)).where(Deal.status == "PENDING")
        )
        active_deals = deal_count.scalar() or 0

        # Tracked items
        item_count = await session.execute(select(func.count(Item.id)))
        total_items = item_count.scalar() or 0

        # Last scan
        last_scan_result = await session.execute(
            select(AuctionSnapshot.scanned_at)
            .order_by(desc(AuctionSnapshot.scanned_at))
            .limit(1)
        )
        last_scan = last_scan_result.scalar()

        # Gold history
        gold_history = await portfolio_service.get_gold_history(days=30)

        # Recent deals
        recent = await session.execute(
            select(Deal)
            .order_by(desc(Deal.detected_at))
            .limit(10)
        )
        recent_deals = list(recent.scalars().all())

        # Inventory value
        inventory = await portfolio_service.get_inventory()
        invested = sum(item["avg_buy_price"] * item["quantity"] for item in inventory)

        return DashboardSummary(
            total_invested_gold=copper_to_gold_str(invested),
            total_profit_gold=copper_to_gold_str(pnl["realized_profit_copper"]),
            current_balance_gold=copper_to_gold_str(
                pnl["realized_profit_copper"] + invested
            ),
            active_deals=active_deals,
            total_items_tracked=total_items,
            last_scan=last_scan,
            gold_history=[GoldBalanceSchema.model_validate(g) for g in gold_history],
            recent_deals=[DealSchema.model_validate(d) for d in recent_deals],
        )


# ════════════════════════════════════════
# Deals
# ════════════════════════════════════════

@router.get("/deals", response_model=list[DealSchema])
async def get_deals(
    status: Optional[str] = Query(None, description="Filter by status: PENDING, EXECUTED, EXPIRED"),
    deal_type: Optional[str] = Query(None, description="Filter by type: BUY or SELL"),
    limit: int = Query(100, ge=1, le=500),
):
    """Get detected deals/opportunities enriched with item name and icon."""
    async with async_session() as session:
        query = (
            select(Deal, Item.name, Item.icon_url)
            .outerjoin(Item, Item.id == Deal.item_id)
            .order_by(
                desc(Deal.profit_margin * Deal.confidence_score)
            )
        )

        if status:
            query = query.where(Deal.status == status.upper())
        if deal_type:
            query = query.where(Deal.deal_type == deal_type.upper())

        query = query.limit(limit)
        result = await session.execute(query)
        rows = result.all()

        schemas = []
        for deal, item_name, icon_url in rows:
            data = {c.key: getattr(deal, c.key) for c in Deal.__table__.columns}
            data["item_name"] = item_name or deal.item_name or f"Item #{deal.item_id}"
            data["icon_url"] = icon_url or None
            schemas.append(DealSchema.model_validate(data))
        return schemas


@router.post("/deals/{deal_id}/execute")
async def execute_deal(deal_id: int):
    """Mark a deal as executed and add to portfolio."""
    async with async_session() as session:
        result = await session.execute(select(Deal).where(Deal.id == deal_id))
        deal = result.scalar_one_or_none()

        if not deal:
            raise HTTPException(status_code=404, detail="Deal not found")

        # Add to portfolio
        await portfolio_service.add_transaction(
            item_id=deal.item_id,
            item_name=deal.item_name,
            action=deal.deal_type,
            quantity=deal.suggested_quantity,
            price_per_unit=deal.suggested_buy_price or deal.current_price,
            notes=f"Deal #{deal.id} - Confidence: {deal.confidence_score}%",
        )

        # Mark deal as executed
        deal.status = "EXECUTED"
        await session.commit()

        return {"message": f"Deal #{deal_id} executed and added to portfolio"}


@router.post("/deals/{deal_id}/skip")
async def skip_deal(deal_id: int):
    """Mark a deal as skipped."""
    async with async_session() as session:
        result = await session.execute(select(Deal).where(Deal.id == deal_id))
        deal = result.scalar_one_or_none()
        if not deal:
            raise HTTPException(status_code=404, detail="Deal not found")
        deal.status = "SKIPPED"
        await session.commit()
        return {"message": f"Deal #{deal_id} skipped"}


# ════════════════════════════════════════
# Portfolio
# ════════════════════════════════════════

@router.get("/portfolio", response_model=list[PortfolioEntrySchema])
async def get_portfolio(limit: int = Query(100, ge=1, le=500)):
    """Get portfolio transactions."""
    transactions = await portfolio_service.get_transactions(limit=limit)
    return [PortfolioEntrySchema.model_validate(t) for t in transactions]


@router.post("/portfolio", response_model=PortfolioEntrySchema)
async def add_portfolio_entry(request: PortfolioAddRequest):
    """Manually add a buy/sell transaction."""
    entry = await portfolio_service.add_transaction(
        item_id=request.item_id,
        item_name=request.item_name,
        action=request.action,
        quantity=request.quantity,
        price_per_unit=request.price_per_unit,
        notes=request.notes,
    )
    return PortfolioEntrySchema.model_validate(entry)


@router.get("/portfolio/inventory")
async def get_inventory():
    """Get current inventory (items in stock)."""
    return await portfolio_service.get_inventory()


@router.get("/portfolio/pnl")
async def get_pnl():
    """Get P&L summary."""
    pnl = await portfolio_service.get_pnl_summary()
    return {
        "total_invested": copper_to_gold_str(pnl["total_invested_copper"]),
        "total_revenue": copper_to_gold_str(pnl["total_revenue_copper"]),
        "ah_fees": copper_to_gold_str(pnl["ah_fees_copper"]),
        "realized_profit": copper_to_gold_str(pnl["realized_profit_copper"]),
        "total_invested_copper": pnl["total_invested_copper"],
        "total_revenue_copper": pnl["total_revenue_copper"],
        "realized_profit_copper": pnl["realized_profit_copper"],
    }


# ════════════════════════════════════════
# Gold History (for charts)
# ════════════════════════════════════════

@router.get("/gold-history", response_model=list[GoldBalanceSchema])
async def get_gold_history(days: int = Query(30, ge=1, le=365)):
    """Get gold balance history for P&L chart."""
    history = await portfolio_service.get_gold_history(days=days)
    return [GoldBalanceSchema.model_validate(h) for h in history]


# ════════════════════════════════════════
# Price History
# ════════════════════════════════════════

@router.get("/prices/{item_id}", response_model=PriceHistoryListSchema)
async def get_price_history(item_id: int, days: int = Query(7, ge=1, le=90)):
    """Get price history for a specific item."""
    from datetime import timedelta
    cutoff = datetime.utcnow() - timedelta(days=days)

    async with async_session() as session:
        # Get item name
        item_result = await session.execute(select(Item).where(Item.id == item_id))
        item = item_result.scalar_one_or_none()

        # Get price history
        history_result = await session.execute(
            select(PriceHistory)
            .where(PriceHistory.item_id == item_id, PriceHistory.scanned_at >= cutoff)
            .order_by(PriceHistory.scanned_at.asc())
        )
        history = history_result.scalars().all()

        return PriceHistoryListSchema(
            item_id=item_id,
            item_name=item.name if item else None,
            history=[PriceHistorySchema.model_validate(h) for h in history],
        )


# ════════════════════════════════════════
# Items
# ════════════════════════════════════════

@router.get("/items/search", response_model=list[ItemSchema])
async def search_items(q: str = Query(..., min_length=2), limit: int = Query(20, ge=1, le=100)):
    """Search items by name."""
    async with async_session() as session:
        result = await session.execute(
            select(Item)
            .where(Item.name.ilike(f"%{q}%"))
            .limit(limit)
        )
        items = result.scalars().all()
        return [ItemSchema.model_validate(i) for i in items]


# ════════════════════════════════════════
# Manual Actions
# ════════════════════════════════════════

@router.post("/scan")
async def trigger_scan():
    """Manually trigger an AH scan."""
    snapshot = await ah_scanner.scan()
    if snapshot:
        return {
            "message": "Scan completed",
            "total_auctions": snapshot["total_auctions"],
            "scanned_at": snapshot["scanned_at"].isoformat(),
        }
    raise HTTPException(status_code=500, detail="Scan failed")


@router.post("/analyze")
async def trigger_analysis():
    """Manually trigger trading analysis."""
    deals = await trading_engine.analyze()
    return {
        "message": f"Analysis complete: {len(deals)} deals found",
        "deals_count": len(deals),
    }


@router.post("/refresh")
async def refresh_all():
    """Trigger a full scan + analysis cycle and return fresh deals."""
    import time
    t0 = time.monotonic()

    snapshot = await ah_scanner.scan()
    if not snapshot:
        raise HTTPException(status_code=500, detail="Scan failed")

    deals = await trading_engine.analyze()
    duration_s = time.monotonic() - t0

    # Notify Discord
    new_deals = [d for d in deals if getattr(d, "notified", False) is False]
    try:
        await discord_bot.send_scan_report(
            auctions_count=snapshot["total_auctions"],
            items_count=snapshot.get("unique_items", 0),
            new_deals_count=len(new_deals),
            duration_s=duration_s,
        )
        if new_deals:
            await discord_bot.send_deals_summary(new_deals)
    except Exception as exc:
        logger.warning(f"Discord notification failed: {exc}")

    return {
        "message": "Refresh complete",
        "total_auctions": snapshot["total_auctions"],
        "unique_items": snapshot.get("unique_items", 0),
        "deals_count": len(deals),
        "scanned_at": snapshot["scanned_at"].isoformat(),
    }
