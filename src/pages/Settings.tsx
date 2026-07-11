import type { Settings } from '../types';
import { PlatformSelector } from '../components/PlatformSelector';

interface Props {
  settings: Settings;
  onSave: (settings: Settings) => void;
}

export function SettingsPage({ settings, onSave }: Props) {
  function setField<K extends keyof Settings>(key: K, value: Settings[K]) {
    onSave({ ...settings, [key]: value });
  }

  return (
    <section className="page-section settings-page">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Settings</div>
          <h1>Shape your creator workspace.</h1>
          <p>Set the defaults used when researching trends, writing scripts, and packaging clips.</p>
        </div>
      </div>

      <div className="settings-product-grid">
        <section className="settings-card settings-card-primary">
          <div className="settings-card-header">
            <div>
              <div className="settings-card-title">Workspace</div>
              <div className="muted-note">Your default research and content context.</div>
            </div>
          </div>

          <div className="settings-form-grid">
            <div className="form-group">
              <label className="form-label" htmlFor="niche">Creator niche</label>
              <input
                id="niche"
                className="form-input"
                value={settings.niche}
                onChange={event => setField('niche', event.target.value)}
                placeholder="AI tools, football, business, education"
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="region">Default market</label>
              <input
                id="region"
                className="form-input"
                value={settings.region}
                onChange={event => setField('region', event.target.value)}
                placeholder="UK · English"
              />
            </div>

            <div className="form-group settings-form-span">
              <label className="form-label" htmlFor="content-style">Content style</label>
              <input
                id="content-style"
                className="form-input"
                value={settings.contentStyle}
                onChange={event => setField('contentStyle', event.target.value)}
                placeholder="Educational, entertaining, direct"
              />
            </div>

            <div className="form-group settings-form-span">
              <label className="form-label" htmlFor="brand-voice">Brand voice</label>
              <textarea
                id="brand-voice"
                className="form-textarea"
                value={settings.brandVoice}
                onChange={event => setField('brandVoice', event.target.value)}
                placeholder="Direct, confident, clear, and useful."
              />
            </div>
          </div>
        </section>

        <section className="settings-card settings-card-primary">
          <div className="settings-card-header">
            <div>
              <div className="settings-card-title">Clip branding</div>
              <div className="muted-note">Reusable defaults for generated clip packages.</div>
            </div>
          </div>

          <div className="settings-form-grid single">
            <div className="form-group">
              <label className="form-label" htmlFor="default-top">Default top text</label>
              <input
                id="default-top"
                className="form-input"
                value={settings.defaultTopText}
                onChange={event => setField('defaultTopText', event.target.value)}
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="default-bottom">Default call to action</label>
              <input
                id="default-bottom"
                className="form-input"
                value={settings.defaultBottomText}
                onChange={event => setField('defaultBottomText', event.target.value)}
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="default-watermark">Watermark or channel name</label>
              <input
                id="default-watermark"
                className="form-input"
                value={settings.defaultWatermark}
                onChange={event => setField('defaultWatermark', event.target.value)}
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="default-layout">Default video layout</label>
              <select
                id="default-layout"
                className="form-input"
                value={settings.defaultLayoutMode}
                onChange={event => setField('defaultLayoutMode', event.target.value as Settings['defaultLayoutMode'])}
              >
                <option value="blurred_background">Blurred background</option>
                <option value="fill_crop">Fill crop</option>
                <option value="fit_with_bars">Fit with bars</option>
              </select>
            </div>
          </div>
        </section>

        <section className="settings-card settings-card-wide">
          <PlatformSelector
            value={settings.platforms}
            onChange={platforms => setField('platforms', platforms)}
            label="Target platforms"
            description="Choose where new scripts and clip packages should be prepared by default. Account connections are managed separately."
          />
        </section>

        <section className="settings-card settings-card-wide settings-info-strip">
          <div>
            <div className="settings-card-title">Publishing and account access</div>
            <div className="muted-note">Connect real social accounts from Connections. Provider diagnostics and API health remain internal and are not shown in the creator workspace.</div>
          </div>
        </section>
      </div>
    </section>
  );
}
