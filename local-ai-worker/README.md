# TrendCortex Local AI Video Worker

This is the local/GPU worker that TrendCortex can call for `local_ai_scene_v1` scene clips.

The worker supports three generator modes:

- `auto_command`: production/local automation mode. TrendCortex submits scene prompts and the worker runs your configured generator command to create `output.mp4`.
- `manual`: developer-only fallback. TrendCortex creates pending job folders and you place an externally generated MP4 into the expected output path.
- `dev_stub`: test-only mode. It writes a tiny stub output and must not be used for production videos.

Production should use `auto_command`. The worker never fakes production AI generation; a job is completed only when `output.mp4` exists and passes the worker's MP4 validation.

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
| `WORKER_GENERATOR_MODE` | `manual` | `auto_command`, `manual`, or `dev_stub`. |
| `WORKER_GENERATOR_COMMAND` | empty | Required for `auto_command`; command template that writes `{output_path}`. |
| `WORKER_MODEL_HINT` | `auto` | Optional model hint returned in health/status metadata. |
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

## Auto Command Mode

Set `WORKER_GENERATOR_MODE=auto_command` and provide `WORKER_GENERATOR_COMMAND`. The command is run once per scene job and must create a valid MP4 at `{output_path}`.

Supported placeholders:

```text
{job_id}
{prompt_file}
{output_path}
{duration_seconds}
{aspect_ratio}
{style_preset}
```

Example shape:

```bash
WORKER_GENERATOR_MODE=auto_command \
WORKER_GENERATOR_COMMAND='your-generator --prompt-file {prompt_file} --duration {duration_seconds} --aspect {aspect_ratio} --style {style_preset} --out {output_path}' \
python3 -m uvicorn app.main:app --host 127.0.0.1 --port 8787
```

If `auto_command` is selected without `WORKER_GENERATOR_COMMAND`, jobs return `generator_not_configured` immediately so the product UI can explain that automatic generation is not configured.

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

Manual mode is intended for developer validation only.

1. Start this worker locally with `WORKER_GENERATOR_MODE=manual`.
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
