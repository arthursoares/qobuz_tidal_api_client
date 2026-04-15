<!--
- Branch off `main`, target `main`. See CONTRIBUTING.md.
- CI runs pytest on both Python packages — must pass.
- Title: conventional-commit prefix (feat:/fix:/chore:/docs:/test:).
-->

## Summary

<!-- What changes and why, in one or two sentences. -->

## Scope

- [ ] Qobuz Python (`clients/python/qobuz/`)
- [ ] Tidal Python (`clients/python/tidal/`)
- [ ] Qobuz Go (`clients/go/`)
- [ ] Tidal Go (`clients/go/tidal/`)
- [ ] Docs / api-spec.md
- [ ] CI / tooling

## Test plan

<!-- New tests added? Manual verification done? -->
- [ ] Added/updated tests in the relevant `tests/` dir
- [ ] `pytest tests/` passes locally for the affected package(s)
- [ ] If touching the HTTP transport: verified mocks cover the new response shape

## Linked issues

<!-- "Fixes #N" / "Refs #N". If a downstream consumer (e.g. Libsync) needs a matching bump, note it here. -->

## Notes for reviewers

<!-- Tricky edge cases, decisions you're unsure about, follow-up work. Delete if none. -->
