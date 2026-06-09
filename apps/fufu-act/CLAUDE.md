# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- Start server: `npm start` (port 18820, or `SLOT_PORT=<n>`)
- No test suite; no build step required

## Architecture

Plain Node.js ESM app (no TypeScript, no frontend build). Express 5 backend serves static HTML from `public/` and a REST API.

**Key files:**

- `server/index.mjs` — entry point; wires Express, DB init, credit worker startup
- `server/db.mjs` — SQLite schema via `better-sqlite3`; tables: `cards`, `spin_log`, `scratch_games`, `credit_queue`
- `server/routes.mjs` — REST endpoints: `/api/login`, `/api/spin`, `/api/scratch/*`, `/api/prizes`
- `server/slot.mjs` — game logic: `SPIN_MAP`, `PRIZE_POOL`, `SCRATCH_REWARDS`
- `server/credit-worker.mjs` — async queue that drains `credit_queue` and calls external API to issue credits
- `scripts/api-act.mjs` — external API calls to MCY shop (card validation) and FuFu token API (credit issuance)

**Data flow:** User submits a card key → `routes.mjs` validates via `api-act.mjs` → game result written to SQLite → winner row inserted into `credit_queue` → `credit-worker.mjs` polls queue and calls FuFu API to deposit credits.

**Databases** live in `data/` (`slot.db` for game state, `act.db`/`fufu.db` for external references). Do not commit `data/*.db` files.
