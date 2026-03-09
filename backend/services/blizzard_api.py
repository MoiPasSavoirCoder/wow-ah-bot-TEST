"""
Blizzard API Client - Fetches auction house data and item information.
"""

import httpx
import logging
from typing import Any

from backend.core.auth import blizzard_auth
from backend.core.config import get_settings

logger = logging.getLogger(__name__)
settings = get_settings()


class BlizzardAPIClient:
    """Client for Blizzard Game Data APIs."""

    def __init__(self):
        self.timeout = httpx.Timeout(30.0, connect=10.0)

    async def _get(self, url: str, namespace: str = "dynamic") -> dict | None:
        """Make an authenticated GET request to Blizzard API."""
        if namespace == "static":
            headers = await blizzard_auth.get_static_auth_headers()
        else:
            headers = await blizzard_auth.get_auth_headers()

        params = {"locale": settings.blizzard_locale}

        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                response = await client.get(url, headers=headers, params=params)
                response.raise_for_status()
                return response.json()
        except httpx.HTTPStatusError as e:
            logger.error(f"Blizzard API HTTP error: {e.response.status_code} - {url}")
            return None
        except httpx.RequestError as e:
            logger.error(f"Blizzard API request error: {e}")
            return None

    # ─── Auction House ───

    async def get_auctions(self) -> list[dict]:
        """
        Fetch all auctions for the connected realm (Archimonde).
        Returns list of auction entries.
        """
        url = settings.blizzard_ah_url
        logger.info(f"Fetching auctions from {url}")

        data = await self._get(url)
        if data and "auctions" in data:
            auctions = data["auctions"]
            logger.info(f"Retrieved {len(auctions)} auctions")
            return auctions
        return []

    async def get_commodities(self) -> list[dict]:
        """
        Fetch all commodity auctions (region-wide).
        """
        url = settings.blizzard_commodities_url
        logger.info(f"Fetching commodities from {url}")

        data = await self._get(url)
        if data and "auctions" in data:
            commodities = data["auctions"]
            logger.info(f"Retrieved {len(commodities)} commodity listings")
            return commodities
        return []

    # ─── Items ───

    async def get_item(self, item_id: int) -> dict | None:
        """Fetch item details by ID."""
        url = f"{settings.blizzard_api_base_url}/data/wow/item/{item_id}"
        return await self._get(url, namespace="static")

    async def get_item_media(self, item_id: int) -> str | None:
        """Fetch item icon URL."""
        url = f"{settings.blizzard_api_base_url}/data/wow/media/item/{item_id}"
        data = await self._get(url, namespace="static")
        if data and "assets" in data:
            for asset in data["assets"]:
                if asset.get("key") == "icon":
                    return asset.get("value")
        return None

    async def get_item_with_details(self, item_id: int) -> dict | None:
        """Fetch item with name and media."""
        item_data = await self.get_item(item_id)
        if not item_data:
            return None

        icon_url = await self.get_item_media(item_id)

        return {
            "id": item_id,
            "name": item_data.get("name", f"Item #{item_id}"),
            "quality": item_data.get("quality", {}).get("type", "COMMON"),
            "item_class": item_data.get("item_class", {}).get("name", "Unknown"),
            "item_subclass": item_data.get("item_subclass", {}).get("name", "Unknown"),
            "level": item_data.get("level", 0),
            "icon_url": icon_url,
            "vendor_price": item_data.get("sell_price", 0),
        }

    # ─── Connected Realm ───

    async def get_connected_realm(self) -> dict | None:
        """Get connected realm info."""
        url = (
            f"{settings.blizzard_api_base_url}/data/wow/connected-realm/"
            f"{settings.blizzard_connected_realm_id}"
        )
        return await self._get(url)

    async def search_connected_realms(self) -> list[dict]:
        """Search for connected realms to find realm ID."""
        url = f"{settings.blizzard_api_base_url}/data/wow/search/connected-realm"
        data = await self._get(url)
        if data and "results" in data:
            return data["results"]
        return []


# Singleton
blizzard_client = BlizzardAPIClient()
