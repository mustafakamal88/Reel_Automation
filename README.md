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

```bash
npm install
npm run dev
```

Open [http://localhost:5173](http://localhost:5173)

Backend:

```bash
cd backend
go run ./cmd/api
```

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
