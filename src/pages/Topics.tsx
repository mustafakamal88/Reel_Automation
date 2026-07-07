import type { ApprovalStatus } from '../types';
import { DailyBatchPage } from './DailyBatch';

interface Props {
  generated: boolean;
  onApprove: (id: string, status: ApprovalStatus) => void;
  onNavigateToApprovals: () => void;
  openTopicId?: string | null;
  onOpenTopic?: (id: string | null) => void;
}

export function TopicsPage(_props: Props) {
  return <DailyBatchPage />;
}
