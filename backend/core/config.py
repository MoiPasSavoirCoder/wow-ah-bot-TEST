"""
Configuration module - loads settings from .env
"""

from pydantic_settings import BaseSettings
from pydantic import Field
from functools import lru_cache
import os


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    # Blizzard API
    blizzard_client_id: str = Field(default="", description="Blizzard API client ID")
    blizzard_client_secret: str = Field(default="", description="Blizzard API client secret")
    blizzard_region: str = Field(default="eu", description="Blizzard API region")
    blizzard_realm_slug: str = Field(default="archimonde", description="Realm slug")
    blizzard_connected_realm_id: int = Field(default=1084, description="Connected realm ID")
    blizzard_locale: str = Field(default="fr_FR", description="Locale for item names")

    # Discord
    discord_bot_token: str = Field(default="", description="Discord bot token")
    discord_channel_id: str = Field(default="", description="Discord channel ID for alerts")

    # Database
    database_url: str = Field(default="sqlite:///./data/wow_ah.db", description="Database URL")

    # Server
    backend_host: str = Field(default="0.0.0.0")
    backend_port: int = Field(default=8000)
    frontend_url: str = Field(default="http://localhost:4200")

    # Trading
    min_profit_margin: float = Field(default=5, description="Min profit margin % to flag a deal")
    max_tracked_items: int = Field(default=5000)
    max_budget_gold: int = Field(default=500000, description="Max budget in gold")
    scan_interval_minutes: int = Field(default=5, description="AH scan interval")

    @property
    def blizzard_api_base_url(self) -> str:
        return f"https://{self.blizzard_region}.api.blizzard.com"

    @property
    def blizzard_token_url(self) -> str:
        return "https://oauth.battle.net/token"

    @property
    def blizzard_ah_url(self) -> str:
        return (
            f"{self.blizzard_api_base_url}/data/wow/connected-realm/"
            f"{self.blizzard_connected_realm_id}/auctions"
        )

    @property
    def blizzard_commodities_url(self) -> str:
        return f"{self.blizzard_api_base_url}/data/wow/auctions/commodities"

    model_config = {
        "env_file": os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(__file__))), ".env"),
        "env_file_encoding": "utf-8",
        "extra": "ignore",
    }


@lru_cache()
def get_settings() -> Settings:
    return Settings()
