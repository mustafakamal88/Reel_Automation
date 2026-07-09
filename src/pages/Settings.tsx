import type { Platform, Settings } from '../types';
import { PLATFORMS } from '../data/platforms';

const CREATOR_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x'];

interface Props {
  settings: Settings;
  onSave: (s: Settings) => void;
}

export function SettingsPage({ settings: initial, onSave }: Props) {
  const settings = initial;

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
          <div className="settings-card-title">Product Status</div>
          <div style={{ display: 'grid', gap: 10 }}>
            <StatusRow label="OpenAI" value="Configured when backend OPENAI_API_KEY is present" />
            <StatusRow label="Trend source" value="Google Trends RSS through backend discovery" />
            <StatusRow label="Social publishing" value="Not connected until OAuth/API setup exists" />
            <StatusRow label="Workspace" value="Local browser preferences only" />
          </div>
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
          <div className="settings-card-title">Publishing Guardrails</div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', lineHeight: 1.7 }}>
            Uploads are disabled until platform OAuth and API publishing support are configured. Manual ZIP downloads stay inside Clip Generator.
          </div>
        </div>
      </div>
    </section>
  );
}

function StatusRow({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ border: '1px solid var(--border-card)', background: 'var(--bg-subtle)', borderRadius: 8, padding: 12 }}>
      <div style={{ fontSize: 12, fontWeight: 800, color: 'var(--text-primary)' }}>{label}</div>
      <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 5, lineHeight: 1.5 }}>{value}</div>
    </div>
  );
}
