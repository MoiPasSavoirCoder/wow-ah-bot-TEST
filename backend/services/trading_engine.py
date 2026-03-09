"""
Trading Engine - Analyzes AH data and detects profitable deals.

Strategy:
1. Compare current min price to historical average/median
2. Items significantly below average → BUY signal
3. Items significantly above average → SELL signal
4. Confidence scoring based on volume, price stability, and margin

This is NOT a real-money trading algorithm. It works with in-game gold only.
"""

import logging
import numpy as np
from datetime import datetime, timedelta
from sqlalchemy import select, func, and_
from sqlalchemy.ext.asyncio import AsyncSession

from backend.core.config import get_settings
from backend.core.database import async_session
from backend.models.models import PriceHistory, Item, Deal

logger = logging.getLogger(__name__)
settings = get_settings()


class TradingEngine:
    """Analyzes market data and generates buy/sell signals."""

    # Minimum data points needed for reliable analysis
    MIN_HISTORY_POINTS = 5
    # Look back period for historical average
    LOOKBACK_DAYS = 7
    # Minimum daily volume to consider an item liquid
    MIN_DAILY_VOLUME = 10

    async def analyze(self) -> list[Deal]:
        """
        Run full market analysis and return detected deals.

        Algorithm:
        1. For each item with enough price history:
           - Compute rolling average & std dev over LOOKBACK_DAYS
           - Compare current price to historical average
           - If current_price < avg * (1 - margin_threshold) → BUY
           - If current_price > avg * (1 + margin_threshold) → SELL
        2. Score each deal by confidence (volume, volatility, margin)
        3. Sort by best risk/reward ratio
        """
        logger.info("🧠 Running trading analysis...")
        deals: list[Deal] = []

        async with async_session() as session:
            # Get items with recent price data
            cutoff = datetime.utcnow() - timedelta(days=self.LOOKBACK_DAYS)

            # Find items with enough history
            item_counts = await session.execute(
                select(
                    PriceHistory.item_id,
                    func.count(PriceHistory.id).label("cnt"),
                )
                .where(PriceHistory.scanned_at >= cutoff)
                .group_by(PriceHistory.item_id)
                .having(func.count(PriceHistory.id) >= self.MIN_HISTORY_POINTS)
            )

            candidate_items = [row[0] for row in item_counts.fetchall()]
            logger.info(f"📊 Analyzing {len(candidate_items)} items with sufficient data")

            for item_id in candidate_items[:settings.max_tracked_items]:
                deal = await self._analyze_item(session, item_id, cutoff)
                if deal:
                    deals.append(deal)

            # Sort by confidence * margin (best deals first)
            deals.sort(key=lambda d: d.confidence_score * d.profit_margin, reverse=True)

            # Save to database
            for deal in deals:
                session.add(deal)
            await session.commit()

            logger.info(f"💰 Found {len(deals)} potential deals")
            return deals

    async def _analyze_item(
        self, session: AsyncSession, item_id: int, cutoff: datetime
    ) -> Deal | None:
        """Analyze a single item and return a Deal if profitable."""

        # Fetch price history
        history = await session.execute(
            select(PriceHistory)
            .where(
                and_(
                    PriceHistory.item_id == item_id,
                    PriceHistory.scanned_at >= cutoff,
                )
            )
            .order_by(PriceHistory.scanned_at.asc())
        )
        records = history.scalars().all()

        if len(records) < self.MIN_HISTORY_POINTS:
            return None

        # Extract price arrays
        min_prices = np.array([r.min_buyout for r in records if r.min_buyout])
        avg_prices = np.array([r.avg_buyout for r in records if r.avg_buyout])
        median_prices = np.array([r.median_buyout for r in records if r.median_buyout])
        volumes = np.array([r.total_quantity for r in records])

        if len(min_prices) < 3 or len(avg_prices) < 3:
            return None

        # Current values (latest scan)
        current_min = int(min_prices[-1])
        current_volume = int(volumes[-1]) if len(volumes) > 0 else 0

        # Historical statistics
        hist_avg = float(np.mean(avg_prices[:-1]))  # Exclude latest
        hist_median = float(np.median(median_prices[:-1])) if len(median_prices) > 1 else hist_avg
        hist_std = float(np.std(avg_prices[:-1])) if len(avg_prices) > 1 else 0
        avg_volume = float(np.mean(volumes[:-1])) if len(volumes) > 1 else 0

        if hist_avg <= 0 or avg_volume < self.MIN_DAILY_VOLUME:
            return None

        # ─── Deal Detection ───

        # Price deviation from historical average
        price_deviation = (current_min - hist_avg) / hist_avg * 100  # In %

        # Calculate profit margin
        # For BUY: we expect price to revert to mean
        # AH cut is 5%, so real profit = (sell - buy) * 0.95
        ah_cut = 0.01

        deal_type = None
        profit_margin = 0.0
        suggested_buy = 0
        suggested_sell = 0

        if price_deviation < -settings.min_profit_margin:
            # Price is significantly BELOW average → BUY opportunity
            deal_type = "BUY"
            suggested_buy = current_min
            # Target sell at median (conservative)
            suggested_sell = int(hist_median)
            raw_margin = (suggested_sell - suggested_buy) / suggested_buy * 100
            profit_margin = raw_margin * (1 - ah_cut)

        elif price_deviation > settings.min_profit_margin * 1.5:
            # Price is significantly ABOVE average → SELL signal
            deal_type = "SELL"
            suggested_sell = current_min
            suggested_buy = int(hist_avg)
            raw_margin = (suggested_sell - suggested_buy) / suggested_buy * 100
            profit_margin = raw_margin * (1 - ah_cut)

        if not deal_type or profit_margin < settings.min_profit_margin:
            return None

        # ─── Confidence Scoring ───
        confidence = self._compute_confidence(
            profit_margin=profit_margin,
            price_deviation=abs(price_deviation),
            hist_std=hist_std,
            hist_avg=hist_avg,
            avg_volume=avg_volume,
            current_volume=current_volume,
            num_data_points=len(records),
        )

        if confidence < 30:
            return None

        # ─── Suggested Quantity ───
        # Buy quantity based on budget and liquidity
        if deal_type == "BUY":
            max_by_budget = settings.max_budget_gold * 10000 // max(suggested_buy, 1)
            # Don't buy more than 20% of daily volume
            max_by_volume = max(1, int(avg_volume * 0.2))
            suggested_qty = min(max_by_budget, max_by_volume, 200)
        else:
            suggested_qty = 0  # SELL signals are informational

        # Get item name
        item_result = await session.execute(
            select(Item.name).where(Item.id == item_id)
        )
        item_name = item_result.scalar() or f"Item #{item_id}"

        return Deal(
            item_id=item_id,
            item_name=item_name,
            detected_at=datetime.utcnow(),
            deal_type=deal_type,
            current_price=current_min,
            avg_price=int(hist_avg),
            suggested_buy_price=suggested_buy,
            suggested_sell_price=suggested_sell,
            suggested_quantity=max(1, int(suggested_qty)),
            profit_margin=round(profit_margin, 2),
            confidence_score=round(confidence, 1),
            status="PENDING",
            notified=False,
        )

    def _compute_confidence(
        self,
        profit_margin: float,
        price_deviation: float,
        hist_std: float,
        hist_avg: float,
        avg_volume: float,
        current_volume: int,
        num_data_points: int,
    ) -> float:
        """
        Compute a 0-100 confidence score for a deal.

        Factors:
        - Margin size (higher = better, but diminishing returns)
        - Price stability (low std = more predictable)
        - Volume / Liquidity (high volume = easier to sell)
        - Data quality (more data points = more reliable)
        """
        score = 0.0

        # Margin score (0-30 points)
        # Sweet spot is 20-80% margin
        margin_score = min(30, profit_margin * 0.5)
        score += margin_score

        # Stability score (0-25 points)
        # Coefficient of variation (lower = more stable)
        if hist_avg > 0:
            cv = hist_std / hist_avg
            if cv < 0.1:
                stability_score = 25
            elif cv < 0.3:
                stability_score = 20
            elif cv < 0.5:
                stability_score = 12
            else:
                stability_score = 5
        else:
            stability_score = 0
        score += stability_score

        # Liquidity score (0-25 points)
        if avg_volume >= 100:
            liquidity_score = 25
        elif avg_volume >= 50:
            liquidity_score = 20
        elif avg_volume >= 20:
            liquidity_score = 15
        elif avg_volume >= self.MIN_DAILY_VOLUME:
            liquidity_score = 10
        else:
            liquidity_score = 0
        score += liquidity_score

        # Data quality score (0-20 points)
        data_score = min(20, num_data_points * 2)
        score += data_score

        return min(100, score)


# Singleton
trading_engine = TradingEngine()
