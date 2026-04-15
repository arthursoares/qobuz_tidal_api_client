# AI Assistant Instructions

## Project Overview

`qobuz_tidal_api_client` is a standalone, language-agnostic SDK for the Qobuz and Tidal streaming services. It was extracted from the [`streamrip`](https://github.com/nathom/streamrip) project (now rebuilt as [`arthursoares/libsync`](https://github.com/arthursoares/libsync)) to create a reusable library-management SDK.

- **Python** (`clients/python/`): async clients for both services with a matching facade shape (`client.catalog`, `client.favorites`, `client.playlists`, `client.streaming`) and `AlbumDownloader` implementations with MQA AES-CTR decryption for Tidal.
- **Go** (`clients/go/`): Qobuz full client + CLI binary (`cmd/qobuz`), Tidal v2 read-only client (metadata only — Tidal v2's public OpenAPI has no playback endpoint).
- **Docs** (`docs/api-spec.md`): API specification validated from real Proxyman captures of the official clients.

## Key facts

- **Both services go through reverse-engineered APIs** — Qobuz and Tidal don't publish public APIs for their music catalog. This is an *unofficial* client. See [Disclaimer](README.md#disclaimer) for terms.
- **Downloads are Python-only.** The Go Tidal client targets Tidal's public OpenAPI v2, which is metadata-only. The Python Tidal SDK targets the legacy v1 API (`api.tidalhifi.com/v1`), the only Tidal API that exposes `playbackinfopostpaywall` for MQA-encrypted streams.
- **Auth is per-service.** Qobuz uses OAuth (Helper app) or a pasted web-player token. Tidal uses device-code OAuth. The `qobuz.auth.APP_SECRET` is hardcoded (decoded from the Qobuz Helper bundle); for web-player tokens, the Python SDK's `spoofer.py` scrapes a working secret from `play.qobuz.com`.
- **Credentials live at `~/.config/qobuz/credentials.json`** (CLI default). The `app_secret` used for stream-URL signing is NOT stored there — it's resolved at runtime.

## Structure

```
clients/
├── python/
│   ├── qobuz/            # Async Qobuz package (pyproject.toml at ../)
│   ├── tests/            # Qobuz pytest suite
│   └── tidal/            # Async Tidal package (own pyproject.toml)
│       ├── tidal/        # source
│       └── tests/        # Tidal pytest suite
└── go/
    ├── cmd/qobuz/        # CLI binary
    └── tidal/            # Tidal v2 read-only package
docs/
└── api-spec.md           # Full API specification
```

## Development conventions

- **Python**: async-first (`aiohttp`), `asyncio_mode = "auto"` in pytest, typed dataclasses for API responses.
- **Tests**: mocked transport via `aioresponses` — no network calls in CI.
- **Go**: mirror the Python facade structure where possible, but don't block Go-side features on Python parity (and vice versa).

## Consumers

The canonical consumer is [`arthursoares/libsync`](https://github.com/arthursoares/libsync), which imports both Python packages as a git submodule at `sdks/qobuz_api_client/`. When changing the SDK, bump the submodule pin in Libsync to verify end-to-end.

## License

GPL-3.0-only (inherited from streamrip). See [LICENSE](LICENSE).
