# TrendCortex Local AI Video Worker

This is the local/GPU worker that TrendCortex can call for `local_ai_scene_v1` scene clips.

The first implementation is intentionally manual. It does not fake production AI generation. TrendCortex submits scene prompts, the worker creates pending job folders, and you place the generated MP4 into the expected output path after using Pinokio/Wan2GP or another local video tool.

## Run Locally

```bash
cd local-ai-worker
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements.txt
WORKER_TOKEN=trend-worker-123 python3 -m uvicorn app.main:app --host 127.0.0.1 --port 8787
```

Configuration:

| Variable | Default | Notes |
| --- | --- | --- |
| `WORKER_HOST` | `127.0.0.1` | Host used by `run_worker.py`. |
| `WORKER_PORT` | `8787` | Port used by `run_worker.py`. |
| `WORKER_TOKEN` | empty | If set, every endpoint requires `Authorization: Bearer <token>`. |
| `WORKER_OUTPUT_DIR` | `local-ai-worker/outputs` | Job folders and generated MP4s. |
| `WORKER_DEV_STUB` | `false` | Dev/test only. When true, the worker writes a small stub output file. Disabled by default. |

Alternative runner:

```bash
cd local-ai-worker
./run_worker.sh
```

`run_worker.sh` defaults `WORKER_TOKEN` to `trend-worker-123` when it is not already set.

Health check:

```bash
curl -H "Authorization: Bearer trend-worker-123" http://127.0.0.1:8787/health
```

Safe config check. This never prints the token value:

```bash
curl http://127.0.0.1:8787/debug/config
```

Dashboard:

Open `http://127.0.0.1:8787/` to see pending/completed jobs, prompts, and expected output paths.

## Connect TrendCortex

Set these in the TrendCortex backend environment:

```bash
LOCAL_AI_WORKER_URL=http://127.0.0.1:8787
LOCAL_AI_WORKER_TOKEN=change-me
```

Use the same secret value for `WORKER_TOKEN` on the worker and `LOCAL_AI_WORKER_TOKEN` on TrendCortex.

## Troubleshooting Local Runs

If `/health` returns `401` after changing `WORKER_TOKEN`, check for a stale process. The worker reads `WORKER_TOKEN` when the uvicorn process starts.

Find and stop anything already listening on port `8787`:

```bash
lsof -nP -iTCP:8787 -sTCP:LISTEN
kill <PID>
```

Run with token auth:

```bash
cd local-ai-worker
WORKER_TOKEN=trend-worker-123 python3 -m uvicorn app.main:app --host 127.0.0.1 --port 8787
curl -H "Authorization: Bearer trend-worker-123" http://127.0.0.1:8787/health
```

Expected response includes:

```json
{"ok": true}
```

Run without token auth for local-only testing:

```bash
cd local-ai-worker
unset WORKER_TOKEN
python3 -m uvicorn app.main:app --host 127.0.0.1 --port 8787
curl http://127.0.0.1:8787/health
```

Expected response includes:

```json
{"ok": true}
```

Use `127.0.0.1` in local curl commands so you know the request is going to the IPv4 listener started by these examples.

## Manual Pinokio/Wan2GP Flow

1. Start this worker locally.
2. Generate AI scenes from TrendCortex.
3. Open the worker dashboard and copy each job's `visual_prompt`.
4. Paste the prompt into Pinokio/Wan2GP or another local video generator.
5. Generate a clip with the requested duration and aspect ratio.
6. Save or copy the MP4 to the dashboard's expected output path:

```text
local-ai-worker/outputs/<job-id>/output.mp4
```

The next `GET /jobs/<job-id>` call marks the job `completed`, and `GET /jobs/<job-id>/output` downloads the MP4.

## Expose Safely To Railway

Keep the worker on your machine and expose it through a tunnel only when needed.

Cloudflare Tunnel example:

```bash
cloudflared tunnel --url http://127.0.0.1:8787
```

ngrok example:

```bash
ngrok http 8787
```

Then set Railway backend variables:

```bash
LOCAL_AI_WORKER_URL=https://your-tunnel-host.example
LOCAL_AI_WORKER_TOKEN=change-me
```

Security notes:

- Always set `WORKER_TOKEN` before exposing the worker.
- Do not expose the worker without a tunnel access policy or bearer token.
- Keep the tunnel URL private.
- Stop the tunnel when generation work is done.

## API

`GET /health` and `POST /health`

Returns worker status and output directory.

`POST /generate-scene`

Request:

```json
{
  "visual_prompt": "Realistic vertical video...",
  "negative_prompt": "blurry, watermark",
  "duration_seconds": 8,
  "aspect_ratio": "9:16",
  "model_hint": "auto",
  "style_preset": "realistic_editorial"
}
```

Response:

```json
{
  "id": "job_...",
  "status": "pending"
}
```

`GET /jobs/{id}`

Returns status, prompt, timestamps, and expected output path.

`GET /jobs/{id}/output`

Downloads the completed MP4.

## Tests

```bash
cd local-ai-worker
python -m pytest
```
