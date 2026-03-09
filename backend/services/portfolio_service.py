"""
Portfolio Service - Tracks buys, sells, and P&L.
"""

import logging
from datetime import datetime
from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession

from backend.core.database import async_session
from backend.models.models import Portfolio, GoldBalance, Deal

logger = logging.getLogger(__name__)


class PortfolioService:
    """Manages the user's portfolio of AH investments."""

    async def add_transaction(
        self,
        item_id: int,
        item_name: str | None,
        action: str,
        quantity: int,
        price_per_unit: int,
        notes: str | None = None,
    ) -> Portfolio:
        """Record a buy or sell transaction."""
        async with async_session() as session:
            entry = Portfolio(
                item_id=item_id,
                item_name=item_name,
                action=action.upper(),
                quantity=quantity,
                price_per_unit=price_per_unit,
                total_price=price_per_unit * quantity,
                created_at=datetime.utcnow(),
                notes=notes,
            )
            session.add(entry)
            await session.commit()
            await session.refresh(entry)

            # Update gold balance
            await self._update_balance(session)

            logger.info(
                f"📝 Portfolio: {action} {quantity}x {item_name or item_id} "
                f"@ {price_per_unit / 10000:.1f}g each"
            )
            return entry

    async def get_transactions(self, limit: int = 100) -> list[Portfolio]:
        """Get recent transactions."""
        async with async_session() as session:
            result = await session.execute(
                select(Portfolio)
                .order_by(Portfolio.created_at.desc())
                .limit(limit)
            )
            return list(result.scalars().all())

    async def get_inventory(self) -> list[dict]:
        """
        Get current inventory (items bought but not yet sold).
        Groups buys and subtracts sells per item.
        """
        async with async_session() as session:
            # Sum buys per item
            buys = await session.execute(
                select(
                    Portfolio.item_id,
                    Portfolio.item_name,
                    func.sum(Portfolio.quantity).label("qty"),
                    func.avg(Portfolio.price_per_unit).label("avg_price"),
                    func.sum(Portfolio.total_price).label("total_invested"),
                )
                .where(Portfolio.action == "BUY")
                .group_by(Portfolio.item_id, Portfolio.item_name)
            )
            buy_map = {}
            for row in buys.fetchall():
                buy_map[row[0]] = {
                    "item_id": row[0],
                    "item_name": row[1],
                    "bought_qty": row[2] or 0,
                    "avg_buy_price": int(row[3] or 0),
                    "total_invested": row[4] or 0,
                }

            # Sum sells per item
            sells = await session.execute(
                select(
                    Portfolio.item_id,
                    func.sum(Portfolio.quantity).label("qty"),
                    func.sum(Portfolio.total_price).label("total_revenue"),
                )
                .where(Portfolio.action == "SELL")
                .group_by(Portfolio.item_id)
            )
            sell_map = {}
            for row in sells.fetchall():
                sell_map[row[0]] = {
                    "sold_qty": row[1] or 0,
                    "total_revenue": row[2] or 0,
                }

            # Compute inventory
            inventory = []
            for item_id, buy_data in buy_map.items():
                sell_data = sell_map.get(item_id, {"sold_qty": 0, "total_revenue": 0})
                remaining_qty = buy_data["bought_qty"] - sell_data["sold_qty"]

                if remaining_qty > 0:
                    inventory.append({
                        "item_id": item_id,
                        "item_name": buy_data["item_name"],
                        "quantity": remaining_qty,
                        "avg_buy_price": buy_data["avg_buy_price"],
                        "total_invested": buy_data["total_invested"],
                        "total_revenue": sell_data["total_revenue"],
                    })

            return inventory

    async def get_pnl_summary(self) -> dict:
        """Get Profit & Loss summary."""
        async with async_session() as session:
            # Total buys
            buy_result = await session.execute(
                select(func.sum(Portfolio.total_price)).where(Portfolio.action == "BUY")
            )
            total_bought = buy_result.scalar() or 0

            # Total sells
            sell_result = await session.execute(
                select(func.sum(Portfolio.total_price)).where(Portfolio.action == "SELL")
            )
            total_sold = sell_result.scalar() or 0

            # AH cut (5%)
            ah_fees = int(total_sold * 0.05)
            realized_profit = total_sold - ah_fees - total_bought

            return {
                "total_invested_copper": total_bought,
                "total_revenue_copper": total_sold,
                "ah_fees_copper": ah_fees,
                "realized_profit_copper": realized_profit,
            }

    async def _update_balance(self, session: AsyncSession):
        """Record current gold balance snapshot."""
        pnl = await self.get_pnl_summary()

        # Get total currently invested (in inventory)
        inventory = await self.get_inventory()
        invested = sum(
            item["avg_buy_price"] * item["quantity"] for item in inventory
        )

        balance = GoldBalance(
            recorded_at=datetime.utcnow(),
            balance_copper=pnl["realized_profit_copper"],
            invested_copper=invested,
            profit_copper=pnl["realized_profit_copper"],
        )

        async with async_session() as new_session:
            new_session.add(balance)
            await new_session.commit()

    async def get_gold_history(self, days: int = 30) -> list[GoldBalance]:
        """Get gold balance history for charts."""
        from datetime import timedelta
        cutoff = datetime.utcnow() - timedelta(days=days)

        async with async_session() as session:
            result = await session.execute(
                select(GoldBalance)
                .where(GoldBalance.recorded_at >= cutoff)
                .order_by(GoldBalance.recorded_at.asc())
            )
            return list(result.scalars().all())

    async def mark_deal_executed(self, deal_id: int):
        """Mark a deal as executed."""
        async with async_session() as session:
            result = await session.execute(
                select(Deal).where(Deal.id == deal_id)
            )
            deal = result.scalar_one_or_none()
            if deal:
                deal.status = "EXECUTED"
                await session.commit()


# Singleton
portfolio_service = PortfolioService()
