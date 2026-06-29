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

export const NAV_ITEMS: NavItem[] = [
  { id: 'signals',  n: '01', label: 'Signals',        badge: 14 },
  { id: 'scoring',  n: '02', label: 'Scoring',        badge: 8 },
  { id: 'topics',   n: '03', label: "Today's 6",      badge: 6 },
  { id: 'workflow', n: '04', label: 'Daily Workflow',  badge: 6 },
  { id: 'competitors', n: '05', label: 'Competitors', badge: 6 },
  { id: 'approvals', n: '06', label: 'Approvals' },
  { id: 'performance', n: '07', label: 'Performance' },
  { id: 'pipeline',   n: '08', label: 'Pipeline' },
  { id: 'settings',   n: '09', label: 'Settings' },
];

const MOBILE_NAV_ITEMS: NavItem[] = [
  { id: 'topics',   n: '01', label: "Today's 6",      badge: 6 },
  { id: 'signals',  n: '02', label: 'Signals',        badge: 14 },
  { id: 'pipeline', n: '03', label: 'Pipeline' },
  { id: 'workflow', n: '04', label: 'Workflow',       badge: 6 },
  { id: 'settings', n: '05', label: 'Settings' },
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
              data-view={item.id}
              onClick={() => onNavigate(item.id)}
              aria-current={active ? 'page' : undefined}
              type="button"
            >
              <span
                className="nav-item-num"
              >
                {item.n}
              </span>
              <span
                className="nav-item-label"
              >
                {item.label}
              </span>
              {badge && (
                <span className="badge">
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

export function MobileBottomNav({ currentView, onNavigate }: Omit<Props, 'approvals'>) {
  return (
    <nav className="mobile-bottom-nav" aria-label="Mobile main navigation">
      {MOBILE_NAV_ITEMS.map(item => {
        const active = item.id === currentView;

        return (
          <button
            key={item.id}
            className={`mobile-nav-item${active ? ' active' : ''}`}
            onClick={() => onNavigate(item.id)}
            aria-current={active ? 'page' : undefined}
            type="button"
          >
            <span className="mobile-nav-icon" aria-hidden="true">{item.n}</span>
            <span className="mobile-nav-label">{item.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
