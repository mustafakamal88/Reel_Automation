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
      const body = await res.json() as { code?: string; error?: string };
      if (body.code) code = body.code;
      if (body.error) {
        if (!body.code) code = body.error.split(':')[0].trim().toLowerCase().replace(/\s+/g, '_');
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

export interface ResearchProviderStatus {
  id: string;
  name: string;
  platform: string;
  status: 'active' | 'not_configured' | 'unavailable';
  message: string;
  scopes?: string[];
  limitations?: string[];
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

export interface ReelContentGenerationRequest {
  trend_candidate_id?: string;
  trend_candidate?: TrendCandidate;
  platform_targets: string[];
  duration_target: string;
  tone_style?: string;
  language: string;
  region: string;
}

export interface ReelContentPackage {
  title: string;
  hook: string;
  script: string;
  caption: string;
  description?: string;
  hashtags: string[];
  platform_posts?: Record<string, string>;
  thumbnail_brief: string;
  instagram_caption: string;
  tiktok_caption: string;
  youtube_title: string;
  youtube_description: string;
  facebook_caption: string;
  x_caption: string;
  safety_grounding_notes: string[];
  grounding?: string;
  source_type?: string;
  source_url?: string;
  created_at?: string;
  inferred_keywords?: string[];
  inferred_niche?: string;
  inferred_angle?: string;
  provider_metadata: {
    provider: string;
    model: string;
    source_candidate_id: string;
    source: string;
    source_url?: string;
    platform_targets: string[];
    duration_target: string;
    generated_at: string;
  };
}

export interface ReelContentGenerationResponse {
  package: ReelContentPackage;
}

export type ResearchScriptSourceType = 'google_trend' | 'youtube_video_analysis' | 'youtube_channel_analysis' | 'niche_idea';

export interface ResearchScriptGenerationRequest {
  source_type: ResearchScriptSourceType;
  source_id?: string;
  source_url?: string;
  topic?: string;
  title?: string;
  summary?: string;
  keywords?: string[];
  inferred_niche?: string;
  inferred_angle?: string;
  performance_signals?: Record<string, unknown>;
  suggested_angle?: string;
  target_platforms?: string[];
  content_style?: string;
  duration_seconds?: number;
  evidence?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  limitations?: string[];
  language?: string;
  region?: string;
}

export interface ResearchScriptGenerationResponse {
  package: ReelContentPackage;
}

export interface NicheOpportunityRequest {
  seed_keyword: string;
  platform: string;
  country: string;
  language: string;
  audience: string;
  content_style: string;
  monetization_goal: string;
  creator_skill_level: string;
  production_difficulty_preference: string;
}

export interface NicheOpportunity {
  niche_name: string;
  platform: string;
  country: string;
  language: string;
  audience: string;
  content_style: string;
  demand_score: number;
  monetization_score: number;
  monetization_confidence?: string;
  competition_score: number;
  creator_fit_score?: number;
  success_probability_score: number;
  success_probability: 'low' | 'possible' | 'promising' | 'strong' | string;
  opportunity_score: number;
  confidence: number;
  evidence_sources?: string[];
  supporting_keywords?: string[];
  related_channels?: string[];
  related_videos?: {
    video_id: string;
    title: string;
    channel_id?: string;
    channel_title?: string;
    published_at: string;
    views?: number;
    likes?: number;
    comments?: number;
  }[];
  estimated_monetization_level: 'low' | 'medium' | 'high' | 'very_high' | string;
  monetization_reason: string;
  competition_level: 'low' | 'medium' | 'high' | 'saturated' | string;
  competition_reason: string;
  demand_reason: string;
  success_reason: string;
  risks?: string[];
  first_10_video_ideas?: string[];
  suggested_keywords?: string[];
  suggested_titles?: string[];
  suggested_clip_angles?: string[];
  limitations?: string[];
}

export interface NicheOpportunityResponse {
  status: 'ok' | 'not_configured' | 'invalid_input' | 'provider_error' | 'insufficient_data' | string;
  message: string;
  provider_status?: ResearchProviderStatus[];
  opportunities?: NicheOpportunity[];
  limitations?: string[];
}

export interface CreatorNicheProfile {
  professional_skills: string;
  hobbies: string;
  lived_experiences: string;
  teaching_subjects: string;
  three_years_ago_advice: string;
  target_audience: string;
  target_country: string;
  target_language: string;
  creator_presence: string;
  content_formats: string[];
  optional_broad_topic: string;
  weekly_production_capacity: string;
}

export interface NicheResearchRequest {
  profile: CreatorNicheProfile;
  refresh?: boolean;
}

export interface NicheReport {
  id: string;
  status: 'ok' | 'openai_unavailable' | 'invalid_model_output' | 'not_configured' | 'invalid_input' | 'insufficient_evidence' | 'no_matching_content' | 'quota_temporarily_unavailable' | 'credentials_invalid' | 'provider_temporarily_unavailable' | 'validation_timeout' | 'research_failed' | string;
  message: string;
  generated_at?: string;
  evidence_freshness?: string;
  analysis_mode?: string;
  provider_status?: ResearchProviderStatus[];
  creator_profile_summary?: string;
  primary_recommendation?: NicheCandidate;
  alternative_candidates?: NicheCandidate[];
  methodology?: string[];
  profile: CreatorNicheProfile;
  candidates: NicheCandidate[];
  cache: { hit: boolean; cache_hit?: boolean; key?: string; stored_at?: string; ttl: string; evidence_fetched_at?: string; evidence_age?: string; freshness?: string };
  limitations?: string[];
  created_at: string;
}

export interface NicheCandidate {
  id: string;
  name?: string;
  concise_positioning?: string;
  category?: string;
  subcategory?: string;
  target_audience?: string;
  audience_problems?: string[];
  creator_advantages?: string[];
  unique_angle?: string;
  overall_score?: number;
  confidence?: string;
  dimensions?: NicheScoreDimensions;
  content_pillars?: ContentPillar[];
  topic_clusters?: TopicCluster[];
  recommended_titles?: VideoTopic[];
  opportunity_gaps?: string[];
  evidence_summary?: string;
  market_evidence?: MarketEvidence;
  search_queries_used?: string[];
  runway?: ContentRunway;
  level_1: string;
  level_2: string;
  level_3: string;
  niche_name: string;
  core_phrase: string;
  target_viewer: string;
  viewer_problem: string;
  creator_advantage: string;
  recommended_content_format: string;
  validation: NicheValidation;
  outliers?: OutlierEvidence[];
  supply_gaps?: SupplyGap[];
  video_topics?: VideoTopic[];
  topic_pillars?: ContentPillar[];
  first_10_titles?: string[];
  sustainability: SustainabilityEvidence;
  scores: NicheScores;
  risks?: string[];
  recommended_first_action: string;
}

export interface NicheScoreDimensions {
  creator_fit: ScoreExplanation;
  audience_demand: ScoreExplanation;
  competition_opportunity: ScoreExplanation;
  sustainability: ScoreExplanation;
  differentiation: ScoreExplanation;
}

export interface TopicCluster {
  name: string;
  description: string;
  titles?: string[];
}

export interface MarketEvidence {
  status: 'live_validated' | 'cache_validated' | 'trend_supported' | 'ai_strategic_analysis' | 'limited_evidence' | string;
  source_types?: string[];
  sample_size: number;
  recent_activity: string;
  median_views?: number | null;
  engagement?: number | null;
  collected_at?: string | null;
  limitations?: string[];
}

export interface ContentRunway {
  viable_topic_count: number;
  estimated_weeks: number;
  weekly_capacity: number;
  estimated_content_runway: string;
}

export interface NicheValidation {
  search_phrases?: string[];
  recent_publication_volume: number;
  sampled_video_count: number;
  total_sampled_views: number;
  median_sampled_views: number;
  engagement_rate: number;
  newest_activity?: string;
  rising_topic_overlap: boolean;
  market_evidence_summary: string;
  competition_level: string;
  validation_budget_used: number;
  evidence_confidence: string;
}

export interface OutlierEvidence {
  title: string;
  thumbnail_url?: string;
  canonical_url: string;
  channel_name: string;
  publication_age: string;
  public_views: number;
  outlier_reason: string;
  outlier_strength: number;
}

export interface SupplyGap {
  statement: string;
  evidence?: string[];
  confidence: string;
}

export interface MonetizationEstimate {
  mode: string;
  commercial_potential: string;
  score: number;
  rpm_estimate_available: boolean;
  currency?: string;
  rpm_low?: number;
  rpm_midpoint?: number;
  rpm_high?: number;
  estimated_earnings?: EarningsProjection[];
  confidence: string;
  calibration_type: string;
  calibration_age?: string;
  calculation_assumptions?: string[];
  unavailable_reason?: string;
  format: string;
  target_market: string;
  advertiser_demand_signals?: string[];
  estimate_disclaimer: string;
}

export interface EarningsProjection {
  views: number;
  low: number;
  midpoint: number;
  high: number;
  formula: string;
}

export interface VideoTopic {
  title: string;
  pillar: string;
  intent: string;
  difficulty: string;
  source: string;
  evidence_status?: 'evidence-backed' | 'related opportunity' | 'unvalidated idea' | string;
}

export interface ContentPillar {
  name: string;
  description?: string;
  percentage?: number;
  topic_count: number;
  example_titles?: string[];
}

export interface SustainabilityEvidence {
  viable_topic_count: number;
  content_pillar_count: number;
  topic_repetition_risk: string;
  estimated_content_runway: string;
  score: number;
  warning?: string;
  deduped_removed: number;
}

export interface NicheScores {
  personal_fit: ScoreExplanation;
  demand: ScoreExplanation;
  opportunity_gap: ScoreExplanation;
  sustainability: ScoreExplanation;
  overall: ScoreExplanation;
  confidence: ScoreExplanation;
}

export interface ScoreExplanation {
  score: number;
  label: string;
  rating_band?: string;
  explanation: string;
  factors?: string[];
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

export interface TrendIntelligenceResult {
  id: string;
  keyword: string;
  normalized_keyword: string;
  display_title: string;
  summary?: string;
  country_code: string;
  language_code?: string;
  region?: string;
  category?: string;
  related_keywords?: string[];
  search_volume_text?: string;
  published_at?: string;
  discovered_at: string;
  trend_age?: string;
  trend_velocity?: number;
  video_count_sampled?: number;
  total_sampled_views?: number;
  median_sampled_views?: number;
  average_sampled_views?: number;
  total_sampled_likes?: number;
  total_sampled_comments?: number;
  newest_relevant_video_at?: string;
  strongest_relevant_video_url?: string;
  strongest_thumbnail_url?: string;
  confidence_score: number;
  opportunity_score: number;
  momentum_score: number;
  demand_score: number;
  competition_score: number;
  scoring_reasons: string[];
  sampled_video_activity?: string;
  supporting_content_available: boolean;
  fetched_at: string;
}

export interface TrendResolvedLocation {
  input: string;
  country: string;
  region?: string;
  city?: string;
  latitude?: number;
  longitude?: number;
  status: string;
  message?: string;
}

export interface TrendIntelligenceResponse {
  status: string;
  message?: string;
  mode: 'discover' | 'keyword';
  query?: string;
  country: string;
  country_name: string;
  language: string;
  language_name: string;
  time_window: string;
  category: string;
  resolved_location?: TrendResolvedLocation;
  local_videos_only: boolean;
  local_radius_km?: number;
  results: TrendIntelligenceResult[];
  fetched_at: string;
  cache: { hit: boolean; stored_at?: string; ttl: string };
}

export interface TrendFilterMetadata {
  countries: { label: string; value: string }[];
  languages: { label: string; value: string }[];
  time_windows: { label: string; value: string }[];
  categories: { label: string; value: string }[];
  video_durations: { label: string; value: string }[];
  local_radii_km: number[];
  sort_orders: { label: string; value: string }[];
  default_country: string;
  default_language: string;
}

export interface TrendSearchParams {
  q?: string;
  country?: string;
  language?: string;
  window?: string;
  category?: string;
  niche?: string;
  postcode?: string;
  local_videos_only?: boolean;
  radius_km?: number;
  include_words?: string;
  exclude_words?: string;
  exact_phrase?: string;
  video_duration?: string;
  min_views?: string;
  max_competition?: string;
  min_opportunity?: string;
  sort?: string;
  refresh?: boolean;
  limit?: number;
}

const trendSearchInflight = new Map<string, {
  controller: AbortController;
  promise: Promise<TrendIntelligenceResponse>;
  consumers: number;
  abortTimer: number | undefined;
}>();

export async function searchTrendIntelligence(params: TrendSearchParams = {}, init?: RequestInit): Promise<TrendIntelligenceResponse> {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '' && value !== false) qs.set(key, String(value));
  });
  const suffix = qs.toString() ? `?${qs.toString()}` : '';
  const path = `/api/trends/search${suffix}`;
  const signal = init?.signal;
  let entry = trendSearchInflight.get(path);

  if (!entry) {
    const controller = new AbortController();
    const headers = init?.headers;
    const promise = apiFetch<TrendIntelligenceResponse>(path, { headers, signal: controller.signal })
      .finally(() => {
        trendSearchInflight.delete(path);
      });
    entry = { controller, promise, consumers: 0, abortTimer: undefined };
    trendSearchInflight.set(path, entry);
  } else if (entry.abortTimer !== undefined) {
    window.clearTimeout(entry.abortTimer);
    entry.abortTimer = undefined;
  }

  if (!signal) return entry.promise;

  entry.consumers += 1;
  const release = () => {
    if (!entry) return;
    entry.consumers = Math.max(0, entry.consumers - 1);
    if (entry.consumers === 0) {
      entry.abortTimer = window.setTimeout(() => {
        if (entry && entry.consumers === 0) entry.controller.abort();
      }, 25);
    }
  };

  if (signal.aborted) {
    release();
    throw new DOMException('The operation was aborted.', 'AbortError');
  }

  signal.addEventListener('abort', release, { once: true });
  try {
    return await entry.promise;
  } finally {
    signal.removeEventListener('abort', release);
    if (!signal.aborted) release();
  }
}

export async function getTrendFilters(): Promise<TrendFilterMetadata> {
  return apiFetch('/api/trends/filters');
}

export async function resolveTrendLocation(location: string): Promise<TrendResolvedLocation> {
  return apiFetch('/api/location/resolve', { method: 'POST', body: JSON.stringify({ location }) });
}

export async function getResearchProviderStatus(): Promise<{ providers: ResearchProviderStatus[] }> {
  return apiFetch('/api/research/providers/status');
}

export async function generateReelScript(body: ReelContentGenerationRequest): Promise<ReelContentGenerationResponse> {
  return apiFetch('/api/reels/generate-script', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function generateResearchScript(body: ResearchScriptGenerationRequest): Promise<ResearchScriptGenerationResponse> {
  return apiFetch('/api/research/script', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function analyzeNicheOpportunities(body: NicheOpportunityRequest): Promise<NicheOpportunityResponse> {
  return apiFetch('/api/research/niche/opportunities', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function researchNiches(body: NicheResearchRequest): Promise<NicheReport> {
  return apiFetch('/api/niches/research', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function analyseNicheGaps(id: string): Promise<NicheReport> {
  return apiFetch(`/api/niches/${encodeURIComponent(id)}/analyse-gaps`, {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

export interface YouTubeVideoAnalysisResponse {
  status: 'not_configured' | 'ok' | 'invalid_input' | 'provider_error' | 'no_data';
  message: string;
  video_url?: string;
  video_id?: string;
  title?: string;
  channel_title?: string;
  channel_id?: string;
  published_at?: string;
  description?: string;
  tags?: string[];
  category?: string;
  duration?: string;
  views?: number;
  likes?: number;
  comments?: number;
  public_topic_details?: string[];
  extracted_keywords?: string[];
  inferred_niche?: string;
  inferred_content_angle?: string;
  hook_analysis?: string;
  title_structure_analysis?: string;
  description_hashtag_analysis?: string;
  performance_signals?: Record<string, unknown>;
  video_snapshot?: Record<string, unknown>;
  keyword_intelligence?: KeywordIntelligence;
  hook_intelligence?: HookIntelligence;
  niche_analysis?: NicheAnalysis;
  creator_opportunities?: CreatorOpportunities;
  suggested_remake_angles?: string[];
  limitations?: string[];
  metadata?: ResearchResultMetadata;
}

export interface KeywordIntelligence {
  primary_keywords?: string[];
  secondary_keywords?: string[];
  long_tail_phrases?: string[];
  hashtags?: string[];
  rejected_noise_terms?: string[];
  inferred_search_intent?: string;
  metadata_strength_score?: number;
}

export interface HookIntelligence {
  hook_type?: string;
  title_length?: number;
  title_pattern?: string;
  emotional_triggers?: string[];
  clarity_score?: number;
  curiosity_score?: number;
  remake_potential_score?: number;
}

export interface NicheAnalysis {
  primary_niche?: string;
  sub_niche?: string;
  audience_type?: string;
  content_format?: string;
  confidence?: number;
  evidence_terms?: string[];
  target_audience?: string;
  inferred_content_angle?: string;
}

export interface CreatorOpportunities {
  suggested_remake_angles?: string[];
  title_ideas?: string[];
  short_form_clip_ideas?: string[];
  script_prompts?: string[];
  content_gaps?: string[];
  underused_topics?: string[];
  localization_options?: string[];
}

export interface KeywordCluster {
  name: string;
  terms?: string[];
  evidence?: string[];
}

export interface YouTubeChannelAnalysisResponse {
  status: 'not_configured' | 'ok' | 'invalid_input' | 'provider_error' | 'no_data';
  message: string;
  channel_url?: string;
  channel_id?: string;
  channel_title?: string;
  description?: string;
  subscribers?: number;
  views?: number;
  video_count?: number;
  country?: string;
  public_topic_details?: string[];
  recent_videos?: {
    video_id: string;
    title: string;
    published_at: string;
    views?: number;
    likes?: number;
    comments?: number;
  }[];
  top_videos_summary?: {
    video_id: string;
    title: string;
    published_at: string;
    views?: number;
    likes?: number;
    comments?: number;
  }[];
  channel_snapshot?: Record<string, unknown>;
  channel_niche?: string;
  niche_analysis?: NicheAnalysis;
  content_pillars?: string[];
  keyword_intelligence?: KeywordIntelligence;
  keyword_clusters?: KeywordCluster[];
  format_patterns?: string[];
  title_patterns?: string[];
  performance_distribution?: Record<string, unknown>;
  upload_frequency?: string;
  top_video_topics?: string[];
  repeated_keywords?: string[];
  view_distribution?: Record<string, unknown>;
  subscriber_view_ratio?: number;
  likely_strategy?: string;
  opportunities?: string[];
  suggested_content_ideas?: string[];
  suggested_short_clip_ideas?: string[];
  limitations?: string[];
  metadata?: ResearchResultMetadata;
}

export interface ResearchResultMetadata {
  source_provider: string;
  source_url?: string;
  evidence_url?: string;
  region?: string;
  language?: string;
  country?: string;
  platform?: string;
  topic?: string;
  keyword?: string;
  score?: number;
  velocity?: number;
  volume?: number;
  views?: number;
  confidence: number;
  limitations: string[];
  fetched_at: string;
  score_reason?: string;
}

export async function analyzeYouTubeVideo(videoURL: string): Promise<YouTubeVideoAnalysisResponse> {
  return apiFetch('/api/research/youtube/video', {
    method: 'POST',
    body: JSON.stringify({ video_url: videoURL }),
  });
}

export async function analyzeYouTubeChannel(channelURL: string): Promise<YouTubeChannelAnalysisResponse> {
  return apiFetch('/api/research/youtube/channel', {
    method: 'POST',
    body: JSON.stringify({ channel_url: channelURL }),
  });
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

export interface DailyPackageReelStatus {
  rank: number;
  candidate_id: string;
  title: string;
  source: string;
  render_status: string;
  render_notes?: string;
  render_error?: string;
  has_video: boolean;
  video_file?: string;
  thumbnail_file?: string;
  duration_seconds?: number;
  resolution?: string;
  renderer_version?: string;
}

export interface DailyPackageResponse {
  status: 'ready' | 'ready_with_render_failures';
  message: string;
  date: string;
  zip_filename: string;
  download_url: string;
  included_files: string[];
  reels: DailyPackageReelStatus[];
}

export async function createDailyPackage(body: {
  date?: string;
  region?: string;
  language?: string;
  platform_targets?: string[];
  duration_target?: string;
  tone_style?: string;
} = {}): Promise<DailyPackageResponse> {
  return apiFetch('/api/daily-package', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function downloadDailyPackageZip(downloadURL: string, filename: string): Promise<void> {
  return downloadZipPath(downloadURL, filename);
}

export interface DailyPackageRenderJob {
  id: string;
  reel_id: string;
  status: 'rendering' | 'completed' | 'failed';
  render_status: 'rendered' | 'failed' | 'not_attempted' | 'rendering' | string;
  render_error?: string;
  message: string;
  total_reels: number;
  current_reel?: string;
  completed_count: number;
  failed_count: number;
  reels?: DailyPackageReelStatus[];
  started_at: string;
  completed_at?: string;
  package?: DailyPackageResponse;
}

export async function renderDailyPackageReel(reelID: string): Promise<DailyPackageRenderJob> {
  return apiFetch(`/api/daily-package/reels/${encodeURIComponent(reelID)}/render`, { method: 'POST' });
}

export async function renderAllDailyPackageReels(): Promise<DailyPackageRenderJob> {
  return apiFetch('/api/daily-package/render-all', { method: 'POST' });
}

export async function getDailyPackageRenderJob(jobID: string): Promise<DailyPackageRenderJob> {
  return apiFetch(`/api/daily-package/render-jobs/${encodeURIComponent(jobID)}`);
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

export type ClipSourceModel =
  | 'user_upload'
  | 'own_channel_source'
  | 'creative_commons'
  | 'public_domain'
  | 'licensed_source'
  | 'external_url_pending_rights_confirmation';

export type ClipLayoutMode = 'fit_with_bars' | 'fill_crop' | 'blurred_background';
export type ClipCTASize = 'small' | 'medium' | 'large';

export interface ClipRightsMetadata {
  source_url?: string;
  source_title?: string;
  source_creator?: string;
  source_license?: string;
  attribution_text?: string;
  user_confirmed_rights: boolean;
  copyright_overlay_text?: string;
  platform_source?: string;
}

export interface ClipBrandingSettings {
  top_banner_text?: string;
  bottom_banner_text?: string;
  logo_path?: string;
  watermark_text?: string;
  cta_text?: string;
  font_style_preset?: string;
  top_banner_color?: string;
  bottom_banner_color?: string;
  cta_size?: ClipCTASize;
}

export interface ClipStudioRenderRequest {
  source_model: ClipSourceModel;
  source_id?: string;
  source_video_path: string;
  rights: ClipRightsMetadata;
  branding: ClipBrandingSettings;
  manual_range: {
    start_seconds: number;
    end_seconds: number;
  };
  captions?: string;
  caption_text?: string;
  include_captions: boolean;
  layout_mode?: ClipLayoutMode;
  ai_highlights: {
    transcription_status: 'not_run';
    suggested_clips_status: 'not_run';
    hook_score_status: 'not_run';
  };
}

export interface ClipStudioRenderResponse {
  success: boolean;
  render_status: string;
  notes?: string;
  clip_id: string;
  zip_filename?: string;
  download_url?: string;
  included_files: string[];
  video_path?: string;
  thumbnail_path?: string;
}

export async function renderClipStudio(body: ClipStudioRenderRequest): Promise<ClipStudioRenderResponse> {
  return apiFetch('/api/clip-studio/render', { method: 'POST', body: JSON.stringify(body) });
}

export interface ClipStudioSourceMetadata {
  source_id: string;
  kind: 'upload' | 'url';
  original_name?: string;
  url?: string;
  file_path?: string;
  content_type?: string;
  size_bytes?: number;
  source_model?: ClipSourceModel;
  rights: ClipRightsMetadata;
  status: string;
  message?: string;
  created_at: string;
  direct_video: boolean;
  supported_type: boolean;
}

export interface ClipStudioSourceResponse {
  source_id: string;
  status: string;
  message?: string;
  metadata: ClipStudioSourceMetadata;
  can_render: boolean;
  direct_video: boolean;
  download_ready: boolean;
}

export interface ClipStudioGenerateRequest {
  source_id?: string;
  source_url?: string;
  prompt: string;
  clip_length: 'auto' | '15s' | '30s' | '60s' | '3min';
  clip_count: 1 | 3 | 6;
  branding: ClipBrandingSettings;
  caption_text?: string;
  layout_mode?: ClipLayoutMode;
  rights: ClipRightsMetadata;
  rights_confirmed: boolean;
  advanced: {
    source_model?: ClipSourceModel;
    source_title?: string;
    source_creator?: string;
    source_license?: string;
    attribution_text?: string;
    copyright_overlay_text?: string;
    platform_source?: string;
  };
}

export interface ClipStudioGeneratedJob {
  clip_id: string;
  render_status: string;
  notes?: string;
  manual_range: {
    start_seconds: number;
    end_seconds: number;
  };
  video_path?: string;
  thumbnail_path?: string;
}

export interface ClipStudioGenerateResponse {
  success: boolean;
  render_status: string;
  notes?: string;
  source_id?: string;
  highlight_detection: 'not_run';
  generated_clip_jobs: ClipStudioGeneratedJob[];
  zip_filename?: string;
  download_url?: string;
  included_files: string[];
}

export async function uploadClipStudioSource(file: File): Promise<ClipStudioSourceResponse> {
  const form = new FormData();
  form.append('video', file);
  let res: Response;
  try {
    res = await fetch(apiUrl('/api/clip-studio/upload'), {
      method: 'POST',
      credentials: 'include',
      body: form,
    });
  } catch {
    throw new ApiError(0, 'network_error', 'Cannot reach the Go backend. Check VITE_API_BASE_URL or run: cd backend && go run ./cmd/api');
  }
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json() as { error?: string };
      if (body.error) message = body.error;
    } catch { /* non-JSON error body */ }
    throw new ApiError(res.status, 'upload_failed', message);
  }
  return res.json() as Promise<ClipStudioSourceResponse>;
}

export async function createClipStudioSource(body: {
  source_url: string;
  rights_confirmed: boolean;
  rights: ClipRightsMetadata;
}): Promise<ClipStudioSourceResponse> {
  return apiFetch('/api/clip-studio/source', { method: 'POST', body: JSON.stringify(body) });
}

export async function importClipStudioURL(body: {
  source_url: string;
  rights_confirmed: boolean;
  rights: ClipRightsMetadata;
}): Promise<ClipStudioSourceResponse> {
  return apiFetch('/api/clip-studio/import-url', { method: 'POST', body: JSON.stringify(body) });
}

export async function generateClipStudio(body: ClipStudioGenerateRequest): Promise<ClipStudioGenerateResponse> {
  return apiFetch('/api/clip-studio/generate', { method: 'POST', body: JSON.stringify(body) });
}

export async function downloadClipStudioZip(downloadURL: string, filename: string): Promise<void> {
  return downloadPath(downloadURL, filename);
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
  return downloadPath(path, filename);
}

async function downloadPath(path: string, filename: string): Promise<void> {
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
