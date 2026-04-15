# Contributing to qobuz_tidal_api_client

Thanks for your interest. This SDK powers [Libsync](https://github.com/arthursoares/libsync) and any other consumer that needs Qobuz/Tidal API access — PRs that improve either Python or Go clients are welcome.

## Branch model

Simpler than a full Gitflow:

- **`main`** — stable. PRs land here directly. Tag a release when there's enough to ship.
- **`feature/*`** — your work. Branch off `main`, PR back.

No `dev` branch. No required-linear-history. Merge commits are fine.

## Local development

### Python clients

```bash
# Qobuz
cd clients/python
pip install -e ".[dev]"
pytest tests/

# Tidal (separate package)
cd tidal
pip install -e ".[dev]"
pytest tests/
```

### Go clients

```bash
cd clients/go
go test ./...
go build ./cmd/qobuz
```

## Submitting a PR

1. Branch off `main`.
2. Make focused changes — one logical concern per PR.
3. **Tests required for new behavior.** Both Python packages use `aioresponses` to mock the HTTP transport; there are no live-network tests in CI.
4. **Don't break the facade.** Both Qobuz and Tidal Python clients expose the same shape (`client.catalog`, `client.favorites`, `client.playlists`, `client.streaming`, `client.__aenter__` / `__aexit__`). New methods should preserve that parity where possible.
5. Commit messages: Conventional Commits prefix (`feat:`, `fix:`, `chore:`, `docs:`, `test:`). Keep the subject ≤ 70 chars.
6. Open the PR against `main`. CI runs pytest for both Python packages.

## Releasing

1. Tag `vX.Y.Z` on `main`.
2. Downstream consumers (e.g. Libsync) bump their submodule pin to the new SHA and land it on their own `dev` branch.

## Reporting bugs / security issues

- **Bugs / features**: use the GitHub Issues templates.
- **Security vulnerabilities**: see [SECURITY.md](SECURITY.md) — don't open a public issue.

## Code of conduct

Be kind. Disagreements about code are fine; disagreements about people are not.

## License

By contributing, you agree your work will be licensed under [GPL-3.0-only](LICENSE).
