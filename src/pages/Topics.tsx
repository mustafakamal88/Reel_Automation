import type { ApprovalStatus } from '../types';

interface Props {
  generated: boolean;
  onApprove: (id: string, status: ApprovalStatus) => void;
  onNavigateToApprovals: () => void;
  openTopicId?: string | null;
  onOpenTopic?: (id: string | null) => void;
}

export function TopicsPage(_props: Props) {
  return (
    <section className="page-section">
      <div className="empty-state">
        <div className="empty-icon">T6</div>
        <div className="empty-title">No real reels generated yet.</div>
        <div className="empty-desc">Run the Phase 4D render + ZIP test to generate the first local export.</div>
      </div>
    </section>
  );
}
