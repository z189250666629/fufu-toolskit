# AGENTS.md

## Commands

- Start server: `npm start` or `go run .` (port `SLOT_PORT`, default `18820`)
- Test: `npm test` or `go test -count=1 ./...`

## Architecture

Go backend serving the existing static activity frontend from `public/`.

- `main.go` — HTTP entry point, SQLite schema/migrations, activity routes, scratch routes, admin stats, credit queue worker.
- Shared business logic comes from `packages/go/fufu` (`config`, `newapi`, `tokens`, `activity`, `auth`).
- Static frontend files remain `public/index.html` and `public/admin.html`.

Runtime SQLite files live in `data/` and must not be committed.
