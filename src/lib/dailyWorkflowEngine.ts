import type {
  DailyReel,
  WorkflowStatus,
  PipelineHandoffStatus,
} from '../types';

// MOCK ONLY — all functions operate on local state, no API calls, no uploads.

export function computeCompositeScore(
  trendScore: number,
  watchTimePotential: number,
  platformFit: number,
  riskScore: number,
): number {
  const riskPenalty = riskScore >= 55 ? 12 : riskScore >= 30 ? 5 : 0;
  const raw = trendScore * 0.35 + watchTimePotential * 0.30 + platformFit * 0.35 - riskPenalty;
  return Math.min(100, Math.max(0, Math.round(raw)));
}

export function rankReels(reels: DailyReel[]): DailyReel[] {
  return [...reels].sort((a, b) => b.compositeScore - a.compositeScore);
}

export function selectTop6(reels: DailyReel[]): DailyReel[] {
  return rankReels(reels).slice(0, 6);
}

export function mockHandoffToPipeline(reel: DailyReel): DailyReel {
  if (reel.pipelineStatus !== 'not-started') return reel;
  return { ...reel, pipelineStatus: 'queued' as PipelineHandoffStatus };
}

export function updateWorkflowStatus(reel: DailyReel, status: WorkflowStatus): DailyReel {
  return { ...reel, workflowStatus: status };
}

export function workflowStatusMeta(status: WorkflowStatus): {
  label: string;
  color: string;
  bg: string;
  border: string;
} {
  switch (status) {
    case 'approved':    return { label: 'Approved',     color: '#5fd39a', bg: 'rgba(95,211,154,.12)',  border: 'rgba(95,211,154,.2)' };
    case 'scheduled':   return { label: 'Scheduled',    color: '#6aa3f0', bg: 'rgba(106,163,240,.12)', border: 'rgba(106,163,240,.2)' };
    case 'needs-review':return { label: 'Needs Review', color: '#e3b54e', bg: 'rgba(227,181,78,.14)',  border: 'rgba(227,181,78,.2)' };
    case 'rejected':    return { label: 'Rejected',     color: '#e8736b', bg: 'rgba(232,115,107,.12)', border: 'rgba(232,115,107,.2)' };
    case 'draft':
    default:            return { label: 'Draft',        color: '#9b9fa8', bg: 'rgba(155,159,168,.10)', border: 'rgba(155,159,168,.15)' };
  }
}

export function pipelineStatusMeta(status: PipelineHandoffStatus): {
  label: string;
  color: string;
  dot: boolean;
} {
  switch (status) {
    case 'done':        return { label: 'Pipeline done',  color: '#5fd39a', dot: false };
    case 'in-progress': return { label: 'In pipeline',    color: '#a78bfa', dot: true };
    case 'queued':      return { label: 'Queued',         color: '#e3b54e', dot: false };
    case 'not-started':
    default:            return { label: 'Not started',    color: '#4a4e56', dot: false };
  }
}
