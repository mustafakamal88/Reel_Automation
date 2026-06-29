# SIGNAL Trend Engine

A real-time trend intelligence dashboard for short-form content creators. SIGNAL monitors social signals across 6 platforms, scores topic candidates using a multi-factor algorithm, generates AI-scripted content ideas daily, and tracks approval workflow through to performance analytics.

## What it does

- **Signals** — live feed of trending searches, hashtags, and viral videos across Google Trends, YouTube, TikTok, Instagram, X/Twitter, Facebook, and Threads
- **Scoring** — ranks keyword candidates by trend velocity, cross-platform reach, views/hour, engagement, search interest, niche match, low-competition bonus, saturation penalty, and risk
- **Today's 6** — one-click generation of 6 AI-scored content topics with hooks, 24-second scripts, captions, and platform targets
- **Topic Drawer** — per-topic detail view with timestamped script breakdown and approve/reject action
- **Competitors** — tracks competitor accounts by followers, views/hr, and latest breakout content
- **Approvals** — review queue for all generated topics with status tracking (pending / approved / rejected)
- **Performance** — own-account metrics: views, retention, best platform, per-post breakdown, and platform split chart
- **Settings** — configure niche, content style, brand voice, region, active platforms, risk tolerance, and API keys

## Tech stack

- React 19 + TypeScript
- Vite 8
- CSS custom properties (no framework)
- Playwright (dev tooling — screenshots only, no test suite yet)

## Local development

```bash
npm install
npm run dev
```

Open [http://localhost:5173](http://localhost:5173)

## Build

```bash
npm run build
```

Output goes to `dist/`.

## Current status

**Demo / mock providers only.** All data is generated from static mock providers in `src/lib/providers/`. No real API calls are made. The app is fully functional as a UI prototype.

## Future API integration TODOs

Connect real data sources by adding the following env vars to `.env.local`:

| Variable | API |
|---|---|
| `VITE_YOUTUBE_API_KEY` | YouTube Data API v3 |
| `VITE_TIKTOK_CLIENT_KEY` | TikTok Research API |
| `VITE_META_ACCESS_TOKEN` | Meta Graph API (IG + FB) |
| `VITE_X_BEARER_TOKEN` | X API v2 |
| `VITE_ANTHROPIC_API_KEY` | Anthropic API (content generation) |

Additional integration work needed:
- Replace mock providers in `src/lib/providers/` with real API clients
- Wire `content-generator.ts` to actual Claude API calls (`VITE_ANTHROPIC_API_KEY`)
- Add a backend proxy or edge function to keep API keys server-side
- Implement Playwright test suite for regression coverage
- Add scheduling/publishing integration (Buffer, later.com, or direct API)
# Reel_Automation
