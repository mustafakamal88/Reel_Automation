export type Platform = 'yt' | 'ig' | 'tt' | 'fb' | 'x' | 'th' | 'gt';

export type RiskLevel = 'Low' | 'Med' | 'High';

export type ApprovalStatus = 'pending' | 'approved' | 'rejected';

export type View =
  | 'signals'
  | 'scoring'
  | 'topics'
  | 'competitors'
  | 'approvals'
  | 'performance'
  | 'pipeline'
  | 'workflow'
  | 'settings';

// ─── Daily Workflow types ─────────────────────────────────────

export type WorkflowStatus = 'draft' | 'needs-review' | 'approved' | 'rejected' | 'scheduled';
export type PipelineHandoffStatus = 'not-started' | 'queued' | 'in-progress' | 'done';

export interface SelectionReason {
  factor: string;
  detail: string;
}

export interface ScheduleEntry {
  platformKey: string;
  platformLabel: string;
  time: string;
}

export interface DailyReel {
  topicId: string;
  rank: number;
  title: string;
  trendScore: number;
  riskScore: number;
  watchTimePotential: number;
  platformFit: number;
  compositeScore: number;
  selectionReasons: SelectionReason[];
  platforms: Platform[];
  pipelineStatus: PipelineHandoffStatus;
  workflowStatus: WorkflowStatus;
  scheduleSlots: ScheduleEntry[];
}

export interface RejectedReel {
  topicId: string;
  title: string;
  trendScore: number;
  riskScore: number;
  compositeScore: number;
  rejectionReasons: string[];
}

export interface DailyWorkflow {
  date: string;
  selectedReels: DailyReel[];
  rejectedReels: RejectedReel[];
  generatedAt: string;
}

export interface PlatformMeta {
  name: string;
  short: string;
  color: string;
  bg: string;
}

export interface Signal {
  id: string;
  src: Platform;
  type: string;
  label: string;
  metric: string;
  growth: string;
  sparkData: number[];
  timestamp: string;
}

export interface Keyword {
  id: string;
  kw: string;
  niche: string;
  trendGrowth: number;
  crossPlatform: number;
  viewsPerHour: number;
  engagement: number;
  searchInterest: number;
  nicheMatch: number;
  lowCompBonus: number;
  saturationPenalty: number;
  riskScore: number;
  finalScore: number;
}

export interface ScriptBeat {
  timeRange: string;
  text: string;
}

export interface TopicSource {
  platform: Platform;
  label: string;
  metric: string;
}

export interface Topic {
  id: string;
  rank: number;
  title: string;
  why: string;
  hook: string;
  platforms: Platform[];
  trendScore: number;
  riskScore: number;
  scriptLength: string;
  scriptBeats: ScriptBeat[];
  caption: string;
  hashtags: string[];
  visualDirection: string;
  sources: TopicSource[];
}

export interface Competitor {
  id: string;
  platform: Platform;
  handle: string;
  niche: string;
  followers: string;
  viewsPerHour: string;
  latestBreakout: string;
  velocity: string;
  velocityUp: boolean;
}

export interface PerfPost {
  id: string;
  platform: Platform;
  title: string;
  views: string;
  avgDuration: string;
  retention: number;
}

export interface PlatformBar {
  platform: Platform;
  name: string;
  value: string;
  rawValue: number;
}

export interface PerfStats {
  label: string;
  value: string;
  delta: string;
  positive: boolean;
}

export interface Settings {
  niche: string;
  region: string;
  platforms: Platform[];
  contentStyle: string;
  riskTolerance: 'low' | 'medium' | 'high';
  brandVoice: string;
}

// ─── Pipeline types ───────────────────────────────────────────

export type PipelineStatus =
  | 'idle'
  | 'generating-script'
  | 'generating-storyboard'
  | 'planning-assets'
  | 'queued-render'
  | 'rendering'
  | 'exporting'
  | 'done'
  | 'error';

export type ExportTarget =
  | 'yt-shorts'
  | 'tiktok'
  | 'ig-reels'
  | 'fb-reels'
  | 'x-twitter'
  | 'threads';

export interface VideoTopic {
  id: string;
  title: string;
  hook: string;
  platforms: Platform[];
  trendScore: number;
  riskScore: number;
}

export interface QualityCheck {
  name: string;
  passed: boolean;
  message: string;
  severity: 'info' | 'warning' | 'error';
}

export interface ScriptDraft {
  topicId: string;
  title: string;
  hook: string;
  body: string;
  cta: string;
  estimatedDuration: number;
  wordCount: number;
  beats: ScriptBeat[];
  qualityChecks: QualityCheck[];
}

export interface StoryboardScene {
  index: number;
  timeRange: string;
  visual: string;
  caption: string;
  type: 'hook' | 'body' | 'cta';
  bRoll: string;
}

export interface AssetPlan {
  topicId: string;
  musicTrack: string;
  voiceStyle: string;
  captionStyle: string;
  colorGrade: string;
  requiredBRoll: string[];
  copyrightSafe: boolean;
  copyrightWarning?: string;
}

export interface RenderJob {
  id: string;
  topicId: string;
  status: 'queued' | 'rendering' | 'done' | 'failed';
  progress: number;
  estimatedSeconds: number;
  format: 'mp4';
  resolution: '1080x1920';
  fps: 30 | 60;
  queuePosition: number;
  enqueuedAt: string;
  completedAt?: string;
}

export interface ExportFile {
  target: ExportTarget;
  filename: string;
  resolution: string;
  durationSec: number;
  sizeKB: number;
  status: 'pending' | 'ready' | 'failed';
}

export interface ExportPackage {
  id: string;
  topicId: string;
  renderJobId: string;
  targets: ExportTarget[];
  status: 'pending' | 'ready' | 'failed';
  files: ExportFile[];
  exportedAt?: string;
}

export interface VideoPipeline {
  id: string;
  topicId: string;
  status: PipelineStatus;
  topic: VideoTopic;
  script?: ScriptDraft;
  storyboard?: StoryboardScene[];
  assetPlan?: AssetPlan;
  renderJob?: RenderJob;
  exportPackage?: ExportPackage;
  qualityWarnings: string[];
  createdAt: string;
  updatedAt: string;
}

export interface AppState {
  view: View;
  openTopicId: string | null;
  signalFilter: Platform | 'all';
  approvals: Record<string, ApprovalStatus>;
  generated: boolean;
  settings: Settings;
}
