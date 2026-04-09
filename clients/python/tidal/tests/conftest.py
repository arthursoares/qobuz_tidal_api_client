"""Shared fixtures and sample payloads for Tidal SDK tests."""

import pytest
from unittest.mock import AsyncMock

# ---------------------------------------------------------------------------
# Sample payloads — shaped like real Tidal v1 responses (trimmed for tests).
# ---------------------------------------------------------------------------

SAMPLE_ALBUM = {
    "id": 12345,
    "title": "Test Album",
    "cover": "abc12345-6789-abcd-ef01-23456789abcd",
    "releaseDate": "2024-01-15",
    "duration": 3600,
    "numberOfTracks": 12,
    "numberOfVolumes": 1,
    "explicit": False,
    "audioQuality": "HI_RES",
    "upc": "0123456789012",
    "copyright": "(C) 2024 Test Label",
    "url": "https://tidal.com/browse/album/12345",
    "artist": {"id": 100, "name": "Test Artist", "type": "MAIN"},
    "artists": [
        {"id": 100, "name": "Test Artist", "type": "MAIN"},
        {"id": 101, "name": "Featured Artist", "type": "FEATURED"},
    ],
}

SAMPLE_TRACK = {
    "id": 67890,
    "title": "Test Track",
    "duration": 240,
    "trackNumber": 3,
    "volumeNumber": 1,
    "explicit": True,
    "isrc": "USABC2412345",
    "audioQuality": "HI_RES",
    "copyright": "(C) 2024 Test Label",
    "url": "https://tidal.com/browse/track/67890",
    "artist": {"id": 100, "name": "Test Artist", "type": "MAIN"},
    "artists": [{"id": 100, "name": "Test Artist", "type": "MAIN"}],
    "album": {"id": 12345, "title": "Test Album", "cover": "abc12345"},
}

SAMPLE_PLAYLIST = {
    "uuid": "1234abcd-5678-efgh-ijkl-mnopqrstuvwx",
    "title": "Test Playlist",
    "description": "A test playlist",
    "numberOfTracks": 25,
    "duration": 5400,
    "publicPlaylist": True,
    "image": "img-uuid",
    "squareImage": "sq-uuid",
    "creator": {"id": 999, "name": "test-user"},
    "created": "2024-01-01T00:00:00.000+0000",
    "lastUpdated": "2024-02-01T00:00:00.000+0000",
    "url": "https://tidal.com/browse/playlist/1234abcd",
}

SAMPLE_FAVORITES_PAGE = {
    "limit": 100,
    "offset": 0,
    "totalNumberOfItems": 2,
    "items": [
        {"created": "2024-01-01T00:00:00.000+0000", "item": SAMPLE_ALBUM},
        {
            "created": "2024-01-02T00:00:00.000+0000",
            "item": {**SAMPLE_ALBUM, "id": 12346, "title": "Another Album"},
        },
    ],
}

SAMPLE_SEARCH_RESPONSE = {
    "limit": 50,
    "offset": 0,
    "totalNumberOfItems": 1,
    "items": [SAMPLE_ALBUM],
}

SAMPLE_ALBUM_ITEMS_PAGE = {
    "limit": 100,
    "offset": 0,
    "totalNumberOfItems": 2,
    "items": [
        {"item": SAMPLE_TRACK, "type": "track"},
        {
            "item": {**SAMPLE_TRACK, "id": 67891, "trackNumber": 4, "title": "Track Two"},
            "type": "track",
        },
    ],
}


@pytest.fixture
def mock_transport() -> AsyncMock:
    """Mock HttpTransport that catalog/favorites/streaming APIs accept.

    The methods all return ``(status, body)`` tuples; tests can override
    ``return_value`` per call.
    """
    transport = AsyncMock()
    transport.access_token = "test-access-token"
    return transport
