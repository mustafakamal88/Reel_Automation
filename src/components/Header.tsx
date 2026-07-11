import type { View } from '../types';

const VIEW_META: Record<View, { title: string; sub: string }> = {
  dashboard:              { title: 'Dashboard',                sub: 'Trend research, scripts, clips, and publishing readiness' },
  aiTools:                { title: 'Research Tools',           sub: 'Find trends, channels, and niche ideas worth acting on.' },
  trendingKeywords:       { title: 'Trending Keywords',        sub: 'Find active search and social topics by market.' },
  platformTrends:         { title: 'Platform Trends',          sub: 'Compare trend signals from each connected source.' },
  youtubeVideoAnalyzer:   { title: 'YouTube Video Analyzer',   sub: 'Break down a video into hooks, keywords, and script angles.' },
  youtubeChannelAnalyzer: { title: 'YouTube Channel Analyzer', sub: 'Review a channel profile, content patterns, and next-video ideas.' },
  nicheFinder:            { title: 'Niche Finder',             sub: 'Evaluate demand, competition and readiness for a creator niche.' },
  scriptStudio:           { title: 'Script Studio',            sub: 'Generated scripts and platform copy' },
  clipStudio:             { title: 'Clip Generator',           sub: 'Upload a video or provide a direct video URL, then download clips' },
  connections:            { title: 'Connections',              sub: 'Connect accounts to publish directly from Clip Generator' },
  settings:               { title: 'Settings',                 sub: 'Workspace preferences and product configuration' },
};

interface Props {
  view: View;
  onMenuClick?: () => void;
}

export function Header({ view, onMenuClick }: Props) {
  const { title, sub } = VIEW_META[view];

  return (
    <header className="header">
      <div className="header-inner">
        <button className="mobile-menu-btn" type="button" aria-label="Open navigation" onClick={onMenuClick}>
          <span />
          <span />
          <span />
        </button>

        <div className="header-title-block">
          <div className="header-title">{title}</div>
          <div className="header-subtitle">{sub}</div>
        </div>
      </div>
    </header>
  );
}
