"""Typed response models for the Tidal v1 API."""

from __future__ import annotations

from dataclasses import dataclass, field

# Quality tier → Tidal audioquality string.
#
# Tier 3 (``HI_RES``) is the *legacy* MQA-folded format: physically a
# 16/44.1 FLAC carrying MQA-encoded subbands. Tidal launched ``HI_RES_LOSSLESS``
# in mid-2024 as a separate, true 24-bit/up-to-192 kHz tier delivered via a
# DASH/MPD manifest with BTS encryption — its JSON-vs-XML manifest split is
# what makes adding it more involved than just appending an enum.
QUALITY_MAP: dict[int, str] = {
    0: "LOW",              # AAC ~96kbps
    1: "HIGH",             # AAC ~320kbps
    2: "LOSSLESS",         # FLAC 16-bit/44.1kHz (CD quality)
    3: "HI_RES",           # MQA-folded FLAC (legacy "HiRes" tier)
    4: "HI_RES_LOSSLESS",  # True 24-bit FLAC, up to 192kHz (BTS / DASH)
}


@dataclass
class ArtistSummary:
    id: int
    name: str
    type: str | None = None  # "MAIN", "FEATURED", etc.

    @classmethod
    def from_dict(cls, d: dict | None) -> ArtistSummary:
        if not d:
            return cls(id=0, name="Unknown")
        return cls(id=d.get("id", 0), name=d.get("name", ""), type=d.get("type"))


@dataclass
class AlbumSummary:
    id: int
    title: str
    cover: str | None = None
    release_date: str | None = None

    @classmethod
    def from_dict(cls, d: dict | None) -> AlbumSummary:
        if not d:
            return cls(id=0, title="")
        return cls(
            id=d.get("id", 0),
            title=d.get("title", ""),
            cover=d.get("cover"),
            release_date=d.get("releaseDate"),
        )


@dataclass
class Album:
    id: int
    title: str
    artist: ArtistSummary
    artists: list[ArtistSummary] = field(default_factory=list)
    cover: str | None = None
    release_date: str | None = None
    duration: int = 0
    number_of_tracks: int = 0
    number_of_volumes: int = 1
    explicit: bool = False
    audio_quality: str | None = None  # "LOSSLESS", "HI_RES", etc.
    upc: str | None = None
    copyright: str | None = None
    url: str | None = None
    tracks: list[dict] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: dict) -> Album:
        artists_raw = d.get("artists") or []
        return cls(
            id=d.get("id", 0),
            title=d.get("title", ""),
            artist=ArtistSummary.from_dict(d.get("artist")),
            artists=[ArtistSummary.from_dict(a) for a in artists_raw],
            cover=d.get("cover"),
            release_date=d.get("releaseDate"),
            duration=d.get("duration", 0),
            number_of_tracks=d.get("numberOfTracks", 0),
            number_of_volumes=d.get("numberOfVolumes", 1),
            explicit=d.get("explicit", False),
            audio_quality=d.get("audioQuality"),
            upc=d.get("upc"),
            copyright=d.get("copyright"),
            url=d.get("url"),
            tracks=d.get("tracks", []) if isinstance(d.get("tracks"), list) else [],
        )


@dataclass
class Track:
    id: int
    title: str
    artist: ArtistSummary
    artists: list[ArtistSummary] = field(default_factory=list)
    album: AlbumSummary | None = None
    track_number: int = 0
    volume_number: int = 1
    duration: int = 0
    explicit: bool = False
    isrc: str | None = None
    audio_quality: str | None = None
    copyright: str | None = None
    url: str | None = None

    @classmethod
    def from_dict(cls, d: dict) -> Track:
        artists_raw = d.get("artists") or []
        return cls(
            id=d.get("id", 0),
            title=d.get("title", ""),
            artist=ArtistSummary.from_dict(d.get("artist")),
            artists=[ArtistSummary.from_dict(a) for a in artists_raw],
            album=AlbumSummary.from_dict(d.get("album")) if d.get("album") else None,
            track_number=d.get("trackNumber", 0),
            volume_number=d.get("volumeNumber", 1),
            duration=d.get("duration", 0),
            explicit=d.get("explicit", False),
            isrc=d.get("isrc"),
            audio_quality=d.get("audioQuality"),
            copyright=d.get("copyright"),
            url=d.get("url"),
        )


@dataclass
class Playlist:
    uuid: str
    title: str
    description: str = ""
    number_of_tracks: int = 0
    duration: int = 0
    public: bool = False
    image: str | None = None
    square_image: str | None = None
    creator: dict = field(default_factory=dict)
    created: str | None = None
    last_updated: str | None = None
    url: str | None = None

    @classmethod
    def from_dict(cls, d: dict) -> Playlist:
        return cls(
            uuid=str(d.get("uuid", "")),
            title=d.get("title", ""),
            description=d.get("description", ""),
            number_of_tracks=d.get("numberOfTracks", 0),
            duration=d.get("duration", 0),
            public=d.get("publicPlaylist", False),
            image=d.get("image"),
            square_image=d.get("squareImage"),
            creator=d.get("creator", {}) or {},
            created=d.get("created"),
            last_updated=d.get("lastUpdated"),
            url=d.get("url"),
        )


@dataclass
class StreamManifest:
    """Decoded playback manifest from playbackinfopostpaywall.

    Two manifest shapes flow through here:

    - **BTS** (legacy MQA / device-code clients): a single ``url`` to a
      complete file, optionally AES-CTR encrypted.
    - **DASH** (PKCE / HiRes-Lossless clients): a list of segment URLs
      that have to be downloaded in order and concatenated into a single
      fragmented-MP4 file. ``url`` aliases ``urls[0]`` for back-compat
      with code that pre-dates DASH support.
    """

    track_id: int
    audio_quality: str  # "LOSSLESS", "HI_RES", etc. (uppercase)
    codec: str         # "flac", "mqa", "aac", ...
    url: str
    encryption_type: str = "NONE"
    encryption_key: str | None = None  # base64
    restrictions: list[dict] = field(default_factory=list)
    # New for DASH: full ordered list of segment URLs (init + N media).
    # For BTS manifests this stays empty and ``url`` is the only handle.
    urls: list[str] = field(default_factory=list)
    is_dash: bool = False

    @property
    def is_encrypted(self) -> bool:
        return self.encryption_type != "NONE" and self.encryption_key is not None

    @property
    def file_extension(self) -> str:
        codec = self.codec.lower()
        if codec in ("flac", "mqa"):
            # DASH-delivered FLAC arrives wrapped in a fragmented MP4 container
            # (mp4-with-FLAC). Most music players handle that under .flac, but
            # strict FLAC parsers won't — see the optional ffmpeg remux step.
            return "flac"
        return "m4a"


@dataclass
class PaginatedResult:
    """Wraps a paginated Tidal API response.

    Tidal v1 favorites/search/playlists return ``{items, totalNumberOfItems, limit, offset}``.
    """

    items: list[dict]
    total: int
    limit: int
    offset: int

    @property
    def has_more(self) -> bool:
        return self.offset + len(self.items) < self.total

    @classmethod
    def from_dict(cls, d: dict) -> PaginatedResult:
        return cls(
            items=d.get("items", []),
            total=d.get("totalNumberOfItems", 0),
            limit=d.get("limit", 0),
            offset=d.get("offset", 0),
        )
