import type { View, ApprovalStatus } from '../types';

interface NavItem {
  id: View;
  n: string;
  label: string;
  badge?: string | number;
}

interface Props {
  currentView: View;
  onNavigate: (v: View) => void;
  approvals: Record<string, ApprovalStatus>;
}

const NAV_ITEMS: NavItem[] = [
  { id: 'signals', n: '01', label: 'Signals', badge: 14 },
  { id: 'scoring', n: '02', label: 'Scoring', badge: 8 },
  { id: 'topics', n: '03', label: "Today's 6", badge: 6 },
  { id: 'competitors', n: '04', label: 'Competitors', badge: 6 },
  { id: 'approvals', n: '05', label: 'Approvals' },
  { id: 'performance', n: '06', label: 'Performance' },
  { id: 'pipeline',    n: '07', label: 'Pipeline' },
  { id: 'settings',    n: '08', label: 'Settings' },
];

export function Sidebar({ currentView, onNavigate, approvals }: Props) {
  const pendingCount = Object.values(approvals).filter(s => s === 'pending').length;

  return (
    <aside className="sidebar" role="navigation" aria-label="Main navigation">
      <div className="sidebar-logo">
        <div className="sidebar-logo-icon" aria-hidden="true">
          <div className="sidebar-logo-icon-inner" />
        </div>
        <div className="sidebar-logo-text">
          <div className="name">SIGNAL</div>
          <div className="tagline">trend engine</div>
        </div>
      </div>

      <nav className="sidebar-nav">
        {NAV_ITEMS.map(item => {
          const active = item.id === currentView;
          const badge = item.id === 'approvals' && pendingCount > 0
            ? String(pendingCount)
            : item.badge != null
              ? String(item.badge)
              : null;

          return (
            <button
              key={item.id}
              className={`nav-item${active ? ' active' : ''}`}
              onClick={() => onNavigate(item.id)}
              aria-current={active ? 'page' : undefined}
              style={{
                color: active ? 'var(--text-primary)' : 'var(--text-muted)',
                background: 'none',
                border: 'none',
                width: '100%',
                textAlign: 'left',
                cursor: 'pointer',
                fontFamily: 'inherit',
              }}
            >
              <span
                className="nav-item-num"
                style={{ color: active ? 'var(--accent)' : 'var(--text-dimmer)' }}
              >
                {item.n}
              </span>
              <span
                className="nav-item-label"
                style={{ fontWeight: active ? 600 : 500 }}
              >
                {item.label}
              </span>
              {badge && (
                <span
                  className="badge"
                  style={{
                    background: active ? 'rgba(167,139,250,.2)' : 'var(--bg-subtle)',
                    color: active ? '#c9b8ff' : 'var(--text-dim)',
                  }}
                >
                  {badge}
                </span>
              )}
            </button>
          );
        })}
      </nav>

      <div className="sidebar-footer">
        <div className="collector-status">
          <span className="collector-dot" aria-hidden="true" />
          <span className="collector-label">6 collectors live</span>
        </div>
        <div className="user-card">
          <div className="user-avatar" aria-hidden="true">JD</div>
          <div>
            <div className="user-name">Jordan D.</div>
            <div className="user-role">Growth lead</div>
          </div>
        </div>
      </div>
    </aside>
  );
}
