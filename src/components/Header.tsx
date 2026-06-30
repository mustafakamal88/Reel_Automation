import type { View } from '../types';

const VIEW_META: Record<View, { title: string; sub: string }> = {
  signals:     { title: 'Signals',           sub: 'Live trend collector — 6 sources, refreshed continuously' },
  scoring:     { title: 'Topic Scoring',     sub: 'Ranked keyword candidates with full score breakdown' },
  topics:      { title: "Today's 6",         sub: 'AI-generated short-form topics, ready to review and approve' },
  workflow:    { title: 'Daily Workflow',    sub: 'Trend discovery → selection → pipeline → approval → schedule' },
  batch:       { title: 'Batch & Publish',  sub: 'Auto-upload to connected platforms or download ZIP — 6 videos/day' },
  realPipeline: { title: 'Real Automation Pipeline', sub: 'Trend discovery → scoring → daily batch → video/export/publish jobs — backed by Postgres' },
  connections: { title: 'Social Connections', sub: 'Connect platform accounts via OAuth — no passwords, official APIs only' },
  competitors: { title: 'Competitor Tracker', sub: 'Estimated reach via public signals — views/hr, velocity, repetition' },
  approvals:   { title: 'Approvals',        sub: 'Review and approve before anything goes live — required before first publish' },
  performance: { title: 'Performance',      sub: 'Own-account watch-time feeding tomorrow\'s topics' },
  pipeline:    { title: 'Pipeline Studio',  sub: 'Script → storyboard → assets → render → export · 6 reels/day' },
  settings:    { title: 'Settings',         sub: 'Brand profile, integrations, publishing accounts, and secret management' },
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
