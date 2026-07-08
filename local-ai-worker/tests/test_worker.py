import asyncio
import json
from pathlib import Path
from typing import Any, Optional

from app.main import Settings, create_app


def make_app(tmp_path: Path, token: str = "", dev_stub: bool = False):
    return create_app(Settings(output_dir=tmp_path, token=token, dev_stub=dev_stub))


def auth(token: str = "secret") -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


def request(
    app: Any,
    method: str,
    path: str,
    headers: Optional[dict[str, str]] = None,
    body: Optional[dict[str, Any]] = None,
) -> tuple[int, dict[str, str], bytes]:
    async def call() -> tuple[int, dict[str, str], bytes]:
        body_bytes = b""
        request_headers = [(b"host", b"testserver")]
        if body is not None:
            body_bytes = json.dumps(body).encode("utf-8")
            request_headers.append((b"content-type", b"application/json"))
        for name, value in (headers or {}).items():
            request_headers.append((name.lower().encode("latin-1"), value.encode("latin-1")))

        scope = {
            "type": "http",
            "asgi": {"version": "3.0"},
            "http_version": "1.1",
            "method": method,
            "scheme": "http",
            "path": path,
            "raw_path": path.encode("ascii"),
            "query_string": b"",
            "headers": request_headers,
            "client": ("testclient", 50000),
            "server": ("testserver", 80),
        }
        sent_request = False
        status = 500
        response_headers: dict[str, str] = {}
        chunks: list[bytes] = []

        async def receive() -> dict[str, Any]:
            nonlocal sent_request
            if sent_request:
                return {"type": "http.disconnect"}
            sent_request = True
            return {"type": "http.request", "body": body_bytes, "more_body": False}

        async def send(message: dict[str, Any]) -> None:
            nonlocal status, response_headers
            if message["type"] == "http.response.start":
                status = int(message["status"])
                response_headers = {
                    key.decode("latin-1").lower(): value.decode("latin-1")
                    for key, value in message.get("headers", [])
                }
            elif message["type"] == "http.response.body":
                chunks.append(message.get("body", b""))

        await app(scope, receive, send)
        return status, response_headers, b"".join(chunks)

    return asyncio.run(call())


def request_json(
    app: Any,
    method: str,
    path: str,
    headers: Optional[dict[str, str]] = None,
    body: Optional[dict[str, Any]] = None,
) -> tuple[int, dict[str, str], dict[str, Any]]:
    status, response_headers, response_body = request(app, method, path, headers, body)
    return status, response_headers, json.loads(response_body.decode("utf-8"))


def test_health(tmp_path: Path) -> None:
    app = make_app(tmp_path)

    status, _, body = request_json(app, "GET", "/health")

    assert status == 200
    assert body["status"] == "ok"


def test_auth_required_when_token_configured(tmp_path: Path) -> None:
    app = make_app(tmp_path, token="secret")

    missing_status, _, _ = request(app, "GET", "/health")
    wrong_status, _, _ = request(app, "GET", "/health", headers={"Authorization": "Bearer wrong"})
    ok_status, _, _ = request(app, "GET", "/health", headers=auth())

    assert missing_status == 401
    assert wrong_status == 401
    assert ok_status == 200


def test_generate_job(tmp_path: Path) -> None:
    app = make_app(tmp_path, token="secret")

    status, _, body = request_json(
        app,
        "POST",
        "/generate-scene",
        headers=auth(),
        body={
            "visual_prompt": "Realistic vertical city scene",
            "negative_prompt": "watermark",
            "duration_seconds": 8,
            "aspect_ratio": "9:16",
            "model_hint": "auto",
            "style_preset": "realistic_editorial",
        },
    )

    assert status == 200
    assert body["id"].startswith("job_")
    assert body["status"] == "pending"
    assert (tmp_path / body["id"] / "job.json").exists()


def test_pending_job_status(tmp_path: Path) -> None:
    app = make_app(tmp_path)
    _, _, generated = request_json(app, "POST", "/generate-scene", body={"visual_prompt": "Prompt"})
    job_id = generated["id"]

    status, _, body = request_json(app, "GET", f"/jobs/{job_id}")

    assert status == 200
    assert body["status"] == "pending"
    assert body["visual_prompt"] == "Prompt"
    assert body["expected_output_path"].endswith(f"{job_id}/output.mp4")


def test_completed_job_status_when_output_file_exists(tmp_path: Path) -> None:
    app = make_app(tmp_path)
    _, _, generated = request_json(app, "POST", "/generate-scene", body={"visual_prompt": "Prompt"})
    job_id = generated["id"]
    output_path = tmp_path / job_id / "output.mp4"
    output_path.write_bytes(b"mp4 bytes")

    status, _, body = request_json(app, "GET", f"/jobs/{job_id}")

    assert status == 200
    assert body["status"] == "completed"


def test_output_download(tmp_path: Path) -> None:
    app = make_app(tmp_path)
    _, _, generated = request_json(app, "POST", "/generate-scene", body={"visual_prompt": "Prompt"})
    job_id = generated["id"]
    output_path = tmp_path / job_id / "output.mp4"
    output_path.write_bytes(b"mp4 bytes")

    status, headers, body = request(app, "GET", f"/jobs/{job_id}/output")

    assert status == 200
    assert headers["content-type"].startswith("video/mp4")
    assert body == b"mp4 bytes"
