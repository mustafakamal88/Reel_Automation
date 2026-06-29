import type { ApprovalStatus } from '../types';
import { TOPICS } from '../data/topics';
import { PLATFORMS } from '../data/platforms';
import { PlatformDot } from '../components/PlatformDot';

interface Props {
  approvals: Record<string, ApprovalStatus>;
  onApprove: (id: string, status: ApprovalStatus) => void;
}

const STATUS_META: Record<ApprovalStatus, { label: string; color: string; bg: string; border: string }> = {
  approved: { label: 'Approved', color: '#5fd39a', bg: 'rgba(95,211,154,.12)', border: 'var(--border-card)' },
  pending:  { label: 'Pending',  color: '#e3b54e', bg: 'rgba(227,181,78,.12)',  border: '#2a2410' },
  rejected: { label: 'Rejected', color: '#e8736b', bg: 'rgba(232,115,107,.12)', border: 'var(--border-card)' },
};

export function ApprovalsPage({ approvals, onApprove }: Props) {
  const counts = TOPICS.reduce(
    (acc, t) => {
      const s = approvals[t.id] ?? 'pending';
      acc[s] = (acc[s] ?? 0) + 1;
      return acc;
    },
    {} as Record<ApprovalStatus, number>,
  );

  return (
    <section className="page-section">
      {/* Summary row */}
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
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 13, fontWeight: 700, color: sm.color }}>
                {counts[status] ?? 0}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{sm.label}</span>
            </div>
          );
        })}
      </div>

      <div className="approvals-list">
        {TOPICS.map(topic => {
          const status = approvals[topic.id] ?? 'pending';
          const sm = STATUS_META[status];
          const rankLabel = String(topic.rank).padStart(2, '0');
          const isApproved = status === 'approved';
          const scheduleText = isApproved
            ? `Scheduled · ${topic.platforms.map(p => PLATFORMS[p].short).join(' · ')}`
            : `Targets: ${topic.platforms.map(p => PLATFORMS[p].short).join(' · ')}`;

          return (
            <div
              key={topic.id}
              className="approval-row"
              style={{
                background: status === 'pending' ? 'rgba(227,181,78,.04)' : 'var(--bg-card)',
                borderColor: sm.border,
              }}
            >
              <span className="approval-rank">{rankLabel}</span>

              <div className="approval-body">
                <div className="approval-title">{topic.title}</div>
                <div className="approval-meta">
                  {topic.platforms.map(p => (
                    <PlatformDot key={p} platform={p} size={16} />
                  ))}
                  <span className="approval-schedule">· {scheduleText}</span>
                </div>
              </div>

              <span
                className="approval-status-badge"
                style={{ color: sm.color, background: sm.bg }}
              >
                {sm.label}
              </span>

              <div className="approval-actions">
                <button
                  className={`btn-approve${isApproved ? ' active' : ''}`}
                  onClick={() => onApprove(topic.id, 'approved')}
                  disabled={isApproved}
                  aria-label={`Approve topic: ${topic.title}`}
                >
                  Approve
                </button>
                <button
                  className="btn-reject"
                  onClick={() => onApprove(topic.id, 'rejected')}
                  aria-label={`Reject topic: ${topic.title}`}
                >
                  Reject
                </button>
              </div>
            </div>
          );
        })}
      </div>

      <div style={{ marginTop: 16 }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Approval state persists in localStorage
        </span>
      </div>
    </section>
  );
}
