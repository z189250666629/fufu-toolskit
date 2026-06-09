# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`fufu-combine` is a token/card merging tool ("合卡") with a Go backend and a vanilla single-page frontend. No frontend build step is required.

## Commands

```bash
go run .        # start server (port 3456 by default)
go test ./...   # compile/check Go backend
```

Optional npm-style wrappers are kept in `package.json`:

```bash
npm run dev
npm run start
npm run build
```

## Architecture

- `main.go` — Go HTTP server. Handles auth, merge jobs, static file serving, and proxying to the upstream New-API-compatible backend configured in `config.json`.
- `public/index.html` — Single-page frontend (vanilla JS, inline CSS). 3-step wizard: input keys → confirm → result.

### Key internals (`main.go`)

- **Config**: read from `config.json` at startup (`url`, `token`, `userId`, optional `quotaUnit`).
- **Auth**: in-memory session store (4h TTL); two roles (`admin`, `user`) identified by hardcoded SHA-256 password hashes.
- **Merge jobs**: in-memory store (30min TTL) with per-token mutex locks; jobs are async and polled via `GET /api/merge-status/:jobId`.
- **Upstream proxy**: all token operations proxy to the URL in `config.json`.
- **Trace DB**: SQLite merge provenance is stored in `data/fufu-combine.db` and ignored by git.

### Routes

| Route | Auth | Purpose |
|---|---|---|
| `POST /api/auth` | public | Login, returns session token |
| `GET /api/session` | user/admin | Validate session |
| `POST /api/search-keys` | public | Batch token lookup |
| `POST /api/public-merge` | public | Guest merge (day cards → weekly only) |
| `POST /api/merge` | user/admin | Full merge with type/quota control |
| `GET /api/merge-status/:jobId` | public | Poll async job |
| `POST /api/generate` | admin | Batch create tokens |
| `DELETE /api/token/:id` | user/admin | Delete token through upstream |

## Sensitive files

`config.json` contains a plaintext API token — do not commit real values.
