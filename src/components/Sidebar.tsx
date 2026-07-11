import { useEffect, useMemo, useState } from 'react';
import type { View } from '../types';
import { LogoMark } from './LogoMark';

interface NavItem {
  id: View;
  label: string;
  short: string;
}

interface Props {
  currentView: View;
  onNavigate: (v: View) => void;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
}

const RESEARCH_TOOL_VIEWS: View[] = [
  'trendingKeywords',
  'platformTrends',
  'youtubeVideoAnalyzer',
  'youtubeChannelAnalyzer',
  'nicheFinder',
];

const PRIMARY_ITEMS: NavItem[] = [
  { id: 'dashboard', label: 'Dashboard', short: 'DB' },
  { id: 'scriptStudio', label: 'Script Studio', short: 'SS' },
  { id: 'clipStudio', label: 'Clip Generator', short: 'CG' },
  { id: 'connections', label: 'Connections', short: 'CN' },
  { id: 'settings', label: 'Settings', short: 'ST' },
];

const RESEARCH_TOOL_ITEMS: NavItem[] = [
  { id: 'trendingKeywords', label: 'Trending Keywords', short: 'TK' },
  { id: 'platformTrends', label: 'Platform Trends', short: 'PT' },
  { id: 'youtubeVideoAnalyzer', label: 'YouTube Video Analyzer', short: 'YV' },
  { id: 'youtubeChannelAnalyzer', label: 'YouTube Channel Analyzer', short: 'YC' },
  { id: 'nicheFinder', label: 'Niche Finder', short: 'NF' },
];

function isAIToolsView(view: View) {
  return view === 'aiTools' || RESEARCH_TOOL_VIEWS.includes(view);
}

function SidebarChrome({ currentView, onNavigate, collapsed = false, onToggleCollapse, onAfterNavigate }: Props & { onAfterNavigate?: () => void }) {
  const aiActive = isAIToolsView(currentView);
  const aiLandingActive = currentView === 'aiTools';
  const [aiOpen, setAiOpen] = useState(aiActive);

  useEffect(() => {
    if (aiActive) setAiOpen(true);
  }, [aiActive]);

  const navItems = useMemo(() => {
    const [dashboard, ...rest] = PRIMARY_ITEMS;
    return { dashboard, rest };
  }, []);

  function navigate(view: View) {
    onNavigate(view);
    onAfterNavigate?.();
  }

  return (
    <aside className={`sidebar${collapsed ? ' is-collapsed' : ''}${aiOpen ? ' ai-open' : ''}`} role="navigation" aria-label="Main navigation">
      <div className="sidebar-logo">
        <LogoMark className="sidebar-logo-icon" />
        <div className="sidebar-logo-text">
          <div className="name">TrendCortex</div>
          <div className="tagline">Creator Intelligence</div>
        </div>
        {onToggleCollapse && (
          <button
            className="sidebar-collapse-btn"
            type="button"
            aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'}
            aria-expanded={!collapsed}
            onClick={onToggleCollapse}
            title={collapsed ? 'Expand navigation' : 'Collapse navigation'}
          >
            <span aria-hidden="true">{collapsed ? '›' : '‹'}</span>
          </button>
        )}
      </div>

      <nav className="sidebar-nav">
        <NavButton item={navItems.dashboard} active={currentView === navItems.dashboard.id} collapsed={collapsed} onNavigate={navigate} />

        <div className={`nav-group${aiActive ? ' active' : ''}${aiOpen ? ' open' : ''}`}>
          <button
            className={`nav-item nav-parent${aiLandingActive ? ' active' : ''}`}
            data-view="aiTools"
            onClick={() => {
              if (collapsed) {
                setAiOpen(true);
                navigate('trendingKeywords');
                return;
              }
              setAiOpen(current => !current);
              if (!aiActive) navigate('trendingKeywords');
            }}
            aria-expanded={aiOpen}
            aria-current={aiActive && currentView === 'aiTools' ? 'page' : undefined}
            aria-label="Research Tools"
            title={collapsed ? 'Research Tools' : undefined}
            type="button"
          >
            <span className="nav-item-num" aria-hidden="true">RT</span>
            <span className="nav-item-label">Research Tools</span>
            <span className="nav-chevron" aria-hidden="true">{aiOpen ? '⌃' : '⌄'}</span>
          </button>

          {aiOpen && (
            <div className="nav-submenu" role="group" aria-label="Research Tools">
              {RESEARCH_TOOL_ITEMS.map(item => (
                <NavButton
                  key={item.id}
                  item={item}
                  active={currentView === item.id}
                  collapsed={collapsed}
                  onNavigate={navigate}
                  child
                />
              ))}
            </div>
          )}
        </div>

        {navItems.rest.map(item => (
          <NavButton key={item.id} item={item} active={currentView === item.id} collapsed={collapsed} onNavigate={navigate} />
        ))}
      </nav>

      <div className="sidebar-footer">
        <div className="collector-status">
          <span className="collector-dot" aria-hidden="true" />
          <span className="collector-label">Creator workspace</span>
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

function NavButton({ item, active, collapsed, child, onNavigate }: {
  item: NavItem;
  active: boolean;
  collapsed?: boolean;
  child?: boolean;
  onNavigate: (view: View) => void;
}) {
  return (
    <button
      className={`nav-item${active ? ' active' : ''}${child ? ' nav-child' : ''}`}
      data-view={item.id}
      onClick={() => onNavigate(item.id)}
      aria-current={active ? 'page' : undefined}
      aria-label={item.label}
      title={collapsed ? item.label : undefined}
      type="button"
    >
      <span className="nav-item-num" aria-hidden="true">{item.short}</span>
      <span className="nav-item-label">{item.label}</span>
    </button>
  );
}

export function Sidebar(props: Props) {
  return <SidebarChrome {...props} />;
}

export function MobileNavDrawer({ currentView, onNavigate, open, onClose }: Props & { open: boolean; onClose: () => void }) {
  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <>
      <button className="mobile-drawer-backdrop" type="button" aria-label="Close navigation" onClick={onClose} />
      <div className="mobile-drawer-panel" role="dialog" aria-modal="true" aria-label="Navigation">
        <SidebarChrome currentView={currentView} onNavigate={onNavigate} onAfterNavigate={onClose} collapsed={false} />
      </div>
    </>
  );
}
