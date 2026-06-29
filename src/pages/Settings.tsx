import { useState } from 'react';
import type { Platform, Settings } from '../types';
import { PLATFORMS } from '../data/platforms';

const ALL_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x', 'th', 'gt'];

interface Props {
  settings: Settings;
  onSave: (s: Settings) => void;
}

export function SettingsPage({ settings: initial, onSave }: Props) {
  const [settings, setSettings] = useState<Settings>(initial);
  const [saved, setSaved] = useState(false);

  function set<K extends keyof Settings>(key: K, value: Settings[K]) {
    setSettings(prev => ({ ...prev, [key]: value }));
    setSaved(false);
  }

  function togglePlatform(p: Platform) {
    const current = settings.platforms;
    const next = current.includes(p) ? current.filter(x => x !== p) : [...current, p];
    set('platforms', next);
  }

  function handleSave() {
    onSave(settings);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  return (
    <section className="page-section">
      <div className="settings-grid">
        {/* Content & Niche */}
        <div className="settings-card">
          <div className="settings-card-title">Content &amp; Niche</div>

          <div className="form-group">
            <label className="form-label" htmlFor="niche">Your niche</label>
            <input
              id="niche"
              type="text"
              className="form-input"
              value={settings.niche}
              onChange={e => set('niche', e.target.value)}
              placeholder="e.g. AI tools & creator workflow"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="content-style">Content style</label>
            <select
              id="content-style"
              className="form-select"
              value={settings.contentStyle}
              onChange={e => set('contentStyle', e.target.value)}
            >
              <option>Educational + entertaining (edutainment)</option>
              <option>Purely educational</option>
              <option>Entertainment first</option>
              <option>News / commentary</option>
              <option>Tutorial / how-to</option>
              <option>Reaction / opinion</option>
            </select>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="brand-voice">Brand voice</label>
            <textarea
              id="brand-voice"
              className="form-textarea"
              value={settings.brandVoice}
              onChange={e => set('brandVoice', e.target.value)}
              placeholder="e.g. Direct, confident, no fluff. First-person."
            />
          </div>
        </div>

        {/* Distribution & Region */}
        <div className="settings-card">
          <div className="settings-card-title">Distribution &amp; Region</div>

          <div className="form-group">
            <label className="form-label" htmlFor="region">Target region</label>
            <select
              id="region"
              className="form-select"
              value={settings.region}
              onChange={e => set('region', e.target.value)}
            >
              <option>US · Global</option>
              <option>US only</option>
              <option>UK</option>
              <option>EU</option>
              <option>APAC</option>
              <option>LATAM</option>
            </select>
          </div>

          <div className="form-group">
            <label className="form-label">Active platforms</label>
            <div className="platform-toggles">
              {ALL_PLATFORMS.map(p => {
                const meta = PLATFORMS[p];
                const active = settings.platforms.includes(p);
                return (
                  <button
                    key={p}
                    className="platform-toggle"
                    onClick={() => togglePlatform(p)}
                    aria-pressed={active}
                    style={{
                      background: active ? meta.bg : 'var(--bg-subtle)',
                      borderColor: active ? meta.color : 'var(--border-strong)',
                      color: active ? meta.color : 'var(--text-dim)',
                      fontFamily: 'inherit',
                      cursor: 'pointer',
                    }}
                  >
                    <span
                      style={{
                        width: 6,
                        height: 6,
                        borderRadius: '50%',
                        background: active ? meta.color : 'var(--text-dimmer)',
                        display: 'inline-block',
                      }}
                    />
                    {meta.short} — {meta.name}
                  </button>
                );
              })}
            </div>
          </div>
        </div>

        {/* Risk & Strategy */}
        <div className="settings-card">
          <div className="settings-card-title">Risk &amp; Strategy</div>

          <div className="form-group">
            <label className="form-label" htmlFor="risk-tolerance">Risk tolerance</label>
            <select
              id="risk-tolerance"
              className="form-select"
              value={settings.riskTolerance}
              onChange={e => set('riskTolerance', e.target.value as Settings['riskTolerance'])}
            >
              <option value="low">Low — only safe, fact-checked topics</option>
              <option value="medium">Medium — balanced (default)</option>
              <option value="high">High — include higher-risk trending topics</option>
            </select>
          </div>

          <div style={{ marginTop: 16 }}>
            <div style={{ fontSize: 12, color: 'var(--text-dim)', lineHeight: 1.6 }}>
              Risk score thresholds:
              <br />
              <span style={{ color: '#5fd39a', fontFamily: 'var(--font-mono)', fontSize: 11 }}>Low</span> = 0–29 ·{' '}
              <span style={{ color: '#e3b54e', fontFamily: 'var(--font-mono)', fontSize: 11 }}>Med</span> = 30–54 ·{' '}
              <span style={{ color: '#e8736b', fontFamily: 'var(--font-mono)', fontSize: 11 }}>High</span> = 55+
            </div>
          </div>
        </div>

        {/* API Integrations */}
        <div className="settings-card">
          <div className="settings-card-title">API Integrations</div>
          <div style={{ fontSize: 12, color: 'var(--text-dim)', lineHeight: 1.7 }}>
            All providers are running in <strong style={{ color: 'var(--text-muted)' }}>demo mode</strong>.
            <br /><br />
            To connect real data sources, add API keys to your environment:
          </div>
          {[
            { key: 'VITE_YOUTUBE_API_KEY', label: 'YouTube Data API v3' },
            { key: 'VITE_TIKTOK_CLIENT_KEY', label: 'TikTok Research API' },
            { key: 'VITE_META_ACCESS_TOKEN', label: 'Meta Graph API (IG + FB)' },
            { key: 'VITE_X_BEARER_TOKEN', label: 'X API v2' },
            { key: 'VITE_ANTHROPIC_API_KEY', label: 'Anthropic (content gen)' },
          ].map(({ key, label }) => (
            <div
              key={key}
              style={{
                marginTop: 10,
                padding: '8px 12px',
                background: 'var(--bg-subtle)',
                border: '1px solid var(--border-strong)',
                borderRadius: 'var(--radius-sm)',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>
                {key}
              </span>
              <span style={{ fontSize: 10, color: 'var(--text-dim)' }}>{label}</span>
            </div>
          ))}
        </div>
      </div>

      <div style={{ marginTop: 20, display: 'flex', alignItems: 'center', gap: 14 }}>
        <button className="save-btn" onClick={handleSave}>
          {saved ? '✓ Saved' : 'Save settings'}
        </button>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Settings persist in localStorage
        </span>
      </div>
    </section>
  );
}
