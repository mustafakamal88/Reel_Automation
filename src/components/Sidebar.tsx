import { useEffect, useState } from 'react';
import type { View } from '../types';
import { getHealth } from '../lib/api/client';

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
  { id: 'connections',  label: 'Connections',    short: 'CN' },
  { id: 'settings',     label: 'Settings',       short: 'ST' },
];

function SidebarChrome({ currentView, onNavigate, onAfterNavigate }: Props & { onAfterNavigate?: () => void }) {
  const [backendConnected, setBackendConnected] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getHealth()
      .then(res => {
        if (!cancelled) setBackendConnected(Boolean(res.ok));
      })
      .catch(() => {
        if (!cancelled) setBackendConnected(false);
      });
    return () => { cancelled = true; };
  }, []);

  return (
    <aside className="sidebar" role="navigation" aria-label="Main navigation">
      <div className="sidebar-logo">
        <div className="sidebar-logo-icon" aria-hidden="true">
          <div className="sidebar-logo-icon-inner" />
        </div>
        <div className="sidebar-logo-text">
          <div className="name">TrendCortex</div>
          <div className="tagline">Research · Script · Clip</div>
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
              onClick={() => {
                onNavigate(item.id);
                onAfterNavigate?.();
              }}
              aria-current={active ? 'page' : undefined}
              type="button"
            >
              <span className="nav-item-num" aria-hidden="true" />
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
          <span
            className="collector-dot"
            aria-hidden="true"
            style={{ background: backendConnected ? 'var(--green)' : 'var(--red)' }}
          />
          <span className="collector-label">{backendConnected ? 'Backend connected' : 'Backend offline'}</span>
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

export function Sidebar(props: Props) {
  return <SidebarChrome {...props} />;
}

export function MobileNavDrawer({ currentView, onNavigate, open, onClose }: Props & { open: boolean; onClose: () => void }) {
  if (!open) return null;

  return (
    <>
      <button className="mobile-drawer-backdrop" type="button" aria-label="Close navigation" onClick={onClose} />
      <div className="mobile-drawer-panel">
        <SidebarChrome currentView={currentView} onNavigate={onNavigate} onAfterNavigate={onClose} />
      </div>
    </>
  );
}
