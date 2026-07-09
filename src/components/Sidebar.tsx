import type { View } from '../types';

interface NavItem {
  id: View;
  label: string;
  short: string;
  badge?: string | number;
}

interface Props {
  currentView: View;
  onNavigate: (v: View) => void;
}

export const NAV_ITEMS: NavItem[] = [
  { id: 'dashboard',    label: 'Dashboard',      short: 'DB' },
  { id: 'trendFinder',  label: 'Trend Finder',   short: 'TF' },
  { id: 'scriptStudio', label: 'Script Studio',  short: 'SS' },
  { id: 'clipStudio',   label: 'Clip Generator', short: 'CG' },
  { id: 'exports',      label: 'Exports',        short: 'EX' },
  { id: 'publish',      label: 'Publish',        short: 'PB' },
  { id: 'connections',  label: 'Connections',    short: 'CN' },
  { id: 'settings',     label: 'Settings',       short: 'ST' },
];

const MOBILE_NAV_ITEMS: NavItem[] = [
  { id: 'dashboard',    label: 'Home',    short: 'DB' },
  { id: 'trendFinder',  label: 'Trends',  short: 'TF' },
  { id: 'scriptStudio', label: 'Scripts', short: 'SS' },
  { id: 'clipStudio',   label: 'Clips',   short: 'CG' },
  { id: 'exports',      label: 'Exports', short: 'EX' },
];

export function Sidebar({ currentView, onNavigate }: Props) {
  return (
    <aside className="sidebar" role="navigation" aria-label="Main navigation">
      <div className="sidebar-logo">
        <div className="sidebar-logo-icon" aria-hidden="true">
          <div className="sidebar-logo-icon-inner" />
        </div>
        <div className="sidebar-logo-text">
          <div className="name">TrendCortex</div>
          <div className="tagline">Catch · Create · Publish</div>
        </div>
      </div>

      <nav className="sidebar-nav">
        {NAV_ITEMS.map(item => {
          const active = item.id === currentView;
          const badge = item.badge != null
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
                {item.short}
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
          <span className="collector-label">Backend connected</span>
        </div>
        <div className="user-card">
          <div className="user-avatar" aria-hidden="true">TC</div>
          <div>
            <div className="user-name">Workspace</div>
            <div className="user-role">No account connected</div>
          </div>
        </div>
      </div>
    </aside>
  );
}

export function MobileBottomNav({ currentView, onNavigate }: Props) {
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
            <span className="mobile-nav-icon" aria-hidden="true">{item.short}</span>
            <span className="mobile-nav-label">{item.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
