import { useEffect, useMemo, useState } from 'react';
import type { Platform } from '../types';
import { PLATFORMS } from '../data/platforms';
import { ApiError, discoverTrendCandidates, type TrendCandidate, type TrendDiscoveryResponse } from '../lib/api/client';

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
  const [response, setResponse] = useState<TrendDiscoveryResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const handleFilter = (f: Platform | 'all') => {
    setFilter(f);
    onFilterChange?.(f);
  };

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    discoverTrendCandidates({ region: 'US', language: 'en-US', limit: 20 })
      .then(data => {
        if (!cancelled) setResponse(data);
      })
      .catch(err => {
        if (cancelled) return;
        if (err instanceof ApiError) {
          setError(err.message);
        } else {
          setError('Trend discovery request failed.');
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const filteredCandidates = useMemo(() => {
    const candidates = response?.candidates ?? [];
    if (filter === 'all') return candidates;
    if (filter === 'gt') return candidates.filter(c => c.source === 'google_trends_rss');
    return [];
  }, [filter, response]);

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
              <span className="filter-chip-count">{countForFilter(f.id, response?.candidates ?? [])}</span>
            </button>
          );
        })}
      </div>

      {loading && (
        <div className="empty-state">
          <div className="empty-icon">ST</div>
          <div className="empty-title">Loading real trend candidates.</div>
          <div className="empty-desc">Requesting backend discovery provider status.</div>
        </div>
      )}

      {!loading && error && (
        <div className="empty-state">
          <div className="empty-icon">ER</div>
          <div className="empty-title">Trend discovery is unavailable.</div>
          <div className="empty-desc">{error}</div>
        </div>
      )}

      {!loading && !error && response?.provider_status === 'provider_not_configured' && (
        <div className="empty-state">
          <div className="empty-icon">NC</div>
          <div className="empty-title">No trend provider configured.</div>
          <div className="empty-desc">{response.message || 'Configure a real backend trend provider to collect signals.'}</div>
        </div>
      )}

      {!loading && !error && response?.provider_status === 'no_data' && (
        <div className="empty-state">
          <div className="empty-icon">ND</div>
          <div className="empty-title">No real trend data found.</div>
          <div className="empty-desc">{response.message || 'The configured provider returned no candidates for this request.'}</div>
        </div>
      )}

      {!loading && !error && response?.provider_status === 'provider_error' && (
        <div className="empty-state">
          <div className="empty-icon">PE</div>
          <div className="empty-title">Trend provider error.</div>
          <div className="empty-desc">{response.message || 'The backend provider request failed.'}</div>
        </div>
      )}

      {!loading && !error && response?.provider_status === 'ok' && filteredCandidates.length === 0 && (
        <div className="empty-state">
          <div className="empty-icon">ST</div>
          <div className="empty-title">No candidates for this source.</div>
          <div className="empty-desc">The backend returned real candidates, but none match the selected source filter.</div>
        </div>
      )}

      {!loading && !error && response?.provider_status === 'ok' && filteredCandidates.length > 0 && (
        <div style={{ display: 'grid', gap: 10 }}>
          {filteredCandidates.map(candidate => (
            <article
              key={candidate.id}
              style={{
                display: 'grid',
                gridTemplateColumns: 'minmax(0, 1fr) auto',
                gap: 14,
                padding: '14px 16px',
                background: 'var(--bg-card)',
                border: '1px solid var(--border-card)',
                borderRadius: 8,
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                  <span className="filter-chip-dot" style={{ background: PLATFORMS.gt.color }} />
                  <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
                    {candidate.source.replaceAll('_', ' ')} · {candidate.region} · {candidate.language}
                  </span>
                </div>
                <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--text-primary)', overflowWrap: 'anywhere' }}>
                  {candidate.title || candidate.keyword}
                </div>
                {candidate.evidence && (
                  <div style={{ marginTop: 5, fontSize: 12, color: 'var(--text-muted)', overflowWrap: 'anywhere' }}>
                    {candidate.evidence}
                  </div>
                )}
                {candidate.source_url && (
                  <a
                    href={candidate.source_url}
                    target="_blank"
                    rel="noreferrer"
                    style={{ display: 'inline-block', marginTop: 8, fontSize: 12, color: 'var(--accent)' }}
                  >
                    Source evidence
                  </a>
                )}
              </div>
              <div style={{ textAlign: 'right', minWidth: 88 }}>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: 20, fontWeight: 700, color: 'var(--green)' }}>
                  {Math.round(candidate.score)}
                </div>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
                  score
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function countForFilter(filter: Platform | 'all', candidates: TrendCandidate[]): number {
  if (filter === 'all') return candidates.length;
  if (filter === 'gt') return candidates.filter(c => c.source === 'google_trends_rss').length;
  return 0;
}
