import type { DailyWorkflow, DailyReel, RejectedReel } from '../types';

const SELECTED: DailyReel[] = [];
const REJECTED: RejectedReel[] = [];

export const DAILY_WORKFLOW: DailyWorkflow = {
  date: '',
  generatedAt: '',
  selectedReels: SELECTED,
  rejectedReels: REJECTED,
};
