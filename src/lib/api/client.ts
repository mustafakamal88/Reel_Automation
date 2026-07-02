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

// ── Health ───────────────────────────────────────────────────────────────────

export interface HealthResponse {
  ok: boolean;
  service: string;
}

export async function getHealth(): Promise<HealthResponse> {
  return apiFetch<HealthResponse>('/health');
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

// ── Phase 4A: trend discovery → scoring → daily batch → reel plan pipeline ────

export interface TrendSource {
  id: string;
  workspace_id: string;
  name: string;
  source_type: string;
  status: string;
  confidence: number;
  created_at: string;
  updated_at: string;
}

export interface TrendItem {
  id: string;
  workspace_id: string;
  trend_source_id: string;
  topic: string;
  description: string;
  platform_hint: string;
  velocity: number;
  status: 'new' | 'scored' | 'batched' | 'rejected';
  discovered_at: string;
  created_at: string;
  updated_at: string;
}

export interface TrendCandidate {
  id: string;
  source: string;
  region: string;
  language: string;
  keyword: string;
  title: string;
  score: number;
  velocity?: number;
  discovered_at: string;
  source_url?: string;
  evidence?: string;
  status: string;
}

export interface TrendDiscoveryResponse {
  provider: string;
  provider_url?: string;
  provider_status: 'provider_not_configured' | 'ok' | 'no_data' | 'provider_error';
  message?: string;
  region: string;
  language: string;
  candidates: TrendCandidate[];
  discovered_at: string;
}

export interface TopicScore {
  id: string;
  workspace_id: string;
  trend_item_id: string;
  total_score: number;
  velocity_score: number;
  source_confidence_score: number;
  platform_fit_score: number;
  safety_score: number;
  watch_time_score: number;
  competition_score: number;
  reason: string;
  breakdown: Record<string, number>;
  created_at: string;
}

export interface DailyBatchV2 {
  id: string;
  workspace_id: string;
  batch_date: string;
  status: 'planned' | 'ready';
  reel_count: number;
  created_at: string;
  updated_at: string;
}

export type ReelExportStatus = 'artifact_missing' | 'video_artifact_missing' | 'thumbnail_artifact_missing' | 'ready';

export interface ReelPlan {
  id: string;
  workspace_id: string;
  daily_batch_id: string;
  trend_item_id: string;
  topic_score_id: string;
  rank: number;
  platform: string;
  title_idea: string;
  script_outline: string;
  description_draft: string;
  hashtags_draft: string;
  thumbnail_idea: string;
  status: 'draft' | 'video_requested';

  video_artifact_path: string | null;
  video_format: string;
  video_width: number | null;
  video_height: number | null;
  video_duration_seconds: number | null;
  video_codec: string;
  audio_codec: string;

  thumbnail_artifact_path: string | null;
  thumbnail_format: string;
  thumbnail_width: number | null;
  thumbnail_height: number | null;

  export_status: ReelExportStatus;
  export_error: string | null;

  created_at: string;
  updated_at: string;
}

export interface VideoJob {
  id: string;
  workspace_id: string;
  reel_plan_id: string;
  status:
    | 'pending_provider_connection'
    | 'draft_ready'
    | 'provider_not_connected'
    | 'renderer_not_available'
    | 'audio_artifact_missing'
    | 'thumbnail_artifact_missing'
    | 'rendering'
    | 'completed'
    | 'failed';
  provider: string;
  notes: string;
  created_at: string;
  updated_at: string;
}

export type ExportJobStatus =
  | 'video_artifact_missing'
  | 'thumbnail_artifact_missing'
  | 'media_artifacts_missing'
  | 'failed'
  | 'completed'
  | 'zip_generation_not_implemented'; // legacy status from before real ZIP export — no longer produced

export interface ExportJob {
  id: string;
  workspace_id: string;
  daily_batch_id: string;
  status: ExportJobStatus;
  zip_path: string | null;
  error_message: string | null;
  created_at: string;
  completed_at: string | null;
}

export interface PublishJobV2 {
  id: string;
  workspace_id: string;
  reel_plan_id: string;
  platform: string;
  status: 'platform_not_connected' | 'queued' | 'running' | 'done' | 'failed' | 'skipped';
  retry_count: number;
  error_message: string | null;
  platform_post_id: string | null;
  scheduled_for: string | null;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

export async function getTrendSources(): Promise<{ trend_sources: TrendSource[] }> {
  return apiFetch('/api/trend-sources');
}

export async function createTrendSource(body: { name: string; source_type: string; confidence?: number }): Promise<TrendSource> {
  return apiFetch('/api/trend-sources', { method: 'POST', body: JSON.stringify(body) });
}

export interface DiscoverTrendsResponse {
  status: 'created' | 'provider_not_connected';
  message?: string;
  count?: number;
  items?: TrendItem[];
}

export async function discoverTrends(trendSourceID: string): Promise<DiscoverTrendsResponse> {
  return apiFetch('/api/trends/discover', {
    method: 'POST',
    body: JSON.stringify({ trend_source_id: trendSourceID }),
  });
}

export async function discoverTrendCandidates(params: { region?: string; language?: string; limit?: number } = {}): Promise<TrendDiscoveryResponse> {
  const qs = new URLSearchParams();
  if (params.region) qs.set('region', params.region);
  if (params.language) qs.set('language', params.language);
  if (params.limit) qs.set('limit', String(params.limit));
  const suffix = qs.toString() ? `?${qs.toString()}` : '';
  return apiFetch(`/api/trends/discover${suffix}`);
}

export async function getTrends(status?: string): Promise<{ trend_items: TrendItem[] }> {
  const qs = status ? `?status=${encodeURIComponent(status)}` : '';
  return apiFetch(`/api/trends${qs}`);
}

export async function scoreTopics(trendItemIDs?: string[]): Promise<{ scored: number; topic_scores: TopicScore[] }> {
  return apiFetch('/api/topics/score', {
    method: 'POST',
    body: JSON.stringify({ trend_item_ids: trendItemIDs ?? [] }),
  });
}

export async function getTopicScores(): Promise<{ topic_scores: TopicScore[] }> {
  return apiFetch('/api/topics/scores');
}

export interface CreateDailyBatchResponse {
  batch: DailyBatchV2;
  reel_plans: ReelPlan[];
  already_existed: boolean;
}

export async function createDailyBatch(date?: string): Promise<CreateDailyBatchResponse> {
  return apiFetch('/api/batches/daily', {
    method: 'POST',
    body: JSON.stringify(date ? { date } : {}),
  });
}

export async function getDailyBatches(): Promise<{ daily_batches: DailyBatchV2[] }> {
  return apiFetch('/api/batches');
}

export async function getBatchReels(batchID: string): Promise<{ reel_plans: ReelPlan[] }> {
  return apiFetch(`/api/batches/${batchID}/reels`);
}

export async function prepareVideoJob(reelID: string): Promise<{ video_job: VideoJob; already_existed: boolean }> {
  return apiFetch(`/api/reels/${reelID}/prepare-video-job`, { method: 'POST' });
}

export async function getVideoJobs(): Promise<{ video_jobs: VideoJob[] }> {
  return apiFetch('/api/video-jobs');
}

export async function renderReel(reelID: string): Promise<{ render_job: VideoJob; status: string; notes: string }> {
  return apiFetch(`/api/reels/${reelID}/render`, { method: 'POST' });
}

export async function getRenderJobs(): Promise<{ render_jobs: VideoJob[] }> {
  return apiFetch('/api/render-jobs');
}

export interface RenderExportTestRequest {
  topic?: string;
  title?: string;
  script?: string;
  caption?: string;
  target_platforms?: string[];
  style?: string;
  format?: string;
  number_of_reels?: number;
  allow_local_fallback?: boolean;
}

export interface RenderExportTestResponse {
  success: boolean;
  render_status: string;
  export_status: string;
  zip_filename: string;
  zip_path: string;
  download_url: string;
  included_files: string[];
  provider: string;
  fallback_reason?: string;
}

export async function runRenderExportTest(body: RenderExportTestRequest): Promise<RenderExportTestResponse> {
  return apiFetch('/api/reels/export-test', { method: 'POST', body: JSON.stringify(body) });
}

export interface CreateExportJobResponse {
  export_job: ExportJob;
  missing_video_reels: number[];
  missing_thumbnail_reels: number[];
}

export async function createBatchExport(batchID: string): Promise<CreateExportJobResponse> {
  return apiFetch(`/api/batches/${batchID}/export`, { method: 'POST' });
}

export async function getExportJobs(): Promise<{ export_jobs: ExportJob[] }> {
  return apiFetch('/api/export-jobs');
}

/**
 * Downloads a completed export job's ZIP and triggers a browser save.
 * Fetches with credentials (rather than a plain <a href>) so this works
 * against a cross-origin API that requires the session cookie. Throws
 * ApiError if the job isn't completed or the ZIP is missing on disk.
 */
export async function downloadExportZip(jobID: string, filename: string): Promise<void> {
  return downloadZipPath(`/api/export-jobs/${jobID}/download`, filename);
}

export async function downloadRenderExportTestZip(downloadURL: string, filename: string): Promise<void> {
  return downloadZipPath(downloadURL, filename);
}

async function downloadZipPath(path: string, filename: string): Promise<void> {
  let res: Response;
  try {
    res = await fetch(apiUrl(path), { credentials: 'include' });
  } catch {
    throw new ApiError(0, 'network_error', 'Cannot reach the Go backend. Check VITE_API_BASE_URL or run: cd backend && go run ./cmd/api');
  }
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json() as { error?: string };
      if (body.error) message = body.error;
    } catch { /* non-JSON error body */ }
    throw new ApiError(res.status, 'download_failed', message);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export async function publishReel(reelID: string): Promise<PublishJobV2> {
  return apiFetch(`/api/reels/${reelID}/publish`, { method: 'POST' });
}

export async function getPublishJobs(): Promise<{ publish_jobs: PublishJobV2[] }> {
  return apiFetch('/api/publish-jobs');
}
