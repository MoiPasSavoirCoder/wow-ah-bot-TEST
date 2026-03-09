"""
Discord Bot - Sends alerts for buy/sell opportunities.
"""

import discord
import logging
import asyncio
from datetime import datetime

from backend.core.config import get_settings
from backend.models.models import Deal
from backend.models.schemas import copper_to_gold_str

logger = logging.getLogger(__name__)
settings = get_settings()


class DiscordAlertBot:
    """Discord bot that sends trading alerts."""

    def __init__(self):
        self.client: discord.Client | None = None
        self.channel: discord.TextChannel | None = None
        self._ready = asyncio.Event()

    async def start(self):
        """Start the Discord bot in the background."""
        if not settings.discord_bot_token:
            logger.warning("⚠️ Discord bot token not set, bot disabled")
            return

        intents = discord.Intents.default()
        intents.message_content = True
        self.client = discord.Client(intents=intents)

        @self.client.event
        async def on_ready():
            logger.info(f"🤖 Discord bot connected as {self.client.user}")
            if settings.discord_channel_id:
                try:
                    self.channel = self.client.get_channel(int(settings.discord_channel_id))
                    if not self.channel:
                        self.channel = await self.client.fetch_channel(int(settings.discord_channel_id))
                    logger.info(f"📢 Alert channel: #{self.channel.name}")
                except Exception as e:
                    logger.error(f"Could not find Discord channel: {e}")
            self._ready.set()

        try:
            await self.client.start(settings.discord_bot_token)
        except Exception as e:
            logger.error(f"Discord bot failed to start: {e}")

    async def wait_ready(self, timeout: float = 30.0):
        """Wait for bot to be ready."""
        try:
            await asyncio.wait_for(self._ready.wait(), timeout=timeout)
        except asyncio.TimeoutError:
            logger.warning("Discord bot took too long to connect")

    async def _get_channel(self) -> discord.TextChannel | None:
        """Return the alert channel, waiting for readiness if needed."""
        if not self.client:
            logger.warning("Discord client not started")
            return None
        await self.wait_ready(timeout=10.0)
        if not self.channel:
            logger.warning("Discord channel not available")
            return None
        return self.channel

    async def send_deal_alert(self, deal: Deal):
        """Send a deal alert to the Discord channel."""
        if not self.channel:
            logger.debug("No Discord channel, skipping alert")
            return

        embed = self._build_deal_embed(deal)

        try:
            await self.channel.send(embed=embed)
            logger.info(f"📨 Discord alert sent for {deal.item_name}")
        except Exception as e:
            logger.error(f"Failed to send Discord alert: {e}")

    async def send_deals_summary(self, deals: list[Deal]):
        """Send a summary of multiple deals."""
        if not self.channel or not deals:
            return

        # Header embed
        header = discord.Embed(
            title="🏦 WoW AH Trading Bot - Nouvelles Opportunités !",
            description=f"**{len(deals)}** deal(s) détecté(s) à {datetime.utcnow().strftime('%H:%M UTC')}",
            color=discord.Color.gold(),
        )
        header.set_footer(text="Serveur Archimonde (EU) • Données mises à jour toutes les heures")

        try:
            await self.channel.send(embed=header)
        except Exception as e:
            logger.error(f"Failed to send header: {e}")
            return

        # Individual deals (max 10 to avoid spam)
        for deal in deals[:10]:
            embed = self._build_deal_embed(deal)
            try:
                await self.channel.send(embed=embed)
                await asyncio.sleep(0.5)  # Rate limit respect
            except Exception as e:
                logger.error(f"Failed to send deal alert: {e}")

        if len(deals) > 10:
            try:
                await self.channel.send(
                    f"... et **{len(deals) - 10}** autres deals. "
                    f"Consultez le dashboard pour la liste complète ! 📊"
                )
            except Exception:
                pass

    def _build_deal_embed(self, deal: Deal) -> discord.Embed:
        """Build a rich embed for a deal."""
        if deal.deal_type == "BUY":
            color = discord.Color.green()
            emoji = "🟢"
            action = "ACHETER"
        else:
            color = discord.Color.red()
            emoji = "🔴"
            action = "VENDRE"

        embed = discord.Embed(
            title=f"{emoji} {action} : {deal.item_name or f'Item #{deal.item_id}'}",
            color=color,
            timestamp=deal.detected_at or datetime.utcnow(),
        )

        # Price info
        embed.add_field(
            name="💰 Prix actuel",
            value=copper_to_gold_str(deal.current_price),
            inline=True,
        )
        embed.add_field(
            name="📊 Prix moyen (7j)",
            value=copper_to_gold_str(deal.avg_price),
            inline=True,
        )
        embed.add_field(
            name="📈 Marge estimée",
            value=f"**{deal.profit_margin:.1f}%**",
            inline=True,
        )

        # Action details
        if deal.deal_type == "BUY":
            embed.add_field(
                name="🛒 Action recommandée",
                value=(
                    f"Acheter **{deal.suggested_quantity}x** "
                    f"à ≤ {copper_to_gold_str(deal.suggested_buy_price)}\n"
                    f"Revendre à ~ {copper_to_gold_str(deal.suggested_sell_price)}"
                ),
                inline=False,
            )
            total_cost = (deal.suggested_buy_price or 0) * deal.suggested_quantity
            potential_profit = (
                ((deal.suggested_sell_price or 0) - (deal.suggested_buy_price or 0))
                * deal.suggested_quantity
                * 0.95  # AH cut
            )
            embed.add_field(
                name="💵 Investissement",
                value=copper_to_gold_str(int(total_cost)),
                inline=True,
            )
            embed.add_field(
                name="💎 Profit potentiel",
                value=copper_to_gold_str(int(potential_profit)),
                inline=True,
            )
        else:
            embed.add_field(
                name="📤 Action recommandée",
                value=f"Vendre à ~ {copper_to_gold_str(deal.suggested_sell_price)}",
                inline=False,
            )

        # Confidence
        conf = deal.confidence_score
        if conf >= 75:
            conf_emoji = "🟢🟢🟢"
        elif conf >= 50:
            conf_emoji = "🟢🟢⚪"
        elif conf >= 30:
            conf_emoji = "🟢⚪⚪"
        else:
            conf_emoji = "⚪⚪⚪"

        embed.add_field(
            name="🎯 Confiance",
            value=f"{conf_emoji} **{conf:.0f}/100**",
            inline=True,
        )

        embed.set_footer(text=f"Item ID: {deal.item_id} • Deal #{deal.id or '?'}")

        return embed

    async def send_scan_report(
        self,
        auctions_count: int,
        items_count: int,
        new_deals_count: int,
        duration_s: float,
    ):
        """Send a scan summary embed after every scan."""
        channel = await self._get_channel()
        if not channel:
            return

        if new_deals_count > 0:
            color = discord.Color.green()
            title = f"✅ Scan terminé — {new_deals_count} opportunité{'s' if new_deals_count > 1 else ''} trouvée{'s' if new_deals_count > 1 else ''} !"
        else:
            color = discord.Color.blurple()
            title = "🔍 Scan terminé — Aucune nouvelle opportunité"

        embed = discord.Embed(
            title=title,
            color=color,
            timestamp=datetime.utcnow(),
        )

        embed.add_field(
            name="📦 Enchères analysées",
            value=f"**{auctions_count:,}**",
            inline=True,
        )
        embed.add_field(
            name="🧩 Items uniques",
            value=f"**{items_count:,}**",
            inline=True,
        )
        embed.add_field(
            name="💡 Nouvelles opportunités",
            value=f"**{new_deals_count}**",
            inline=True,
        )
        embed.add_field(
            name="⏱️ Durée du scan",
            value=f"**{duration_s:.1f}s**",
            inline=True,
        )

        next_scan_minutes = settings.scan_interval_minutes
        embed.add_field(
            name="⏰ Prochain scan",
            value=f"Dans **{next_scan_minutes} min**",
            inline=True,
        )

        embed.set_footer(text="WoW AH Bot • Archimonde EU")

        try:
            await channel.send(embed=embed)
        except Exception as e:
            logger.error(f"Failed to send scan report: {e}")

    async def stop(self):
        """Gracefully stop the bot."""
        if self.client:
            await self.client.close()


# Singleton
discord_bot = DiscordAlertBot()
