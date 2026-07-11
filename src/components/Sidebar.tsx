import { useEffect, useRef, useState } from 'react';
import type { View } from '../types';
import { LogoMark } from './LogoMark';

interface NavItem {
  id: View;
  label: string;
  future?: boolean;
}

interface Props {
  currentView: View;
  onNavigate: (v: View) => void;
}

const RESEARCH_VIEWS: View[] = [
  'trendingKeywords',
  'platformTrends',
  'youtubeVideoAnalyzer',
  'youtubeChannelAnalyzer',
  'nicheFinder',
];

const CONTENT_VIEWS: View[] = [
  'scriptStudio',
  'clipStudio',
  'voiceStudio',
  'thumbnailStudio',
  'assets',
];

const PUBLISHING_VIEWS: View[] = [
  'connections',
  'calendar',
  'analytics',
];

const DASHBOARD_ITEM: NavItem = { id: 'dashboard', label: 'Dashboard' };
const SETTINGS_ITEM: NavItem = { id: 'settings', label: 'Settings' };

const RESEARCH_ITEMS: NavItem[] = [
  { id: 'trendingKeywords', label: 'Trending Keywords' },
  { id: 'platformTrends', label: 'Platform Trends' },
  { id: 'youtubeVideoAnalyzer', label: 'Video Analyzer' },
  { id: 'youtubeChannelAnalyzer', label: 'Channel Analyzer' },
  { id: 'nicheFinder', label: 'Niche Finder' },
];

const CONTENT_ITEMS: NavItem[] = [
  { id: 'scriptStudio', label: 'Script Studio' },
  { id: 'clipStudio', label: 'Clip Generator' },
  { id: 'voiceStudio', label: 'Voice Studio', future: true },
  { id: 'thumbnailStudio', label: 'Thumbnail Studio', future: true },
  { id: 'assets', label: 'Assets', future: true },
];

const PUBLISHING_ITEMS: NavItem[] = [
  { id: 'connections', label: 'Connections' },
  { id: 'calendar', label: 'Calendar', future: true },
  { id: 'analytics', label: 'Analytics', future: true },
];

function SidebarChrome({ currentView, onNavigate, onAfterNavigate, onClose, drawer = false }: Props & { onAfterNavigate?: () => void; onClose?: () => void; drawer?: boolean }) {
  const navRef = useRef<HTMLElement | null>(null);
  const researchActive = RESEARCH_VIEWS.includes(currentView);
  const contentActive = CONTENT_VIEWS.includes(currentView);
  const publishingActive = PUBLISHING_VIEWS.includes(currentView);
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({
    research: researchActive,
    content: contentActive,
    publishing: publishingActive,
  });

  useEffect(() => {
    if (researchActive) setOpenGroups(current => ({ ...current, research: true }));
    if (contentActive) setOpenGroups(current => ({ ...current, content: true }));
    if (publishingActive) setOpenGroups(current => ({ ...current, publishing: true }));
  }, [contentActive, publishingActive, researchActive]);

  useEffect(() => {
    const activeItem = navRef.current?.querySelector<HTMLElement>('.nav-item.active');
    activeItem?.scrollIntoView({ block: 'nearest' });
  }, [currentView, openGroups.content, openGroups.publishing, openGroups.research]);

  function navigate(view: View) {
    onNavigate(view);
    onAfterNavigate?.();
  }

  function toggleGroup(group: 'research' | 'content' | 'publishing') {
    setOpenGroups(current => ({ ...current, [group]: !current[group] }));
  }

  return (
    <aside className="sidebar" role="navigation" aria-label="Main navigation">
      <div className="sidebar-logo">
        <div className="sidebar-brand-lockup">
          <LogoMark className="sidebar-logo-icon" />
          <div className="sidebar-logo-text">
            <div className="name">TrendCortex</div>
            <div className="tagline">Creator Intelligence</div>
          </div>
        </div>
        {drawer && onClose && (
          <button
            className="sidebar-drawer-close"
            type="button"
            aria-label="Close navigation"
            onClick={onClose}
          >
            <span aria-hidden="true">x</span>
          </button>
        )}
      </div>

      <nav className="sidebar-nav" ref={navRef}>
        <NavButton item={DASHBOARD_ITEM} active={currentView === DASHBOARD_ITEM.id} onNavigate={navigate} />

        <NavGroup
          id="research"
          label="Research"
          items={RESEARCH_ITEMS}
          open={openGroups.research}
          active={researchActive}
          currentView={currentView}
          onToggle={() => toggleGroup('research')}
          onNavigate={navigate}
        />

        <NavGroup
          id="content"
          label="Content"
          items={CONTENT_ITEMS}
          open={openGroups.content}
          active={contentActive}
          currentView={currentView}
          onToggle={() => toggleGroup('content')}
          onNavigate={navigate}
        />

        <NavGroup
          id="publishing"
          label="Publishing"
          items={PUBLISHING_ITEMS}
          open={openGroups.publishing}
          active={publishingActive}
          currentView={currentView}
          onToggle={() => toggleGroup('publishing')}
          onNavigate={navigate}
        />

        <NavButton item={SETTINGS_ITEM} active={currentView === SETTINGS_ITEM.id} onNavigate={navigate} />
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

function NavGroup({ id, label, items, open, active, currentView, onToggle, onNavigate }: {
  id: string;
  label: string;
  items: NavItem[];
  open: boolean;
  active: boolean;
  currentView: View;
  onToggle: () => void;
  onNavigate: (view: View) => void;
}) {
  return (
    <div className={`nav-group${active ? ' active' : ''}${open ? ' open' : ''}`}>
      <button
        className="nav-item nav-parent"
        data-view={id}
        onClick={onToggle}
        aria-expanded={open}
        aria-label={label}
        type="button"
      >
        <span className="nav-item-label">{label}</span>
        <span className="nav-chevron" aria-hidden="true">{open ? '⌃' : '⌄'}</span>
      </button>

      {open && (
        <div className="nav-submenu" role="group" aria-label={label}>
          {items.map(item => (
            <NavButton
              key={item.id}
              item={item}
              active={currentView === item.id}
              onNavigate={onNavigate}
              child
            />
          ))}
        </div>
      )}
    </div>
  );
}

function NavButton({ item, active, child, onNavigate }: {
  item: NavItem;
  active: boolean;
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
      type="button"
    >
      <span className="nav-item-label">{item.label}</span>
      {item.future && <span className="nav-future-badge">Soon</span>}
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
        <SidebarChrome currentView={currentView} onNavigate={onNavigate} onAfterNavigate={onClose} onClose={onClose} drawer />
      </div>
    </>
  );
}
