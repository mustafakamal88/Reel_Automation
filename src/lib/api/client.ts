/**
 * src/lib/api/client.ts
 *
 * Thin fetch wrapper for the TrendCortex Go backend.
 *
 * Calls use relative URLs when VITE_API_BASE_URL is empty, so the Vite dev
 * proxy routes them to http://localhost:8080. Production builds can set
 * VITE_API_BASE_URL to the deployed API origin.
 *
 * Token boundary: this file NEVER handles or stores OAuth tokens, refresh
 * tokens, client secrets, or API keys. Only status metadata crosses the wire
 * to the browser.
 */

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }

  get isCredentialsMissing(): boolean {
    return this.code === 'credentials_missing' || this.status === 503;
  }

  get isNotImplemented(): boolean {
    return this.status === 501;
  }

  get isBackendOffline(): boolean {
    return this.status === 0;
  }
}

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/+$/, '');

export function apiUrl(path: string): string {
  if (!API_BASE_URL) return path;
  return `${API_BASE_URL}${path.startsWith('/') ? path : `/${path}`}`;
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(apiUrl(path), {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...init?.headers },
      ...init,
    });
  } catch {
    throw new ApiError(0, 'network_error', 'Cannot reach the Go backend. Check VITE_API_BASE_URL or run: cd backend && go run ./cmd/api');
  }

  if (!res.ok) {
    let code = 'unknown';
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json() as { error?: string };
      if (body.error) {
        code = body.error.split(':')[0].trim().toLowerCase().replace(/\s+/g, '_');
        message = body.error;
      }
    } catch { /* non-JSON error body */ }
    throw new ApiError(res.status, code, message);
  }

  return res.json() as Promise<T>;
}

// ── Platform connections ──────────────────────────────────────────────────────

export interface PlatformStatus {
  platform: string;
  name: string;
  status: 'credentials_missing' | 'not_connected' | 'connected' | 'expired';
  scopes: string[];
  can_publish: boolean;
  handle?: string;
  expires_at?: string;
}

export interface PlatformConnectionsResponse {
  platforms: PlatformStatus[];
}

export async function getPlatformConnections(): Promise<PlatformConnectionsResponse> {
  return apiFetch<PlatformConnectionsResponse>('/platforms/connections');
}

// ── OAuth ─────────────────────────────────────────────────────────────────────

export interface OAuthStartResponse {
  authorize_url: string;
}

/**
 * Fetches the authorization URL for a platform OAuth flow.
 * The caller should redirect window.location.href to authorize_url on success.
 * Throws ApiError with isCredentialsMissing=true when .env creds are absent.
 */
export async function getOAuthStartURL(platform: string): Promise<OAuthStartResponse> {
  return apiFetch<OAuthStartResponse>(`/oauth/${platform}/start`);
}

// ── Batch operations ──────────────────────────────────────────────────────────

export interface JobRef {
  job_id: string;
  status: string;
}

export interface BatchZipResponse extends JobRef {}
export interface BatchPublishResponse {
  job_ids: string[];
  queued: number;
}

export async function requestBatchZip(batchID: string): Promise<BatchZipResponse> {
  return apiFetch<BatchZipResponse>(`/batches/${batchID}/zip`, { method: 'POST' });
}

export async function requestBatchPublish(batchID: string): Promise<BatchPublishResponse> {
  return apiFetch<BatchPublishResponse>(`/batches/${batchID}/publish`, { method: 'POST' });
}

// ── Job polling ───────────────────────────────────────────────────────────────

export interface JobStatus {
  job_id: string;
  type: string;
  status: 'queued' | 'running' | 'done' | 'failed';
  error_message?: string;
  completed_at?: string;
}

export async function getJobStatus(jobID: string): Promise<JobStatus> {
  return apiFetch<JobStatus>(`/jobs/${jobID}`);
}
