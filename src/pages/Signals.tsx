import { useState } from 'react';
import type { Platform } from '../types';
import { SIGNALS } from '../data/signals';
import { PLATFORMS } from '../data/platforms';
import { Sparkline } from '../components/Sparkline';

const FILTERS: { id: Platform | 'all'; label: string; dot: string }[] = [
  { id: 'all', label: 'All sources', dot: '#a78bfa' },
  { id: 'gt', label: 'Google Trends', dot: PLATFORMS.gt.color },
  { id: 'yt', label: 'YouTube', dot: PLATFORMS.yt.color },
  { id: 'tt', label: 'TikTok', dot: PLATFORMS.tt.color },
  { id: 'ig', label: 'Instagram', dot: PLATFORMS.ig.color },
  { id: 'x', label: 'X', dot: PLATFORMS.x.color },
  { id: 'fb', label: 'Facebook', dot: PLATFORMS.fb.color },
];

interface Props {
  initialFilter?: Platform | 'all';
  onFilterChange?: (f: Platform | 'all') => void;
}

export function SignalsPage({ initialFilter = 'all', onFilterChange }: Props) {
  const [filter, setFilter] = useState<Platform | 'all'>(initialFilter);

  const handleFilter = (f: Platform | 'all') => {
    setFilter(f);
    onFilterChange?.(f);
  };

  const filtered = filter === 'all'
    ? SIGNALS
    : SIGNALS.filter(s => s.src === filter);

  return (
    <section className="page-section">
      {/* Filter bar */}
      <div className="filter-bar" role="group" aria-label="Filter by source">
        {FILTERS.map(f => {
          const active = f.id === filter;
          const count = f.id === 'all' ? SIGNALS.length : SIGNALS.filter(s => s.src === f.id).length;
          return (
            <button
              key={f.id}
              className="filter-chip"
              onClick={() => handleFilter(f.id)}
              aria-pressed={active}
              style={{
                background: active ? '#1c2026' : 'var(--bg-card)',
                borderColor: active ? '#343942' : 'var(--border-card)',
                color: active ? 'var(--text-primary)' : 'var(--text-muted)',
                fontFamily: 'inherit',
                cursor: 'pointer',
              }}
            >
              <span className="filter-chip-dot" style={{ background: f.dot }} />
              {f.label}
              <span className="filter-chip-count">{count}</span>
            </button>
          );
        })}
      </div>

      {/* Signal cards */}
      {filtered.length === 0 ? (
        <div className="empty-state">
          <div className="empty-icon">📡</div>
          <div className="empty-title">No signals for this source</div>
          <div className="empty-desc">Try a different filter or check back soon.</div>
        </div>
      ) : (
        <div className="signals-grid">
          {filtered.map(signal => {
            const meta = PLATFORMS[signal.src];
            const up = !signal.growth.startsWith('−');
            const trendClr = up ? '#5fd39a' : '#e8736b';
            return (
              <article key={signal.id} className="signal-card">
                <div
                  className="signal-src-icon"
                  style={{ background: meta.bg, color: meta.color }}
                  aria-label={meta.name}
                >
                  {meta.short}
                </div>
                <div className="signal-body">
                  <div className="signal-type">{signal.type}</div>
                  <div className="signal-label" title={signal.label}>{signal.label}</div>
                  <div className="signal-metric">{signal.metric}</div>
                </div>
                <Sparkline data={signal.sparkData} color={trendClr} />
                <div className="signal-stat">
                  <div className="signal-growth" style={{ color: trendClr }}>{signal.growth}</div>
                  <div className="signal-time">{signal.timestamp}</div>
                </div>
              </article>
            );
          })}
        </div>
      )}

      <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Demo data
        </span>
      </div>
    </section>
  );
}
