import type { ApprovalStatus, Settings, WorkflowStatus } from '../types';

const STORAGE_VERSION = 'phase-4f-real-empty-state';
const KEY_STORAGE_VERSION = 'trendcortex_storage_version';
const KEY_APPROVALS = 'signal_approvals';
const KEY_SETTINGS = 'signal_settings';
const KEY_VIEW = 'signal_view';
const KEY_GENERATED = 'signal_generated';
const KEY_WORKFLOW_STATUSES = 'signal_workflow_statuses';

const LEGACY_STORED_DATA_KEYS = [
  KEY_APPROVALS,
  KEY_GENERATED,
  KEY_WORKFLOW_STATUSES,
  'signal_topics',
  'signal_reels',
  'signal_signals',
  'signal_workflow',
  'signal_daily_workflow',
  'signal_batch',
  'signal_batches',
  'signal_performance',
  'signal_pipeline',
  'signal_pipelines',
  'trendcortex_topics',
  'trendcortex_reels',
  'trendcortex_signals',
  'trendcortex_workflow',
  'trendcortex_batch',
  'trendcortex_performance',
  'trendcortex_pipeline',
];

const LEGACY_STORED_DATA_MARKERS = [
  ['AI ', 'agents in 2026'],
  ['home ', 'studio'],
  ['Most people are still using ', 'ChatGPT wrong'],
  ['Generated ', 'just now'],
  ['MO', 'CK / DE', 'MO'],
  ['no real ', 'uploads'],
  ['Motivational ', 'cinematic'],
  ['Pixa', 'bay'],
  ['copyright', '-safe'],
];

export const DEFAULT_SETTINGS: Settings = {
  niche: 'AI tools & creator workflow',
  region: 'US · Global',
  platforms: ['yt', 'tt', 'ig', 'fb', 'x'],
  contentStyle: 'Educational + entertaining (edutainment)',
  riskTolerance: 'medium',
  brandVoice: 'Direct, confident, no fluff. First-person.',
};

export const DEFAULT_APPROVALS: Record<string, ApprovalStatus> = {
};

function containsLegacyStoredData(value: unknown): boolean {
  try {
    const text = typeof value === 'string' ? value : JSON.stringify(value);
    return LEGACY_STORED_DATA_MARKERS.some(marker => text.includes(marker.join('')));
  } catch {
    return false;
  }
}

export function runStorageMigration(): void {
  try {
    const version = localStorage.getItem(KEY_STORAGE_VERSION);
    if (version === STORAGE_VERSION) return;

    for (const key of LEGACY_STORED_DATA_KEYS) {
      localStorage.removeItem(key);
    }

    for (let i = localStorage.length - 1; i >= 0; i -= 1) {
      const key = localStorage.key(i);
      if (!key) continue;
      const raw = localStorage.getItem(key);
      if (raw && containsLegacyStoredData(raw)) {
        localStorage.removeItem(key);
      }
    }

    localStorage.setItem(KEY_STORAGE_VERSION, STORAGE_VERSION);
  } catch {
    // localStorage might be unavailable in some environments
  }
}

function safeGet<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as T;
    if (containsLegacyStoredData(parsed)) {
      localStorage.removeItem(key);
      return fallback;
    }
    return parsed;
  } catch {
    return fallback;
  }
}

function safeSet(key: string, value: unknown): void {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // localStorage might be unavailable in some environments
  }
}

export const storage = {
  migrate(): void {
    runStorageMigration();
  },

  getApprovals(): Record<string, ApprovalStatus> {
    return safeGet(KEY_APPROVALS, DEFAULT_APPROVALS);
  },
  setApprovals(v: Record<string, ApprovalStatus>): void {
    safeSet(KEY_APPROVALS, v);
  },

  getSettings(): Settings {
    return safeGet(KEY_SETTINGS, DEFAULT_SETTINGS);
  },
  setSettings(v: Settings): void {
    safeSet(KEY_SETTINGS, v);
  },

  getView(): string {
    return safeGet(KEY_VIEW, 'signals');
  },
  setView(v: string): void {
    safeSet(KEY_VIEW, v);
  },

  getWorkflowStatuses(): Record<string, WorkflowStatus> {
    return safeGet(KEY_WORKFLOW_STATUSES, {});
  },
  setWorkflowStatuses(v: Record<string, WorkflowStatus>): void {
    safeSet(KEY_WORKFLOW_STATUSES, v);
  },
};
