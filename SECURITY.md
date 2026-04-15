# Security Policy

## Supported versions

Only the latest tagged release on `main` is supported. This SDK is pre-1.0 and has no LTS branches.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Use one of these private channels:

1. **GitHub private security advisories** (preferred) — https://github.com/arthursoares/qobuz_tidal_api_client/security/advisories/new
2. **Email** — `github@arthursoares.com` with subject prefix `[qobuz_tidal_api_client security]`.

Include:
- Affected version (commit SHA or tag).
- Which client (`qobuz` / `tidal` / Python / Go).
- Reproduction steps or proof-of-concept.
- Impact assessment.
- Suggested fix, if you have one.

## What to expect

- **Acknowledgement**: within 7 days.
- **Triage and fix timeline**: within 14 days. Critical issues prioritized.
- **Disclosure**: coordinated with you, default 90 days from report or release of fix (whichever is sooner).
- **Credit**: in release notes unless you prefer anonymity.

## Scope

**In scope:**
- Credential-handling paths (OAuth flows, token storage in `~/.config/qobuz/credentials.json`, `app_secret` resolution via `spoofer.py`).
- Request-signing correctness (if signatures leak secret state or are reusable).
- HTTP transport (token exposure in logs, URL leakage).
- Downloader integrity (MQA AES-CTR decryption, file integrity).

**Out of scope** (upstream or user responsibility):
- Vulnerabilities in Qobuz's or Tidal's own services — report to those vendors.
- Using the SDK with credentials obtained in violation of Qobuz/Tidal ToS.
- The local filesystem where `~/.config/qobuz/credentials.json` is stored — protect your machine.

## Security-relevant context

- The SDK stores OAuth tokens in plaintext at `~/.config/qobuz/credentials.json`. Anyone with read access to that file can impersonate you against the streaming service. Protect your home directory accordingly.
- The Qobuz `APP_SECRET` hardcoded in `clients/python/qobuz/auth.py` was decoded from the Qobuz Helper application bundle; it's a public-knowledge constant used by Qobuz's own desktop app. Leaking it is no worse than the desktop client's behavior.
- The web-player-token code path (`clients/python/qobuz/spoofer.py`) scrapes `play.qobuz.com` to find a working signing secret. This is a reverse-engineered request and can break whenever Qobuz changes their bundle layout; report breakage as a regular bug.
