import { useEffect, useState } from 'react';
import { getResearchProviderStatus, type ResearchProviderStatus } from '../lib/api/client';

const PROVIDER_ORDER = [
  'google_trends_rss',
  'youtube_data_api',
  'tiktok_research_api',
  'instagram_graph_api',
  'facebook_graph_api',
  'x_api',
  'youtube_analytics',
  'google_ads_keyword_planner',
  'openai',
];

export function DeveloperSystemStatusPage() {
  const [providers, setProviders] = useState<ResearchProviderStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getResearchProviderStatus()
      .then(data => {
        if (!cancelled) {
          setProviders(withOpenAIStatus(data.providers));
          setError(null);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProviders(withOpenAIStatus([]));
          setError('Provider status could not be checked.');
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!import.meta.env.DEV) {
    return (
      <section className="page-section">
        <div className="empty-state system-state is-unavailable" role="status">
          <div className="empty-icon" aria-hidden="true">-</div>
          <div className="empty-title">Developer diagnostics are unavailable.</div>
          <div className="empty-desc">This page is only rendered in development builds.</div>
        </div>
      </section>
    );
  }

  const sorted = [...providers].sort((a, b) => PROVIDER_ORDER.indexOf(a.id) - PROVIDER_ORDER.indexOf(b.id));

  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Developer</div>
          <h1>System Status</h1>
          <p>Safe provider readiness for development and testing. Credential values and raw environment values are never displayed.</p>
        </div>
      </div>

      <div className="settings-card developer-status-card">
        <div className="settings-card-title">Provider diagnostics</div>
        {loading && <div className="status-banner is-loading" role="status" aria-live="polite"><strong>Checking status:</strong> Provider readiness is being loaded.</div>}
        {error && <div className="status-banner is-error" role="alert"><strong>Status unavailable:</strong> {error}</div>}
        <div className="status-list">
          {sorted.map(provider => (
            <div className="status-row" key={provider.id}>
              <span>{provider.name}</span>
              <strong>{safeProviderLabel(provider.status)}</strong>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function withOpenAIStatus(providers: ResearchProviderStatus[]): ResearchProviderStatus[] {
  if (providers.some(provider => provider.id === 'openai')) return providers;
  return [
    ...providers,
    {
      id: 'openai',
      name: 'OpenAI',
      platform: 'ai',
      status: 'unavailable',
      message: 'OpenAI readiness is checked server-side during generation.',
    },
  ];
}

function safeProviderLabel(status: ResearchProviderStatus['status'] | 'unknown'): string {
  if (status === 'active') return 'Ready';
  if (status === 'not_configured') return 'Setup needed';
  if (status === 'unavailable') return 'Status unavailable';
  return 'Status unavailable';
}
