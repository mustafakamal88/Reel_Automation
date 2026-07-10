import { useEffect, useState } from 'react';
import type { Platform, Settings } from '../types';
import { PLATFORMS } from '../data/platforms';
import { getResearchProviderStatus, type ResearchProviderStatus } from '../lib/api/client';

const CREATOR_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x'];

interface Props {
  settings: Settings;
  onSave: (s: Settings) => void;
}

export function SettingsPage({ settings: initial, onSave }: Props) {
  const settings = initial;
  const [providers, setProviders] = useState<ResearchProviderStatus[]>([]);

  useEffect(() => {
    let cancelled = false;
    getResearchProviderStatus()
      .then(data => {
        if (!cancelled) setProviders(data.providers);
      })
      .catch(() => {
        if (!cancelled) setProviders([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function setField<K extends keyof Settings>(key: K, value: Settings[K]) {
    onSave({ ...settings, [key]: value });
  }

  function togglePlatform(platform: Platform) {
    const platforms = settings.platforms.includes(platform)
      ? settings.platforms.filter(item => item !== platform)
      : [...settings.platforms, platform];
    setField('platforms', platforms);
  }

  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Settings</div>
          <h1>Configure the creator workspace.</h1>
          <p>Manage workspace preferences, source readiness, publishing setup, and default branding without exposing secrets.</p>
        </div>
      </div>

      <div className="settings-grid">
        <div className="settings-card">
          <div className="settings-card-title">Workspace</div>
          <div className="form-group">
            <label className="form-label" htmlFor="niche">Creator niche</label>
            <input id="niche" className="form-input" value={settings.niche} onChange={event => setField('niche', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="region">Region</label>
            <input id="region" className="form-input" value={settings.region} onChange={event => setField('region', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="content-style">Content style</label>
            <input id="content-style" className="form-input" value={settings.contentStyle} onChange={event => setField('contentStyle', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="brand-voice">Brand voice</label>
            <textarea id="brand-voice" className="form-textarea" value={settings.brandVoice} onChange={event => setField('brandVoice', event.target.value)} />
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Trend Data Providers</div>
          <div className="status-list">
            <ProviderStatusRow providers={providers} id="google_trends_rss" fallbackLabel="Google Trends RSS" />
            <ProviderStatusRow providers={providers} id="youtube_data_api" fallbackLabel="YouTube Data API" />
            <ProviderStatusRow providers={providers} id="tiktok_research_api" fallbackLabel="TikTok Research API" />
            <ProviderStatusRow providers={providers} id="instagram_graph_api" fallbackLabel="Instagram Graph/Meta" />
            <ProviderStatusRow providers={providers} id="x_api" fallbackLabel="X API" />
            <ProviderStatusRow providers={providers} id="facebook_graph_api" fallbackLabel="Facebook/Meta" />
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">YouTube Analyzer</div>
          <div className="status-list">
            <ProviderStatusRow providers={providers} id="youtube_data_api" fallbackLabel="YouTube analyzer" />
          </div>
          <div className="muted-note">Only configuration status is displayed. API keys are never shown in the browser.</div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Monetization Data Providers</div>
          <div className="status-list">
            <ProviderStatusRow providers={providers} id="youtube_analytics" fallbackLabel="YouTube Analytics" />
            <ProviderStatusRow providers={providers} id="google_ads_keyword_planner" fallbackLabel="Google Ads Keyword Planner" />
            <ProviderStatusRow providers={providers} id="youtube_data_api" fallbackLabel="YouTube Data API" />
            <ProviderStatusRow providers={providers} id="google_trends_rss" fallbackLabel="Google Trends RSS" />
          </div>
          <div className="muted-note">Exact revenue/RPM requires authorized YouTube Analytics for owned channels. Public niche monetization uses proxy estimates.</div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">AI / Script Provider</div>
          <div className="status-list">
            <StatusRow label="OpenAI" value="Ready when the server-side key is configured" />
          </div>
          <div className="muted-note">API keys stay server-side and are never displayed here.</div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Social Publishing Providers</div>
          <div className="status-list">
            <StatusRow label="Account connection setup" value="Ready when platform apps are configured" />
            <StatusRow label="Direct publishing" value="Disabled until real platform APIs are wired" />
          </div>
          <div className="muted-note">Publishing starts from Clip Generator after accounts are connected.</div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Target Platforms</div>
          <div className="platform-toggles">
            {CREATOR_PLATFORMS.map(platform => {
              const meta = PLATFORMS[platform];
              const active = settings.platforms.includes(platform);
              return (
                <button
                  key={platform}
                  type="button"
                  className="platform-toggle"
                  onClick={() => togglePlatform(platform)}
                  style={{
                    borderColor: active ? meta.color : 'var(--border-card)',
                    background: active ? meta.bg : 'var(--bg-subtle)',
                    color: active ? meta.color : 'var(--text-muted)',
                  }}
                >
                  <span className="filter-chip-dot" style={{ background: meta.color }} />
                  {meta.name}
                </button>
              );
            })}
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Brand Defaults</div>
          <div className="form-group">
            <label className="form-label" htmlFor="default-top">Default top text</label>
            <input id="default-top" className="form-input" value={settings.defaultTopText} onChange={event => setField('defaultTopText', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="default-bottom">Default bottom text</label>
            <input id="default-bottom" className="form-input" value={settings.defaultBottomText} onChange={event => setField('defaultBottomText', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="default-watermark">Default watermark / channel name</label>
            <input id="default-watermark" className="form-input" value={settings.defaultWatermark} onChange={event => setField('defaultWatermark', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="default-layout">Default layout mode</label>
            <select id="default-layout" className="form-input" value={settings.defaultLayoutMode} onChange={event => setField('defaultLayoutMode', event.target.value as Settings['defaultLayoutMode'])}>
              <option value="blurred_background">Blurred background</option>
              <option value="fill_crop">Fill crop</option>
              <option value="fit_with_bars">Fit with bars</option>
            </select>
          </div>
        </div>
      </div>
    </section>
  );
}

function StatusRow({ label, value, tone }: { label: string; value: string; tone?: 'good' }) {
  return (
    <div className="status-row">
      <span>{label}</span>
      <strong style={{ color: tone === 'good' ? 'var(--green)' : undefined }}>{value}</strong>
    </div>
  );
}

function ProviderStatusRow({ providers, id, fallbackLabel }: { providers: ResearchProviderStatus[]; id: string; fallbackLabel: string }) {
  const provider = providers.find(item => item.id === id);
  const active = provider?.status === 'active';
  return (
    <StatusRow
      label={provider?.name ?? fallbackLabel}
      value={provider ? readinessLabel(provider) : 'Status unavailable'}
      tone={active ? 'good' : undefined}
    />
  );
}

function readinessLabel(provider: ResearchProviderStatus): string {
  if (provider.status === 'active') return 'Ready';
  if (provider.status === 'not_configured') return 'Setup needed';
  if (provider.status === 'unavailable') return 'Temporarily unavailable';
  return 'Status unavailable';
}
