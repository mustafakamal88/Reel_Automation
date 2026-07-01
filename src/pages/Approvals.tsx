import type { ApprovalStatus } from '../types';

interface Props {
  approvals: Record<string, ApprovalStatus>;
  onApprove: (id: string, status: ApprovalStatus) => void;
}

const STATUS_META: Record<ApprovalStatus, { label: string; color: string; bg: string; border: string }> = {
  approved: { label: 'Approved', color: '#5fd39a', bg: 'rgba(95,211,154,.12)', border: 'var(--border-card)' },
  pending:  { label: 'Pending',  color: '#e3b54e', bg: 'rgba(227,181,78,.12)',  border: '#2a2410' },
  rejected: { label: 'Rejected', color: '#e8736b', bg: 'rgba(232,115,107,.12)', border: 'var(--border-card)' },
};

export function ApprovalsPage(_props: Props) {
  return (
    <section className="page-section">
      <div style={{ display: 'flex', gap: 10, marginBottom: 20 }}>
        {(['pending', 'approved', 'rejected'] as ApprovalStatus[]).map(status => {
          const sm = STATUS_META[status];
          return (
            <div
              key={status}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 7,
                padding: '6px 13px',
                borderRadius: 'var(--radius-md)',
                background: sm.bg,
                border: `1px solid ${sm.border}`,
              }}
            >
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 13, fontWeight: 700, color: sm.color }}>0</span>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{sm.label}</span>
            </div>
          );
        })}
      </div>

      <div className="empty-state">
        <div className="empty-icon">AP</div>
        <div className="empty-title">No reels awaiting approval.</div>
        <div className="empty-desc">Approved, rejected, and pending reels will appear here after real reel plans exist.</div>
      </div>
    </section>
  );
}
