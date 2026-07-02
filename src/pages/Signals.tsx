import { useEffect, useMemo, useState } from 'react';
import type { Platform } from '../types';
import { PLATFORMS } from '../data/platforms';
import {
  ApiError,
  discoverTrendCandidates,
  generateReelScript,
  type ReelContentPackage,
  type TrendCandidate,
  type TrendDiscoveryResponse,
} from '../lib/api/client';

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
  onStatusChange?: (status: string) => void;
}

export function SignalsPage({ initialFilter = 'all', onFilterChange, onStatusChange }: Props) {
  const [filter, setFilter] = useState<Platform | 'all'>(initialFilter);
  const [response, setResponse] = useState<TrendDiscoveryResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [generatingID, setGeneratingID] = useState<string | null>(null);
  const [generated, setGenerated] = useState<Record<string, ReelContentPackage>>({});
  const [generationErrors, setGenerationErrors] = useState<Record<string, string>>({});

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

  useEffect(() => {
    if (!onStatusChange) return;
    if (loading) {
      onStatusChange('Checking live trend provider status');
      return;
    }
    if (error) {
      onStatusChange('Trend discovery unavailable');
      return;
    }
    onStatusChange(statusSubtitle(response));
  }, [error, loading, onStatusChange, response]);

  const filteredCandidates = useMemo(() => {
    const candidates = response?.candidates ?? [];
    if (filter === 'all') return candidates;
    if (filter === 'gt') return candidates.filter(c => c.source === 'google_trends_rss');
    return [];
  }, [filter, response]);

  const handleGenerate = (candidate: TrendCandidate) => {
    setGeneratingID(candidate.id);
    setGenerationErrors(prev => {
      const next = { ...prev };
      delete next[candidate.id];
      return next;
    });
    generateReelScript({
      trend_candidate_id: candidate.id,
      trend_candidate: candidate,
      platform_targets: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
      duration_target: '30s',
      language: candidate.language || 'en-US',
      region: candidate.region || 'US',
    })
      .then(data => {
        setGenerated(prev => ({ ...prev, [candidate.id]: data.package }));
      })
      .catch(err => {
        if (err instanceof ApiError) {
          setGenerationErrors(prev => ({ ...prev, [candidate.id]: err.message }));
        } else {
          setGenerationErrors(prev => ({ ...prev, [candidate.id]: 'Script generation request failed.' }));
        }
      })
      .finally(() => {
        setGeneratingID(current => (current === candidate.id ? null : current));
      });
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
                <div style={{ marginTop: 12, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                  <button
                    type="button"
                    onClick={() => handleGenerate(candidate)}
                    disabled={generatingID === candidate.id}
                    style={{
                      border: '1px solid var(--border-card)',
                      background: generatingID === candidate.id ? '#1c2026' : 'var(--accent)',
                      color: generatingID === candidate.id ? 'var(--text-muted)' : '#07110d',
                      borderRadius: 6,
                      padding: '7px 10px',
                      fontFamily: 'inherit',
                      fontSize: 12,
                      fontWeight: 700,
                      cursor: generatingID === candidate.id ? 'wait' : 'pointer',
                    }}
                  >
                    {generatingID === candidate.id ? 'Generating...' : 'Generate script'}
                  </button>
                  {generated[candidate.id] && (
                    <span style={{ alignSelf: 'center', fontSize: 12, color: 'var(--green)' }}>
                      Script package ready
                    </span>
                  )}
                </div>
                {generationErrors[candidate.id] && (
                  <div style={{ marginTop: 10, fontSize: 12, color: 'var(--red)', overflowWrap: 'anywhere' }}>
                    {generationErrors[candidate.id]}
                  </div>
                )}
                {generated[candidate.id] && (
                  <GeneratedPackageView pkg={generated[candidate.id]} />
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

function statusSubtitle(response: TrendDiscoveryResponse | null): string {
  if (!response) return 'No real trend data found';
  if (response.provider_status === 'provider_not_configured') return 'No trend provider configured';
  if (response.provider_status === 'provider_error') return 'Trend provider error';
  if (response.provider_status === 'no_data') return 'No real trend data found';
  if ((response.candidates?.length ?? 0) > 0) return 'Live Google Trends RSS data';
  return 'No real trend data found';
}

function GeneratedPackageView({ pkg }: { pkg: ReelContentPackage }) {
  return (
    <div
      style={{
        marginTop: 12,
        padding: 12,
        border: '1px solid var(--border-card)',
        borderRadius: 8,
        background: '#10141a',
        display: 'grid',
        gap: 10,
      }}
    >
      <div style={{ fontSize: 13, fontWeight: 800, color: 'var(--text-primary)', overflowWrap: 'anywhere' }}>
        {pkg.title}
      </div>
      <TextBlock label="Hook" value={pkg.hook} />
      <TextBlock label="Script" value={pkg.script} />
      <TextBlock label="Caption" value={pkg.caption} />
      {pkg.hashtags?.length > 0 && (
        <div style={{ fontSize: 12, color: 'var(--text-muted)', overflowWrap: 'anywhere' }}>
          <strong style={{ color: 'var(--text-primary)' }}>Hashtags:</strong> {pkg.hashtags.join(' ')}
        </div>
      )}
      <div style={{ display: 'grid', gap: 6 }}>
        <TextBlock label="Instagram" value={pkg.instagram_caption} />
        <TextBlock label="TikTok" value={pkg.tiktok_caption} />
        <TextBlock label="YouTube title" value={pkg.youtube_title} />
        <TextBlock label="Facebook" value={pkg.facebook_caption} />
        <TextBlock label="X" value={pkg.x_caption} />
      </div>
      <TextBlock label="Thumbnail brief" value={pkg.thumbnail_brief} />
      {pkg.safety_grounding_notes?.length > 0 && (
        <div style={{ fontSize: 12, color: 'var(--text-dim)', overflowWrap: 'anywhere' }}>
          <strong>Grounding:</strong> {pkg.safety_grounding_notes.join(' ')}
        </div>
      )}
      <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
        {pkg.provider_metadata.provider} · {pkg.provider_metadata.model} · {pkg.provider_metadata.source}
      </div>
      <button
        type="button"
        disabled
        title="Prepared for Phase 4I/4J pipeline handoff"
        style={{
          justifySelf: 'start',
          border: '1px solid var(--border-card)',
          background: '#151a21',
          color: 'var(--text-dim)',
          borderRadius: 6,
          padding: '7px 10px',
          fontFamily: 'inherit',
          fontSize: 12,
        }}
      >
        Use in Pipeline
      </button>
    </div>
  );
}

function TextBlock({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <div style={{ fontSize: 12, color: 'var(--text-muted)', overflowWrap: 'anywhere', lineHeight: 1.5 }}>
      <strong style={{ color: 'var(--text-primary)' }}>{label}:</strong> {value}
    </div>
  );
}
