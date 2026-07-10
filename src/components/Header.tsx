import type { View } from '../types';

const VIEW_META: Record<View, { title: string; sub: string }> = {
  dashboard:    { title: 'Dashboard',      sub: 'Trend discovery, scripts, clips, and publishing readiness' },
  trendFinder:  { title: 'Trend Finder',   sub: 'Real keyword discovery from connected sources' },
  scriptStudio: { title: 'Script Studio',  sub: 'Generated scripts and platform copy' },
  clipStudio:   { title: 'Clip Generator', sub: 'Upload a video or provide a direct video URL, then download clips' },
  connections:  { title: 'Connections',    sub: 'Connect accounts to publish directly from Clip Generator' },
  settings:     { title: 'Settings',       sub: 'Workspace preferences and product configuration' },
};

interface Props {
  view: View;
  region?: string;
  subtitleOverride?: string;
  onMenuClick?: () => void;
}

export function Header({ view, region = 'US · Global', subtitleOverride, onMenuClick }: Props) {
  const { title, sub } = VIEW_META[view];

  return (
    <header className="header">
      <button className="mobile-menu-btn" type="button" aria-label="Open navigation" onClick={onMenuClick}>
        <span />
        <span />
        <span />
      </button>

      <div className="header-title-block">
        <div className="header-title">{title}</div>
        <div className="header-subtitle">{subtitleOverride || sub}</div>
      </div>

      <div className="header-chip">
        <span className="label">REGION</span>
        <span className="value">{region}</span>
        <span style={{ color: 'var(--text-dim)' }}>▾</span>
      </div>
    </header>
  );
}
