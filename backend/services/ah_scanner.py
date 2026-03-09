"""
Auction House Scanner - Fetches, processes and stores AH data.
"""

import logging
import numpy as np
from datetime import datetime
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from backend.core.database import async_session
from backend.models.models import Item, AuctionSnapshot, AuctionEntry, PriceHistory
from backend.services.blizzard_api import blizzard_client

logger = logging.getLogger(__name__)


class AHScanner:
    """Scans the Auction House and stores data."""

    async def scan(self) -> dict | None:
        """
        Perform a full AH scan:
        1. Fetch auctions from Blizzard API
        2. Store raw auction entries
        3. Compute price aggregates per item
        4. Update item cache for unknown items
        """
        logger.info("🔍 Starting AH scan...")

        # Fetch auctions
        auctions = await blizzard_client.get_auctions()
        if not auctions:
            logger.warning("No auctions retrieved, skipping scan")
            return None

        async with async_session() as session:
            # Create snapshot
            snapshot = AuctionSnapshot(
                scanned_at=datetime.utcnow(),
                total_auctions=len(auctions),
            )
            session.add(snapshot)
            await session.flush()  # Get snapshot.id

            # Process auctions
            item_auctions: dict[int, list[dict]] = {}
            total_volume = 0

            for auction in auctions:
                item_id = auction.get("item", {}).get("id")
                if not item_id:
                    continue

                # Determine price (unit_price for commodities, buyout for regular)
                unit_price = auction.get("unit_price", 0)
                buyout = auction.get("buyout", 0)
                bid = auction.get("bid", 0)
                quantity = auction.get("quantity", 1)
                price = unit_price or buyout

                if price <= 0:
                    continue

                # Store entry
                entry = AuctionEntry(
                    snapshot_id=snapshot.id,
                    auction_id=auction.get("id", 0),
                    item_id=item_id,
                    quantity=quantity,
                    unit_price=unit_price if unit_price else None,
                    buyout=buyout if buyout else None,
                    bid=bid if bid else None,
                    time_left=auction.get("time_left"),
                )
                session.add(entry)

                # Group for price aggregation
                if item_id not in item_auctions:
                    item_auctions[item_id] = []
                item_auctions[item_id].append({
                    "price": price,
                    "quantity": quantity,
                })
                total_volume += price * quantity

            snapshot.total_gold_volume = total_volume

            # Compute price history aggregates
            now = datetime.utcnow()
            for item_id, entries in item_auctions.items():
                prices = []
                total_qty = 0
                for e in entries:
                    # Weight each price by quantity for proper median/avg
                    prices.extend([e["price"]] * e["quantity"])
                    total_qty += e["quantity"]

                if not prices:
                    continue

                prices_arr = np.array(prices)

                price_history = PriceHistory(
                    item_id=item_id,
                    scanned_at=now,
                    min_buyout=int(np.min(prices_arr)),
                    avg_buyout=int(np.mean(prices_arr)),
                    median_buyout=int(np.median(prices_arr)),
                    max_buyout=int(np.max(prices_arr)),
                    total_quantity=total_qty,
                    num_auctions=len(entries),
                )
                session.add(price_history)

            await session.commit()
            logger.info(
                f"✅ Scan complete: {len(auctions)} auctions, "
                f"{len(item_auctions)} unique items"
            )

            # Capture values before session closes
            snapshot_data = {
                "id": snapshot.id,
                "total_auctions": snapshot.total_auctions,
                "total_gold_volume": snapshot.total_gold_volume,
                "scanned_at": snapshot.scanned_at,
                "unique_items": len(item_auctions),
            }

            # Fetch item details for new items (async, best effort)
            await self._update_unknown_items(session, list(item_auctions.keys()))

            return snapshot_data

    async def _update_unknown_items(self, session: AsyncSession, item_ids: list[int]):
        """Fetch and cache item details for items we haven't seen before."""
        # Find items not yet in our DB
        existing = await session.execute(
            select(Item.id).where(Item.id.in_(item_ids))
        )
        existing_ids = {row[0] for row in existing.fetchall()}
        new_ids = [iid for iid in item_ids if iid not in existing_ids]

        if not new_ids:
            return

        # Limit to avoid hammering API (fetch top 50 new items per scan)
        new_ids = new_ids[:50]
        logger.info(f"📦 Fetching details for {len(new_ids)} new items...")

        for item_id in new_ids:
            try:
                details = await blizzard_client.get_item_with_details(item_id)
                if details:
                    item = Item(
                        id=details["id"],
                        name=details["name"],
                        quality=details["quality"],
                        item_class=details["item_class"],
                        item_subclass=details["item_subclass"],
                        level=details["level"],
                        icon_url=details["icon_url"],
                        vendor_price=details["vendor_price"],
                    )
                    session.add(item)
            except Exception as e:
                logger.debug(f"Could not fetch item {item_id}: {e}")

        try:
            await session.commit()
        except Exception:
            await session.rollback()


# Singleton
ah_scanner = AHScanner()
