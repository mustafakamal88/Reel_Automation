# TrendCortex — Real System Architecture

> Version: Phase 1 foundation  
> Last updated: 2026-06-29

---

## What is TrendCortex?

TrendCortex is a daily short-form video automation platform.

**Tagline:** Catch trends. Create reels. Publish everywhere.

Core loop:
1. AI/trend engine finds what is trending right now
2. System generates 6 daily video packages (script → storyboard → assets → render)
3. User reviews and approves
4. System auto-uploads to all connected platforms OR user downloads ZIP for manual publishing

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser / Flutter app                                           │
│  React (Vite) + future Flutter mobile                           │
│  No secrets · No tokens · Status only                           │
└──────────────┬──────────────────────────────────────────────────┘
               │  HTTPS REST (JSON)  /api/*
               │  Session cookie (HttpOnly, Secure, SameSite=Lax)
               ▼
┌─────────────────────────────────────────────────────────────────┐
│  Go API Service  (backend/cmd/api)                              │
│  - Authentication & session management                          │
│  - OAuth redirect flow + token exchange                         │
│  - Platform adapter registry (YouTube, TikTok, IG, FB, TH, X)  │
│  - Job queue dispatcher                                         │
│  - Batch/ZIP creation endpoint                                  │
│  - REST/OpenAPI — Flutter-ready                                 │
└────┬──────────────────────────────────────────────┬────────────┘
     │                                              │
     ▼                                              ▼
┌──────────────┐                        ┌───────────────────────┐
│  PostgreSQL  │                        │  Object Storage       │
│  (Railway)   │                        │  (Railway volumes /   │
│              │                        │   S3-compatible)      │
│  - users     │                        │  - video renders      │
│  - workspaces│                        │  - thumbnails         │
│  - batches   │                        │  - captions           │
│  - jobs      │                        │  - daily ZIP files    │
│  - audit_log │                        └───────────────────────┘
│  - token refs│
└──────────────┘
     │
     ▼
┌──────────────────────────────────────────────────────────────────┐
│  Token Vault  (encrypted_payload column in token_vault_refs)     │
│  AES-256-GCM encryption · key stored separately (env var/KMS)   │
│  Raw OAuth tokens NEVER leave this layer                         │
└──────────────────────────────────────────────────────────────────┘
```

---

## Go Backend Services

### Package Layout

```
backend/
  cmd/
    api/
      main.go              — entry point: load config, connect DB, start HTTP server
  internal/
    config/
      config.go            — load all env vars; fail fast if required are missing
    http/
      router.go            — all routes + CORS + session middleware
      health.go            — GET /health → {"ok":true,"service":"trendcortex-api"}
    database/
      db.go                — PostgreSQL connection (lib/pq)
      schema.go            — DDL for all 14 tables; safe to run repeatedly
    models/
      models.go            — Go structs for every DB table
    auth/
      session.go           — HttpOnly session cookies; GenerateID; SessionStore interface
    oauth/
      adapter.go           — Adapter interface + Registry; ErrCredentialsMissing
    platforms/
      youtube.go           — YouTube OAuth + Content Upload adapter
      tiktok.go            — TikTok Content Posting API adapter
      instagram.go         — Instagram Graph API adapter
      facebook.go          — Facebook Reels adapter
      threads.go           — Threads API adapter
      x.go                 — X (Twitter) OAuth 2.0 PKCE adapter
      registry.go          — BuildRegistry(cfg) → oauth.Registry
    jobs/
      types.go             — Job struct, Queue interface, payload types, ZipStructure
    storage/
      storage.go           — BuildBatchZip; compliance-checklist writer
    audit/
      audit.go             — Structured audit log; all OAuth/publish actions logged
```

---

## OAuth Login Flow

```
Frontend                 Go Backend               Platform
   │                         │                       │
   │  POST /api/oauth/       │                       │
   │  start/{platform}       │                       │
   │────────────────────────►│                       │
   │                         │ generate state+PKCE   │
   │                         │ store in oauth_conns  │
   │  { redirect_url }       │                       │
   │◄────────────────────────│                       │
   │                         │                       │
   │  redirect user to       │                       │
   │  platform login page ───┼──────────────────────►│
   │                         │                       │ user logs in
   │                         │                       │ on platform
   │                         │◄──────────────────────│
   │                         │  GET /api/oauth/       │
   │                         │  callback/{platform}  │
   │                         │  ?code=X&state=Y      │
   │                         │                       │
   │                         │ verify state (CSRF)   │
   │                         │ exchange code→tokens  │
   │                         │ encrypt + store tokens│
   │                         │ create platform_account│
   │                         │ write audit log entry │
   │  GET /oauth/success     │                       │
   │◄────────────────────────│                       │
   │  (frontend reads status │                       │
   │   via GET /api/oauth/   │                       │
   │   status — no tokens)   │                       │
```

**Key security rules:**
- No social media passwords ever accepted
- Only official platform OAuth redirect flow
- State parameter verified before code exchange (CSRF protection)
- PKCE used for all platforms that support it (YouTube, X, TikTok)
- Tokens stored AES-256-GCM encrypted in `token_vault_refs.encrypted_payload`
- Frontend never receives raw access or refresh tokens
- Token status only returned to frontend: `not_connected | connected | expired | revoked | needs_review`

---

## Secure Token Storage Design

```
token_vault_refs table:
┌──────────────────────────────────────────────────────┐
│  id               UUID                                │
│  platform_key     TEXT  (youtube, tiktok, etc.)       │
│  workspace_id     UUID                                │
│  encrypted_payload BYTEA  ← AES-256-GCM ciphertext   │
│  key_version      INT    ← rotation tracking          │
│  created_at       TIMESTAMPTZ                         │
│  updated_at       TIMESTAMPTZ                         │
└──────────────────────────────────────────────────────┘

Encryption:
  key = TOKEN_ENCRYPTION_KEY (env var, 32 bytes, hex-encoded)
  nonce = random 12 bytes prepended to ciphertext
  algorithm = AES-256-GCM
  payload = JSON{ access_token, refresh_token, expires_at, scopes }

The encrypted_payload column is the only place raw tokens exist.
The TOKEN_ENCRYPTION_KEY is never stored in the database.
Key rotation re-encrypts all vault entries with the new key version.
```

---

## Daily 6-Video Batch Flow

```
1. Trend Engine (scheduled cron or user-triggered)
   → fetches signals from YouTube Data API, Google Trends, X Search, etc.
   → scores and selects 6 topics

2. Pipeline (render queue worker)
   → generates script, storyboard, asset plan per topic
   → renders 1080×1920 MP4 (60fps, ≤60s)
   → generates thumbnail, captions.srt, per-platform metadata

3. Human Approval Gate
   → user reviews in Approvals page
   → marks each video: approved / needs_review / rejected
   → REQUIRED before first auto-publish is enabled

4. Batch actions (both available once 6 videos are ready):
   a. Auto-upload: POST /api/batch/{date}/publish
   b. Download ZIP: POST /api/batch/{date}/zip
```

---

## ZIP Download Flow

```
POST /api/batch/{date}/zip
→ queues a download_zip_job
→ Go worker assembles:

trendcortex-daily-batch-YYYY-MM-DD.zip
├── video-01/
│   ├── video.mp4          (1080×1920, ≤60s, H.264)
│   ├── thumbnail.jpg      (1080×1920)
│   ├── title.txt
│   ├── description.txt
│   ├── hashtags.txt
│   ├── captions.srt       (or .vtt)
│   └── platforms.json     (per-platform metadata)
├── video-02/ … video-06/
├── batch-summary.json     (all videos + dates + platform list)
└── compliance-checklist.json

GET /api/batch/{date}/zip/download
→ streams the ready ZIP with Content-Disposition: attachment
```

---

## Auto-Upload Job Flow

```
POST /api/batch/{date}/publish
→ creates one publish_job per video per connected platform
   (e.g. 6 videos × 4 platforms = 24 jobs)
→ jobs are written to publish_jobs table with status=queued

Go job worker (runs continuously):
  LOOP:
    1. SELECT ... FOR UPDATE SKIP LOCKED one queued job
    2. Fetch tokens from token_vault_refs, decrypt
    3. Check platform rate limits (platform_rate_limits table)
    4. Call platform adapter: adapter.PublishVideo(ctx, accessToken, req)
    5. On success: update job.status=done, store platform_post_id
    6. On failure: increment retry_count, schedule retry with backoff
    7. Write audit_log entry
    8. After max retries: job.status=failed, alert workspace

Platform compliance before each upload:
  ✓ ai_disclosure flag set in video metadata
  ✓ human_approved=true on the video_asset
  ✓ rate limit not exceeded for the platform today
  ✓ account token_status=connected (not expired/revoked)
```

---

## Railway Deployment Plan

### Environment Setup
```bash
# Create project on Railway: railway.app
railway init

# Add PostgreSQL plugin → DATABASE_URL auto-set
# Add environment variables via Railway dashboard (never in code):
SESSION_SECRET=<openssl rand -hex 32>
TOKEN_ENCRYPTION_KEY=<openssl rand -hex 32>
APP_BASE_URL=https://your-frontend.up.railway.app
API_BASE_URL=https://your-api.up.railway.app
# ... platform credentials
```

### Go Service Dockerfile (add to backend/)
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o trendcortex-api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/trendcortex-api .
EXPOSE 8080
CMD ["./trendcortex-api"]
```

### railway.toml
```toml
[build]
builder = "dockerfile"
dockerfilePath = "backend/Dockerfile"

[deploy]
healthcheckPath = "/health"
healthcheckTimeout = 300
restartPolicyType = "ON_FAILURE"
```

---

## Platform Rate Limits & Policy Notes

| Platform  | Daily limit | Max duration | Max file | Notes |
|-----------|-------------|-------------|---------|-------|
| YouTube   | 100         | 60s         | 256 MB  | Shorts: vertical ≥1:1 ratio |
| TikTok    | 10/day      | 60s         | 287 MB  | Requires Content Posting API approval |
| Instagram | 25/day      | 90s         | 1 GB    | Reels via Graph API; Page required for FB |
| Facebook  | 25/day      | 240s        | 1 GB    | Page required; Reels via Pages API |
| Threads   | 250/day     | 5 min       | 1 GB    | Threads API separate from IG Graph |
| X         | 17/day      | 2m20s       | 512 MB  | Free tier; v1.1 media upload + v2 tweets |

**Compliance requirements enforced before every publish:**
- AI-generated content disclosure (TikTok, YouTube require labelling by 2025 policy)
- No impersonation or deepfakes without consent
- No spam / repetitive bulk posting (violates all platform ToS)
- No fake engagement signals
- No copyrighted music without licence (use royalty-free only)
- No medical, legal, or financial claims without professional review gate
- Paid partnership / affiliate disclosure field on video_assets
- Human approval on every video before first auto-publish

---

## Future Flutter / Mobile API Compatibility

The Go API is designed for cross-platform clients from day one:

- Pure JSON REST (no server-side HTML rendering)
- No cookies for auth on mobile — mobile clients use Bearer token in Authorization header
  - Session cookie = web; Bearer token = mobile (same session backend, different transport)
- All endpoints return consistent JSON error shapes: `{"error": "..."}`
- All list endpoints will support pagination: `?cursor=...&limit=...`
- OpenAPI spec to be generated from route annotations (planned: Phase 2)
- CORS configured per-origin; mobile apps bypass CORS but need valid session token

Flutter integration will call:
```
GET  /health                  → service alive check
POST /api/auth/login          → returns session token
GET  /api/oauth/status        → platform connection statuses
POST /api/batch/{date}/publish → queue auto-upload
POST /api/batch/{date}/zip    → create download ZIP
GET  /api/batch/{date}/status → polling job status
```
