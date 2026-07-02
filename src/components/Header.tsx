import { useEffect, useState } from 'react';
import type { View } from '../types';
import { getHealth } from '../lib/api/client';

const VIEW_META: Record<View, { title: string; sub: string }> = {
  signals:     { title: 'Signals',           sub: 'No live trend data connected yet' },
  scoring:     { title: 'Topic Scoring',     sub: 'Real scores will appear after trend sources are connected' },
  topics:      { title: "Today's 6",         sub: 'No real reels generated yet' },
  workflow:    { title: 'Daily Workflow',    sub: 'No automation run yet' },
  batch:       { title: 'Batch & Publish',  sub: 'No uploads have been attempted' },
  realPipeline: { title: 'Real Automation Pipeline', sub: 'Trend discovery → scoring → daily batch → video/export/publish jobs — backed by Postgres' },
  connections: { title: 'Social Connections', sub: 'Connect platform accounts via OAuth — no passwords, official APIs only' },
  competitors: { title: 'Competitor Tracker', sub: 'No competitor tracking configured' },
  approvals:   { title: 'Approvals',        sub: 'No reels awaiting approval' },
  performance: { title: 'Performance',      sub: 'No real publishing history yet' },
  pipeline:    { title: 'Pipeline Studio',  sub: 'Run the Phase 4D render + ZIP test to generate the first local export' },
  settings:    { title: 'Settings',         sub: 'Brand profile, integrations, publishing accounts, and secret management' },
};

interface Props {
  view: View;
  region?: string;
  subtitleOverride?: string;
}

export function Header({ view, region = 'US · Global', subtitleOverride }: Props) {
  const { title, sub } = VIEW_META[view];
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
    <header className="header">
      <div className="header-title-block">
        <div className="header-title">{title}</div>
        <div className="header-subtitle">{subtitleOverride || sub}</div>
      </div>

      <div className="header-chip">
        <span className="label">REGION</span>
        <span className="value">{region}</span>
        <span style={{ color: 'var(--text-dim)' }}>▾</span>
      </div>

      <div className="header-chip" aria-label="Automation status">
        <span
          className="generate-btn-dot"
          style={{ background: backendConnected ? 'var(--green)' : '#15121f' }}
        />
        {backendConnected ? 'Backend connected' : 'No automation run yet'}
      </div>
    </header>
  );
}
