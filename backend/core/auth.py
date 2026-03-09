"""
Blizzard OAuth2 Authentication.
"""

import httpx
import time
from backend.core.config import get_settings


class BlizzardAuth:
    """Handles OAuth2 client_credentials flow for Blizzard API."""

    def __init__(self):
        self.settings = get_settings()
        self._access_token: str | None = None
        self._token_expires_at: float = 0

    @property
    def is_token_valid(self) -> bool:
        return self._access_token is not None and time.time() < self._token_expires_at

    async def get_access_token(self) -> str:
        """Get a valid access token, refreshing if necessary."""
        if self.is_token_valid:
            return self._access_token

        async with httpx.AsyncClient() as client:
            response = await client.post(
                self.settings.blizzard_token_url,
                data={"grant_type": "client_credentials"},
                auth=(self.settings.blizzard_client_id, self.settings.blizzard_client_secret),
            )
            response.raise_for_status()
            data = response.json()

        self._access_token = data["access_token"]
        # Expire 60 seconds early to be safe
        self._token_expires_at = time.time() + data.get("expires_in", 86400) - 60

        return self._access_token

    async def get_auth_headers(self) -> dict:
        """Get headers with Bearer token."""
        token = await self.get_access_token()
        return {
            "Authorization": f"Bearer {token}",
            "Battlenet-Namespace": f"dynamic-{self.settings.blizzard_region}",
        }

    async def get_static_auth_headers(self) -> dict:
        """Get headers for static data (items, etc.)."""
        token = await self.get_access_token()
        return {
            "Authorization": f"Bearer {token}",
            "Battlenet-Namespace": f"static-{self.settings.blizzard_region}",
        }


# Singleton
blizzard_auth = BlizzardAuth()
