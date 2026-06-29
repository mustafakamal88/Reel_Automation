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
  | 'settings';

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

export interface AppState {
  view: View;
  openTopicId: string | null;
  signalFilter: Platform | 'all';
  approvals: Record<string, ApprovalStatus>;
  generated: boolean;
  settings: Settings;
}
