import { useState } from 'react';
import type { Platform } from '../types';
import { PLATFORMS } from '../data/platforms';

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

  return (
    <section className="page-section">
      <div className="filter-bar" role="group" aria-label="Filter by source">
        {FILTERS.map(f => {
          const active = f.id === filter;
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
              <span className="filter-chip-count">0</span>
            </button>
          );
        })}
      </div>

      <div className="empty-state">
        <div className="empty-icon">ST</div>
        <div className="empty-title">No live trend data connected yet.</div>
        <div className="empty-desc">Connect trend sources/API keys to start collecting real signals.</div>
      </div>
    </section>
  );
}
