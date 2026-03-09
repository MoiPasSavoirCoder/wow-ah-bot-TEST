"""
WoW AH Trading Bot - Main entry point.
Starts FastAPI server, scheduler, and Discord bot.
"""

import asyncio
import logging
import sys
import os

# Add project root to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager

from backend.core.config import get_settings
from backend.core.database import init_db
from backend.api.routes import router
from backend.scheduler.jobs import start_scheduler, stop_scheduler
from backend.bot.discord_bot import discord_bot

# ─── Logging ───
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-7s | %(name)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("wow-ah-bot")

settings = get_settings()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifecycle: startup and shutdown."""
    logger.info("🚀 WoW AH Trading Bot starting...")

    # Init database
    await init_db()
    logger.info("✅ Database initialized")

    # Start scheduler
    start_scheduler()

    # Start Discord bot in background
    discord_task = None
    if settings.discord_bot_token:
        discord_task = asyncio.create_task(discord_bot.start())
        logger.info("🤖 Discord bot starting in background...")
    else:
        logger.warning("⚠️ Discord bot token not configured, bot disabled")

    logger.info(f"🌐 API available at http://localhost:{settings.backend_port}/docs")
    logger.info(f"🎮 Realm: {settings.blizzard_realm_slug} ({settings.blizzard_region.upper()})")

    yield

    # Shutdown
    logger.info("Shutting down...")
    stop_scheduler()
    if discord_task:
        await discord_bot.stop()
        discord_task.cancel()


# ─── FastAPI App ───
app = FastAPI(
    title="WoW AH Trading Bot",
    description="Bot intelligent de trading pour l'Hôtel des Ventes de World of Warcraft",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS for Angular frontend
app.add_middleware(
    CORSMiddleware,
    allow_origins=[settings.frontend_url, "http://localhost:4200"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include API routes
app.include_router(router, prefix="/api")


@app.get("/")
async def root():
    return {
        "name": "WoW AH Trading Bot",
        "version": "1.0.0",
        "realm": f"{settings.blizzard_realm_slug} ({settings.blizzard_region.upper()})",
        "docs": "/docs",
    }


if __name__ == "__main__":
    uvicorn.run(
        "backend.main:app",
        host=settings.backend_host,
        port=settings.backend_port,
        reload=True,
    )
