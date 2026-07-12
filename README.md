# TrendCortex / Reels Automation

TrendCortex is a short-form video automation workspace backed by a Go API and Postgres. The frontend shows honest empty states until real providers, accounts, and backend data are connected.

## What it does

- **Signals** — empty until real trend sources are connected.
- **Scoring** — empty until real trend items exist in the backend.
- **Today's 6** — empty until real reel plans are created.
- **Real Pipeline** — reads trend, scoring, daily batch, render, export, and publish job state from the Go backend.
- **Pipeline Studio** — keeps the real Phase 4D render + ZIP test flow.
- **Connections and Settings** — show connection status only; secrets stay in backend environment variables.

## Tech stack

- React 19 + TypeScript
- Vite 8
- Go backend
- Postgres
- CSS custom properties (no framework)
- Playwright for browser smoke checks

## Local development

Before any UI change, follow [docs/UI_UX_ENGINEERING_STANDARD.md](docs/UI_UX_ENGINEERING_STANDARD.md).

1. Start Docker Desktop and confirm the engine is running:

```bash
docker version
docker compose version
```

2. Create a private local environment file:

```bash
cp .env.example .env
chmod 600 .env
```

Generate separate local-only values for `SESSION_SECRET` and
`TOKEN_ENCRYPTION_KEY`:

```bash
openssl rand -hex 32
openssl rand -hex 32
```

Paste one generated value into each variable in `.env`. Do not reuse one value
for both variables.

Add your YouTube Data API v3 key to `YOUTUBE_API_KEY` in `.env` to enable real
video validation, enrichment, and keyword research. Keep all API keys and
secrets backend-only; never use `VITE_` variables for secrets.

Do not commit `.env`. Do not point local development at the Railway production
database or copy Railway production values into `.env`.

3. Start local PostgreSQL:

```bash
docker compose -f compose.dev.yml config
docker compose -f compose.dev.yml up -d
docker compose -f compose.dev.yml ps
docker compose -f compose.dev.yml logs --tail=100
```

The Compose database uses the official `postgres:16` image with database
`trendcortex`, user `trendcortex`, password `trendcortex_dev`, and the named
project volume `reelsautomation_trendcortex_postgres_dev`.

This checkout maps the container's PostgreSQL port to local host port `55432`
because port `5432` may already be used by a local PostgreSQL installation.
Keep `DATABASE_URL` in `.env` aligned with the host port in `compose.dev.yml`.

To intentionally reset the local development database, stop Compose and remove
only this project's named volume after confirming you do not need the data:

```bash
docker compose -f compose.dev.yml down
docker volume rm reelsautomation_trendcortex_postgres_dev
```

For normal shutdown, preserve the database volume:

```bash
docker compose -f compose.dev.yml down
```

4. Start the backend:

```bash
cd backend
GOCACHE=/private/tmp/reels-go-cache go run ./cmd/api
```

The backend loads the root `.env`, connects to Postgres, applies migrations,
and listens on [http://127.0.0.1:8080](http://127.0.0.1:8080). Check:

```bash
curl -i http://127.0.0.1:8080/health
curl -i http://127.0.0.1:8080/api/trends/filters
```

5. Start the frontend:

```bash
npm install
npm run dev
```

Open [http://127.0.0.1:5173/ai-tools/trending-keywords](http://127.0.0.1:5173/ai-tools/trending-keywords).
The Vite proxy forwards API requests to
[http://127.0.0.1:8080](http://127.0.0.1:8080) when `VITE_API_BASE_URL` is
blank.

Stop the frontend and backend with `Ctrl-C` in their terminal sessions.

Backend runtime artifacts:

- `EXPORT_DIR` controls where ZIP exports are written.
- `MEDIA_OUTPUT_DIR` controls where generated video/thumbnail artifacts are written.
- If unset, both default to writable temp paths under `/tmp/trendcortex/` on Unix-like systems.
- For persistent production exports, point these variables at a Railway volume or future object-storage staging path.

## Build

```bash
npm run build
```

Output goes to `dist/`.

## Current status

The app intentionally does not create placeholder content. Without connected providers and backend records, the UI should show empty states such as “No live trend data connected yet” and “No real reels generated yet.”

## Provider integration notes

Secrets must be configured server-side only. Do not use `VITE_` variables for API keys, OAuth secrets, refresh tokens, or model provider keys.

Required backend variables include `DATABASE_URL`, `SESSION_SECRET`, and `TOKEN_ENCRYPTION_KEY`.
