"""Async Python client for the Qobuz API."""

from .auth import load_credentials, login, save_credentials
from .client import QobuzClient
from .errors import (
    AuthenticationError,
    ForbiddenError,
    InvalidAppError,
    NonStreamableError,
    NotFoundError,
    QobuzError,
    RateLimitError,
)
from .types import (
    Album,
    AlbumSummary,
    ArtistRole,
    ArtistSummary,
    AudioInfo,
    Award,
    FavoriteIds,
    FileUrl,
    Genre,
    ImageSet,
    Label,
    LastUpdate,
    PaginatedResult,
    Playlist,
    Rights,
    Session,
    Track,
    UserSummary,
)

__all__ = [
    "QobuzClient",
    "load_credentials", "login", "save_credentials",
    "QobuzError", "AuthenticationError", "ForbiddenError", "InvalidAppError",
    "NonStreamableError", "NotFoundError", "RateLimitError",
    "Album", "AlbumSummary", "ArtistRole", "ArtistSummary", "AudioInfo", "Award",
    "FavoriteIds", "FileUrl", "Genre", "ImageSet", "Label", "LastUpdate",
    "PaginatedResult", "Playlist", "Rights", "Session", "Track", "UserSummary",
]
