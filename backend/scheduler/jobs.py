"""
Scheduler - Periodic tasks for AH scanning and analysis.
"""

import logging
import time
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.interval import IntervalTrigger

from backend.core.config import get_settings
from backend.services.ah_scanner import ah_scanner
from backend.services.trading_engine import trading_engine
from backend.bot.discord_bot import discord_bot
from backend.models.models import Deal
from backend.core.database import async_session
from sqlalchemy import select, and_

logger = logging.getLogger(__name__)
settings = get_settings()

scheduler = AsyncIOScheduler()


async def scan_and_analyze():
    """Main scheduled job: scan AH, analyze, and send alerts."""
    logger.info("⏰ Scheduled scan starting...")

    try:
        # Step 1: Scan AH
        scan_start = time.time()
        snapshot = await ah_scanner.scan()
        duration_s = time.time() - scan_start

        if not snapshot:
            logger.warning("Scan failed, skipping analysis")
            return

        auctions_count = snapshot.get("total_auctions", 0)
        items_count = snapshot.get("unique_items", 0)

        # Step 2: Run trading analysis
        deals = await trading_engine.analyze()
        unnotified_deals = [d for d in deals if not d.notified] if deals else []

        # Step 3: Always send a scan report to Discord
        await discord_bot.send_scan_report(
            auctions_count=auctions_count,
            items_count=items_count,
            new_deals_count=len(unnotified_deals),
            duration_s=duration_s,
        )

        # Step 4: Send deal alerts only if there are new deals
        if unnotified_deals:
            await discord_bot.send_deals_summary(unnotified_deals)

            # Mark as notified
            async with async_session() as session:
                for deal in unnotified_deals:
                    if deal.id:
                        result = await session.execute(
                            select(Deal).where(Deal.id == deal.id)
                        )
                        db_deal = result.scalar_one_or_none()
                        if db_deal:
                            db_deal.notified = True
                await session.commit()

        logger.info(
            f"✅ Scheduled scan complete: {auctions_count} auctions, "
            f"{items_count} items, {len(unnotified_deals)} new deals "
            f"in {duration_s:.1f}s"
        )

    except Exception as e:
        logger.error(f"❌ Scheduled scan failed: {e}", exc_info=True)


def start_scheduler():
    """Start the periodic scheduler."""
    scheduler.add_job(
        scan_and_analyze,
        trigger=IntervalTrigger(minutes=settings.scan_interval_minutes),
        id="ah_scan",
        name="AH Scan & Analysis",
        replace_existing=True,
    )
    scheduler.start()
    logger.info(
        f"📅 Scheduler started: scanning every {settings.scan_interval_minutes} minutes"
    )


def stop_scheduler():
    """Stop the scheduler."""
    scheduler.shutdown(wait=False)
    logger.info("Scheduler stopped")
