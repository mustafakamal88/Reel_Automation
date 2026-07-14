import type { View } from '../types';

const VIEW_META: Record<View, { title: string; sub: string }> = {
  dashboard:              { title: 'Dashboard',                sub: 'Trend research, scripts, clips, and publishing readiness' },
  discoverTrends:         { title: 'Discover Trends',          sub: 'The latest content opportunities for your selected market.' },
  trendingKeywords:       { title: 'Keyword Search',           sub: 'Search and rank content opportunities by keyword.' },
  youtubeVideoAnalyzer:   { title: 'Video Analyzer',           sub: 'Break down a YouTube video into hooks, keywords, and script angles.' },
  youtubeChannelAnalyzer: { title: 'Channel Analyzer',         sub: 'Review a YouTube channel profile, content patterns, and next-video ideas.' },
  nicheFinder:            { title: 'Niche Finder',             sub: 'Evaluate demand, competition and readiness for a creator niche.' },
  contentProjects:        { title: 'Projects',                 sub: 'Persistent content projects and production status' },
  scriptStudio:           { title: 'Script Studio',            sub: 'Generated scripts and platform copy' },
  clipStudio:             { title: 'Clip Generator',           sub: 'Upload a video or provide a direct video URL, then download clips' },
  voiceStudio:            { title: 'Voice Studio',             sub: 'Planned narration and voice workflow' },
  thumbnailStudio:        { title: 'Thumbnail Studio',         sub: 'Planned thumbnail creation workflow' },
  assets:                 { title: 'Assets',                   sub: 'Planned creative asset workspace' },
  connections:            { title: 'Connections',              sub: 'Connect accounts to publish directly from Clip Generator' },
  calendar:               { title: 'Calendar',                 sub: 'Planned publishing schedule' },
  analytics:              { title: 'Analytics',                sub: 'Planned connected-platform performance reporting' },
  settings:               { title: 'Settings',                 sub: 'Workspace preferences and product configuration' },
  developerSystemStatus:  { title: 'System Status',            sub: 'Development-only provider diagnostics' },
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
