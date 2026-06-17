# AGENTS.md

## Commands

- Start standalone module server for local debugging: `npm start` or `go run ./cmd/fufu-act` (port `SLOT_PORT`, default `18820`; production is served by `apps/fufu-tool-site`)
- Test: `npm test` or `go test -count=1 ./...`

## Architecture

Go backend serving the existing static activity frontend from `public/`.

- `main.go` — HTTP entry point, SQLite schema/migrations, activity routes, scratch routes, admin stats, credit queue worker.
- Shared business logic comes from `packages/go/fufu` (`config`, `newapi`, `tokens`, `activity`, `auth`).
- Static frontend files remain `public/index.html`; activity administration is handled by the unified `apps/fufu-tool-site` `/admin` page through `/api/admin/*`.

Runtime SQLite files live in `data/` and must not be committed.
