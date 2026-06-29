import type { ApprovalStatus, Settings } from '../types';

const KEY_APPROVALS = 'signal_approvals';
const KEY_SETTINGS = 'signal_settings';
const KEY_VIEW = 'signal_view';
const KEY_GENERATED = 'signal_generated';

export const DEFAULT_SETTINGS: Settings = {
  niche: 'AI tools & creator workflow',
  region: 'US · Global',
  platforms: ['yt', 'tt', 'ig', 'fb', 'x'],
  contentStyle: 'Educational + entertaining (edutainment)',
  riskTolerance: 'medium',
  brandVoice: 'Direct, confident, no fluff. First-person.',
};

export const DEFAULT_APPROVALS: Record<string, ApprovalStatus> = {
  t1: 'approved',
  t2: 'pending',
  t3: 'rejected',
  t4: 'pending',
  t5: 'approved',
  t6: 'pending',
};

function safeGet<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return fallback;
    return JSON.parse(raw) as T;
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

  getGenerated(): boolean {
    return safeGet(KEY_GENERATED, false);
  },
  setGenerated(v: boolean): void {
    safeSet(KEY_GENERATED, v);
  },
};
