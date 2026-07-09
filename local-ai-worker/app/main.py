from __future__ import annotations

import html
import json
import logging
import os
import shlex
import subprocess
import time
import uuid
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, AsyncIterator, Optional

from fastapi import BackgroundTasks, Depends, FastAPI, Header, HTTPException, Response
from fastapi.responses import FileResponse, HTMLResponse
from pydantic import BaseModel, Field

logger = logging.getLogger("trendcortex.local_ai_worker")
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")


def _env_bool(name: str, default: bool = False) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def _generator_mode() -> str:
    mode = os.getenv("WORKER_GENERATOR_MODE", "").strip().lower()
    if mode:
        return mode
    if _env_bool("WORKER_DEV_STUB", False):
        return "dev_stub"
    return "manual"


class Settings(BaseModel):
    host: str = Field(default_factory=lambda: os.getenv("WORKER_HOST", "127.0.0.1"))
    port: int = Field(default_factory=lambda: int(os.getenv("WORKER_PORT", "8787")))
    token: str = Field(default_factory=lambda: os.getenv("WORKER_TOKEN", "").strip())
    output_dir: Path = Field(
        default_factory=lambda: Path(os.getenv("WORKER_OUTPUT_DIR", Path(__file__).resolve().parents[1] / "outputs"))
    )
    dev_stub: bool = Field(default_factory=lambda: _env_bool("WORKER_DEV_STUB", False))
    generator_mode: str = Field(default_factory=_generator_mode)
    generator_command: str = Field(default_factory=lambda: os.getenv("WORKER_GENERATOR_COMMAND", "").strip())
    model_hint: str = Field(default_factory=lambda: os.getenv("WORKER_MODEL_HINT", "auto").strip() or "auto")

    def model_post_init(self, __context: Any) -> None:
        self.generator_mode = self.generator_mode.strip().lower() or "manual"
        if self.dev_stub and self.generator_mode == "manual" and not os.getenv("WORKER_GENERATOR_MODE"):
            self.generator_mode = "dev_stub"


class SceneRequest(BaseModel):
    visual_prompt: str
    negative_prompt: str = ""
    duration_seconds: float = 8
    aspect_ratio: str = "9:16"
    model_hint: str = "auto"
    style_preset: str = "realistic_editorial"


class JobSummary(BaseModel):
    id: str
    status: str
    error: str = ""
    worker_message: str = ""
    next_action: str = ""
    visual_prompt: str = ""
    negative_prompt: str = ""
    duration_seconds: Optional[float] = None
    aspect_ratio: str = ""
    model_hint: str = ""
    style_preset: str = ""
    created_at: str = ""
    updated_at: str = ""
    expected_output_path: str = ""
    manual_output_path: str = ""
    timeout_seconds: int = 120
    progress_percent: int = 0
    current_step: str = ""
    estimated_next_action: str = ""
    downloadable: bool = False


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def create_app(settings: Optional[Settings] = None) -> FastAPI:
    cfg = settings or Settings()
    cfg.output_dir.mkdir(parents=True, exist_ok=True)

    def config_status() -> dict[str, Any]:
        auth_required = bool(cfg.token)
        return {
            "auth_required": auth_required,
            "token_configured": auth_required,
            "generator_mode": cfg.generator_mode,
            "auto_command_configured": bool(cfg.generator_command),
        }

    @asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[None]:
        status = config_status()
        logger.info(
            "worker config auth_required=%s token_configured=%s host=%s port=%s output_dir=%s generator_mode=%s auto_command_configured=%s",
            status["auth_required"],
            status["token_configured"],
            cfg.host,
            cfg.port,
            cfg.output_dir,
            cfg.generator_mode,
            status["auto_command_configured"],
        )
        yield

    app = FastAPI(title="TrendCortex Local AI Video Worker", version="0.1.0", lifespan=lifespan)
    app.state.settings = cfg

    def require_auth(authorization: Optional[str] = Header(default=None)) -> None:
        if not cfg.token:
            return
        expected = f"Bearer {cfg.token}"
        if authorization != expected:
            raise HTTPException(status_code=401, detail="missing or invalid bearer token")

    auth_dep = Depends(require_auth)

    def job_dir(job_id: str) -> Path:
        if "/" in job_id or "\\" in job_id or job_id in {"", ".", ".."}:
            raise HTTPException(status_code=404, detail="job not found")
        return cfg.output_dir / job_id

    def job_file(job_id: str) -> Path:
        return job_dir(job_id) / "job.json"

    def output_file(job_id: str) -> Path:
        return job_dir(job_id) / "output.mp4"

    def prompt_file(job_id: str) -> Path:
        return job_dir(job_id) / "prompt.txt"

    def read_job(job_id: str) -> dict[str, Any]:
        path = job_file(job_id)
        if not path.exists():
            raise HTTPException(status_code=404, detail="job not found")
        return json.loads(path.read_text(encoding="utf-8"))

    def write_job(job: dict[str, Any]) -> None:
        path = job_file(job["id"])
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(job, indent=2, sort_keys=True), encoding="utf-8")

    def output_is_valid(path: Path) -> bool:
        if not path.exists() or path.stat().st_size <= 0:
            return False
        try:
            header = path.read_bytes()[:64]
        except OSError:
            return False
        if b"ftyp" in header:
            return True
        try:
            completed = subprocess.run(
                ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", str(path)],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                timeout=5,
                check=False,
            )
            return completed.returncode == 0
        except (OSError, subprocess.SubprocessError):
            return False

    def refresh_job(job: dict[str, Any]) -> dict[str, Any]:
        out = output_file(job["id"])
        if output_is_valid(out) and job.get("status") != "completed":
            job["status"] = "completed"
            job["progress_percent"] = 100
            job["current_step"] = "Video ready"
            job["updated_at"] = now_iso()
            job["output_path"] = str(out)
            job["worker_message"] = "Generated output.mp4 found."
            job["estimated_next_action"] = "Download generated video."
            write_job(job)
            logger.info("job completed id=%s output=%s", job["id"], out)
        return job

    def summarize(job: dict[str, Any]) -> JobSummary:
        req = job.get("request", {})
        expected_output = str(output_file(job["id"]))
        status = job.get("status", "pending")
        mode = job.get("generator_mode", cfg.generator_mode)
        next_action = job.get(
            "next_action",
            "Manual worker mode: generate this scene in Pinokio/Wan2GP, then save it as output.mp4 in the shown job folder.",
        )
        worker_message = job.get("worker_message", "")
        if status == "generator_not_configured" and not worker_message:
            worker_message = "AI video generator is connected but automatic model generation is not configured yet."
        elif status == "pending" and mode == "manual" and not worker_message:
            worker_message = "Place generated MP4 at outputs/<job-id>/output.mp4"
        return JobSummary(
            id=job["id"],
            status=status,
            error=job.get("error", ""),
            worker_message=worker_message,
            next_action=next_action,
            visual_prompt=req.get("visual_prompt", ""),
            negative_prompt=req.get("negative_prompt", ""),
            duration_seconds=req.get("duration_seconds"),
            aspect_ratio=req.get("aspect_ratio", ""),
            model_hint=req.get("model_hint", ""),
            style_preset=req.get("style_preset", ""),
            created_at=job.get("created_at", ""),
            updated_at=job.get("updated_at", ""),
            expected_output_path=expected_output,
            manual_output_path=expected_output,
            timeout_seconds=int(job.get("timeout_seconds", 120)),
            progress_percent=int(job.get("progress_percent", 0)),
            current_step=job.get("current_step", ""),
            estimated_next_action=job.get("estimated_next_action", ""),
            downloadable=status == "completed" and output_is_valid(Path(job.get("output_path", expected_output))),
        )

    def list_jobs() -> list[JobSummary]:
        jobs: list[JobSummary] = []
        for path in sorted(cfg.output_dir.glob("*/job.json"), reverse=True):
            try:
                jobs.append(summarize(refresh_job(json.loads(path.read_text(encoding="utf-8")))))
            except (OSError, json.JSONDecodeError, KeyError) as exc:
                logger.warning("skip unreadable job file path=%s error=%s", path, exc)
        return jobs

    def background_process(job_id: str) -> None:
        job = read_job(job_id)
        mode = job.get("generator_mode", cfg.generator_mode)
        if mode == "manual":
            logger.info("job pending id=%s expected_output=%s", job_id, output_file(job_id))
            return
        if mode == "auto_command":
            if not cfg.generator_command:
                job["status"] = "generator_not_configured"
                job["progress_percent"] = 30
                job["current_step"] = "Generator not configured"
                job["worker_message"] = "AI video generator is connected but automatic model generation is not configured yet."
                job["estimated_next_action"] = "Configure WORKER_GENERATOR_COMMAND."
                job["updated_at"] = now_iso()
                write_job(job)
                return
            job["status"] = "running"
            job["progress_percent"] = 50
            job["current_step"] = "Generating video scenes"
            job["worker_message"] = "Running configured local generator command."
            job["updated_at"] = now_iso()
            write_job(job)
            req = job.get("request", {})
            prompt = str(req.get("visual_prompt", ""))
            prompt_file(job_id).write_text(prompt, encoding="utf-8")
            out = output_file(job_id)
            values = {
                "job_id": job_id,
                "prompt_file": str(prompt_file(job_id)),
                "output_path": str(out),
                "duration_seconds": str(req.get("duration_seconds", "")),
                "aspect_ratio": str(req.get("aspect_ratio", "")),
                "style_preset": str(req.get("style_preset", "")),
            }
            try:
                command = cfg.generator_command.format(**values)
                completed = subprocess.run(
                    shlex.split(command),
                    cwd=str(job_dir(job_id)),
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    timeout=max(30, int(job.get("timeout_seconds", 120))),
                    check=False,
                )
                job = read_job(job_id)
                if completed.returncode != 0:
                    job["status"] = "failed"
                    job["error"] = (completed.stderr or completed.stdout or "generator command failed")[-2000:]
                    job["current_step"] = "Generation failed"
                    job["updated_at"] = now_iso()
                    write_job(job)
                    return
            except Exception as exc:  # noqa: BLE001 - persisted as worker job error
                job = read_job(job_id)
                job["status"] = "failed"
                job["error"] = str(exc)
                job["current_step"] = "Generation failed"
                job["updated_at"] = now_iso()
                write_job(job)
                return
            job = read_job(job_id)
            if not output_is_valid(out):
                job["status"] = "failed"
                job["error"] = "generator command finished but output.mp4 is missing or invalid"
                job["current_step"] = "Generation failed"
                job["updated_at"] = now_iso()
                write_job(job)
                return
            job["status"] = "completed"
            job["progress_percent"] = 100
            job["current_step"] = "Video ready"
            job["output_path"] = str(out)
            job["worker_message"] = "Generated output.mp4 is ready."
            job["estimated_next_action"] = "Download generated video."
            job["updated_at"] = now_iso()
            write_job(job)
            return
        if mode != "dev_stub":
            job["status"] = "failed"
            job["error"] = f"unsupported generator mode: {mode}"
            job["updated_at"] = now_iso()
            write_job(job)
            return
        time.sleep(0.05)
        out = output_file(job_id)
        out.write_bytes(b"\x00\x00\x00\x18ftypmp42trendcortex worker dev stub output\n")
        job = read_job(job_id)
        job["status"] = "completed"
        job["progress_percent"] = 100
        job["current_step"] = "Video ready"
        job["updated_at"] = now_iso()
        job["output_path"] = str(out)
        job["worker_message"] = "Dev stub output is ready."
        write_job(job)
        logger.info("dev stub completed job id=%s output=%s", job_id, out)

    @app.get("/", response_class=HTMLResponse, dependencies=[auth_dep])
    def dashboard() -> str:
        rows = []
        for job in list_jobs():
            rows.append(
                "<tr>"
                f"<td><code>{html.escape(job.id)}</code></td>"
                f"<td>{html.escape(job.status)}</td>"
                f"<td>{html.escape(job.visual_prompt)}</td>"
                f"<td><code>{html.escape(job.expected_output_path)}</code></td>"
                f"<td>{html.escape(job.worker_message or job.error)}</td>"
                "</tr>"
            )
        body = "\n".join(rows) or "<tr><td colspan='5'>No jobs yet.</td></tr>"
        return f"""<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TrendCortex Local AI Worker</title>
  <style>
    body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; color: #111827; }}
    table {{ border-collapse: collapse; width: 100%; table-layout: fixed; }}
    th, td {{ border-bottom: 1px solid #e5e7eb; padding: 10px; text-align: left; vertical-align: top; word-wrap: break-word; }}
    th {{ font-size: 12px; text-transform: uppercase; color: #4b5563; }}
    code {{ font-size: 12px; }}
  </style>
</head>
<body>
  <h1>TrendCortex Local AI Worker</h1>
  <p>Output directory: <code>{html.escape(str(cfg.output_dir))}</code></p>
  <p><strong>Generator mode:</strong> <code>{html.escape(cfg.generator_mode)}</code></p>
  <p><strong>Manual worker mode:</strong> generate each scene in Pinokio/Wan2GP, then place generated MP4 at <code>outputs/&lt;job-id&gt;/output.mp4</code>.</p>
  <table>
    <thead><tr><th>Job</th><th>Status</th><th>Visual prompt</th><th>Expected output file</th><th>Worker message</th></tr></thead>
    <tbody>{body}</tbody>
  </table>
</body>
</html>"""

    @app.get("/health", dependencies=[auth_dep])
    @app.post("/health", dependencies=[auth_dep])
    def health() -> dict[str, Any]:
        return {
            "ok": True,
            "status": "ok",
            "message": "TrendCortex local AI worker ready",
            "output_dir": str(cfg.output_dir),
            "dev_stub": cfg.generator_mode == "dev_stub",
            "generator_mode": cfg.generator_mode,
            "auto_command_configured": bool(cfg.generator_command),
            "model_hint": cfg.model_hint,
        }

    @app.get("/debug/config")
    def debug_config() -> dict[str, Any]:
        return config_status()

    @app.post("/generate-scene", dependencies=[auth_dep])
    def generate_scene(scene: SceneRequest, background_tasks: BackgroundTasks) -> dict[str, Any]:
        job_id = "job_" + uuid.uuid4().hex
        created = now_iso()
        job = {
            "id": job_id,
            "status": "pending",
            "generator_mode": cfg.generator_mode,
            "created_at": created,
            "updated_at": created,
            "request": scene.model_dump(),
            "expected_output_path": str(output_file(job_id)),
            "manual_output_path": str(output_file(job_id)),
            "timeout_seconds": 120,
            "progress_percent": 30 if cfg.generator_mode == "auto_command" else 0,
            "current_step": "Sending to local AI worker" if cfg.generator_mode == "auto_command" else "Waiting for manual output",
            "next_action": "Manual worker mode: generate this scene in Pinokio/Wan2GP, then save it as output.mp4 in the shown job folder.",
            "worker_message": "Place generated MP4 at outputs/<job-id>/output.mp4" if cfg.generator_mode == "manual" else "",
            "estimated_next_action": "Generate output.mp4 with configured local model.",
        }
        write_job(job)
        logger.info(
            "job created id=%s duration=%s aspect=%s style=%s output=%s prompt=%s",
            job_id,
            scene.duration_seconds,
            scene.aspect_ratio,
            scene.style_preset,
            output_file(job_id),
            scene.visual_prompt,
        )
        background_tasks.add_task(background_process, job_id)
        return summarize(job).model_dump()

    @app.get("/jobs", dependencies=[auth_dep])
    def jobs() -> dict[str, Any]:
        summaries = list_jobs()
        return {"jobs": [job.model_dump() for job in summaries]}

    @app.get("/jobs/{job_id}", dependencies=[auth_dep])
    def get_job(job_id: str) -> dict[str, Any]:
        job = refresh_job(read_job(job_id))
        return summarize(job).model_dump()

    @app.get("/jobs/{job_id}/output", dependencies=[auth_dep])
    def get_output(job_id: str) -> Response:
        job = refresh_job(read_job(job_id))
        if job.get("status") != "completed":
            raise HTTPException(status_code=409, detail="job output is not ready")
        out = output_file(job_id)
        if not out.exists():
            raise HTTPException(status_code=404, detail="job output not found")
        return FileResponse(out, media_type="video/mp4", filename=f"{job_id}.mp4")

    return app


app = create_app()
