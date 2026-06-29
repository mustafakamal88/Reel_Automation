import type { View } from '../types';

const VIEW_META: Record<View, { title: string; sub: string }> = {
  signals: { title: 'Signals', sub: 'Live trend collector — 6 sources, refreshed continuously' },
  scoring: { title: 'Topic Scoring', sub: 'Ranked keyword candidates with full score breakdown' },
  topics: { title: "Today's 6", sub: 'AI-generated short-form topics, ready to review' },
  competitors: { title: 'Competitor Tracker', sub: 'Estimated reach via public signals — views/hr, velocity, repetition' },
  approvals: { title: 'Approvals', sub: 'Review and schedule before anything goes live' },
  performance: { title: 'Performance', sub: 'Own-account watch-time feeding tomorrow\'s topics' },
  settings: { title: 'Settings', sub: 'Configure your niche, region, platforms, and brand voice' },
};

interface Props {
  view: View;
  generated: boolean;
  onGenerate: () => void;
  region?: string;
}

export function Header({ view, generated, onGenerate, region = 'US · Global' }: Props) {
  const { title, sub } = VIEW_META[view];

  return (
    <header className="header">
      <div className="header-title-block">
        <div className="header-title">{title}</div>
        <div className="header-subtitle">{sub}</div>
      </div>

      <div className="header-chip">
        <span className="label">REGION</span>
        <span className="value">{region}</span>
        <span style={{ color: 'var(--text-dim)' }}>▾</span>
      </div>

      <div className="header-chip" aria-label="Current date">
        Mon · Jun 29 2026
      </div>

      <button
        className={`generate-btn${generated ? ' done' : ' idle'}`}
        onClick={onGenerate}
        aria-label={generated ? 'Topics already generated' : 'Generate today\'s 6 topics'}
      >
        <span
          className="generate-btn-dot"
          style={{ background: generated ? 'var(--green)' : '#15121f' }}
        />
        {generated ? 'Generated · just now' : "Generate today's 6"}
      </button>
    </header>
  );
}
