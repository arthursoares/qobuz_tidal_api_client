"""Async Python client for the Tidal API (v1, with downloads).

This SDK targets Tidal's legacy v1 API (``api.tidalhifi.com/v1``) — the
only API that exposes ``playbackinfopostpaywall`` for downloads. The Go
Tidal client in this same repo (``clients/go/tidal/``) targets the new
public OpenAPI v2 (``openapi.tidal.com/v2``), which is metadata-only.
The two SDKs serve different purposes and aren't interchangeable.
"""

from .auth import (
    CREDENTIALS_FILE,
    load_credentials,
    poll_device_code,
    refresh_access_token,
    request_device_code,
    save_credentials,
)
from .catalog import CatalogAPI
from .client import TidalClient
from .downloader import (
    AlbumDownloader,
    AlbumResult,
    DownloadConfig,
    TrackResult,
)
from .errors import (
    AuthenticationError,
    ForbiddenError,
    NonStreamableError,
    NotFoundError,
    RateLimitError,
    TidalError,
)
from .favorites import FavoritesAPI
from .streaming import StreamingAPI
from .types import (
    Album,
    AlbumSummary,
    ArtistSummary,
    PaginatedResult,
    Playlist,
    QUALITY_MAP,
    StreamManifest,
    Track,
)

__all__ = [
    # Client + facades
    "TidalClient",
    "CatalogAPI",
    "FavoritesAPI",
    "StreamingAPI",
    # Downloader
    "AlbumDownloader",
    "AlbumResult",
    "DownloadConfig",
    "TrackResult",
    # Auth
    "CREDENTIALS_FILE",
    "load_credentials",
    "save_credentials",
    "request_device_code",
    "poll_device_code",
    "refresh_access_token",
    # Errors
    "TidalError",
    "AuthenticationError",
    "ForbiddenError",
    "NonStreamableError",
    "NotFoundError",
    "RateLimitError",
    # Types
    "Album",
    "AlbumSummary",
    "ArtistSummary",
    "PaginatedResult",
    "Playlist",
    "QUALITY_MAP",
    "StreamManifest",
    "Track",
]
