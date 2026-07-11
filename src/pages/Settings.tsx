import { useEffect, useMemo, useState, type MouseEvent } from 'react';
import type { Platform, Settings } from '../types';
import { PLATFORMS } from '../data/platforms';
import { getResearchProviderStatus, type ResearchProviderStatus } from '../lib/api/client';

interface Props {
  settings: Settings;
  onSave: (s: Settings) => void;
}

type SettingsSection = 'workspace' | 'ai' | 'publishing' | 'branding' | 'billing' | 'team';
type SaveState = 'idle' | 'dirty' | 'saving' | 'saved' | 'error';

const SETTINGS_SECTIONS: { id: SettingsSection; label: string }[] = [
  { id: 'workspace', label: 'Workspace' },
  { id: 'ai', label: 'AI' },
  { id: 'publishing', label: 'Publishing' },
  { id: 'branding', label: 'Branding' },
  { id: 'billing', label: 'Billing' },
  { id: 'team', label: 'Team' },
];

const CREATOR_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x'];

export function SettingsPage({ settings: initial, onSave }: Props) {
  const [draft, setDraft] = useState(initial);
  const [activeSection, setActiveSection] = useState<SettingsSection>('workspace');
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [providers, setProviders] = useState<ResearchProviderStatus[]>([]);
  const [providerError, setProviderError] = useState<string | null>(null);

  useEffect(() => {
    setDraft(initial);
    setSaveState(current => (current === 'saved' ? 'saved' : 'idle'));
  }, [initial]);

  useEffect(() => {
    let cancelled = false;
    getResearchProviderStatus()
      .then(data => {
        if (!cancelled) {
          setProviders(data.providers);
          setProviderError(null);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProviders([]);
          setProviderError('AI provider status is unavailable while the backend cannot be reached.');
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const hasChanges = useMemo(() => JSON.stringify(draft) !== JSON.stringify(initial), [draft, initial]);

  function setField<K extends keyof Settings>(key: K, value: Settings[K]) {
    setDraft(current => ({ ...current, [key]: value }));
    setSaveState('dirty');
  }

  function togglePublishingPlatform(platform: Platform) {
    const current = draft.publishingPlatforms ?? [];
    const publishingPlatforms = current.includes(platform)
      ? current.filter(item => item !== platform)
      : [...current, platform];
    setField('publishingPlatforms', publishingPlatforms);
  }

  function saveSettings() {
    setSaveState('saving');
    try {
      onSave(draft);
      setSaveState('saved');
    } catch {
      setSaveState('error');
    }
  }

  return (
    <section className="page-section settings-page">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Settings</div>
          <h1>Configure the creator workspace.</h1>
          <p>Manage creator preferences, brand defaults, and publishing defaults without exposing secrets.</p>
        </div>
      </div>

      <div className="settings-actions-bar">
        <div>
          <strong>Workspace settings</strong>
          <span>{settingsSaveMessage(saveState, hasChanges)}</span>
        </div>
        <button className="generate-btn idle" type="button" onClick={saveSettings} disabled={!hasChanges || saveState === 'saving'}>
          {saveState === 'saving' ? 'Saving' : 'Save Settings'}
        </button>
      </div>

      <div className="neutral-callout settings-persistence-note">
        Settings are saved in this browser. Server-side workspace settings persistence is not available in this build.
      </div>

      <div className="settings-workspace">
        <div className="settings-tabs-bar" role="tablist" aria-label="Settings sections">
          {SETTINGS_SECTIONS.map(section => (
            <button
              key={section.id}
              type="button"
              className={`settings-tab${activeSection === section.id ? ' active' : ''}`}
              role="tab"
              aria-selected={activeSection === section.id}
              onClick={() => setActiveSection(section.id)}
            >
              {section.label}
              {(section.id === 'billing' || section.id === 'team') && <span className="settings-tab-count">Soon</span>}
            </button>
          ))}
        </div>

        {activeSection === 'workspace' && <WorkspaceSettings draft={draft} setField={setField} />}
        {activeSection === 'ai' && <AISettings providers={providers} providerError={providerError} />}
        {activeSection === 'publishing' && (
          <PublishingSettings
            draft={draft}
            setField={setField}
            onTogglePlatform={togglePublishingPlatform}
          />
        )}
        {activeSection === 'branding' && <BrandingSettings draft={draft} setField={setField} />}
        {activeSection === 'billing' && <ComingSoonSettings title="Billing" description="Plan management and invoices are not available in this build." />}
        {activeSection === 'team' && <ComingSoonSettings title="Team" description="Team seats, roles, and invitations are not available in this build." />}
      </div>
    </section>
  );
}

function WorkspaceSettings({ draft, setField }: { draft: Settings; setField: <K extends keyof Settings>(key: K, value: Settings[K]) => void }) {
  return (
    <div className="settings-grid creator-settings-grid">
      <section className="settings-card">
        <div className="settings-card-title">Creator context</div>
        <p className="settings-section-desc">Defaults that shape research, script tone, and creator context.</p>
        <div className="form-group">
          <label className="form-label" htmlFor="niche">Creator niche</label>
          <input id="niche" className="form-input" value={draft.niche} onChange={event => setField('niche', event.target.value)} />
        </div>
        <div className="form-grid two">
          <div className="form-group">
            <label className="form-label" htmlFor="region">Default region</label>
            <input id="region" className="form-input" value={draft.region} onChange={event => setField('region', event.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="language">Language</label>
            <input id="language" className="form-input" value={draft.language} onChange={event => setField('language', event.target.value)} />
          </div>
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="audience-culture">Audience/culture</label>
          <input id="audience-culture" className="form-input" value={draft.audienceCulture} onChange={event => setField('audienceCulture', event.target.value)} />
        </div>
      </section>

      <section className="settings-card">
        <div className="settings-card-title">Style and voice</div>
        <p className="settings-section-desc">Creative defaults used when generating scripts and clip direction.</p>
        <div className="form-group">
          <label className="form-label" htmlFor="content-style">Content style</label>
          <input id="content-style" className="form-input" value={draft.contentStyle} onChange={event => setField('contentStyle', event.target.value)} />
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="brand-voice">Brand voice</label>
          <textarea id="brand-voice" className="form-textarea" value={draft.brandVoice} onChange={event => setField('brandVoice', event.target.value)} />
        </div>
      </section>
    </div>
  );
}

function AISettings({ providers, providerError }: { providers: ResearchProviderStatus[]; providerError: string | null }) {
  const openAIProvider = providers.find(provider => provider.id === 'openai');
  const ready = openAIProvider?.status === 'active';

  return (
    <div className="settings-grid creator-settings-grid">
      <section className="settings-card">
        <div className="settings-card-title">AI provider status</div>
        <p className="settings-section-desc">Only safe readiness states are shown here. API keys and secrets are never displayed in the browser.</p>
        {providerError && <div className="neutral-callout">{providerError}</div>}
        <div className="status-list">
          <StatusRow
            label="OpenAI generation"
            value={ready ? 'Connected' : 'Checked server-side during generation'}
            tone={ready ? 'good' : undefined}
          />
        </div>
      </section>

      <section className="settings-card">
        <div className="settings-card-title">Model selection</div>
        <p className="settings-section-desc">Model/provider selection is not exposed because this frontend does not have a supported model configuration endpoint.</p>
        <div className="empty-state inline-empty-state">
          <div className="empty-icon">AI</div>
          <div className="empty-title">Selection unavailable</div>
          <div className="empty-desc">Generation uses the backend configuration for this environment.</div>
        </div>
      </section>
    </div>
  );
}

function PublishingSettings({ draft, setField, onTogglePlatform }: {
  draft: Settings;
  setField: <K extends keyof Settings>(key: K, value: Settings[K]) => void;
  onTogglePlatform: (platform: Platform) => void;
}) {
  return (
    <div className="settings-grid creator-settings-grid">
      <section className="settings-card">
        <div className="settings-card-title">Publishing defaults</div>
        <p className="settings-section-desc">These are creator preferences. OAuth setup and account management stay on Connections.</p>
        <div className="platform-toggles settings-platform-toggles" aria-label="Default publishing platforms">
          {CREATOR_PLATFORMS.map(platform => {
            const meta = PLATFORMS[platform];
            const active = draft.publishingPlatforms.includes(platform);
            return (
              <button
                key={platform}
                type="button"
                className="platform-toggle"
                onClick={() => onTogglePlatform(platform)}
                aria-pressed={active}
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
        <div className="settings-link-panel">
          <span>Connect or disconnect accounts from Connections.</span>
          <a className="link-button" href="/connections" onClick={navigateWithHistory}>Open Connections</a>
        </div>
      </section>

      <section className="settings-card">
        <div className="settings-card-title">Review and scheduling</div>
        <p className="settings-section-desc">Defaults used when publishing workflows become available.</p>
        <div className="form-group">
          <label className="form-label" htmlFor="approval-behavior">Approval behaviour</label>
          <select id="approval-behavior" className="form-input" value={draft.approvalBehavior} onChange={event => setField('approvalBehavior', event.target.value as Settings['approvalBehavior'])}>
            <option value="manual_review">Require review before publishing</option>
            <option value="auto_approve">Auto-approve where supported</option>
          </select>
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="schedule-timezone">Scheduling/timezone default</label>
          <input id="schedule-timezone" className="form-input" value={draft.schedulingTimezone} onChange={event => setField('schedulingTimezone', event.target.value)} />
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="default-visibility">Default visibility</label>
          <select id="default-visibility" className="form-input" value={draft.defaultVisibility} onChange={event => setField('defaultVisibility', event.target.value as Settings['defaultVisibility'])}>
            <option value="private">Private</option>
            <option value="unlisted">Unlisted</option>
            <option value="public">Public</option>
          </select>
        </div>
      </section>
    </div>
  );
}

function BrandingSettings({ draft, setField }: { draft: Settings; setField: <K extends keyof Settings>(key: K, value: Settings[K]) => void }) {
  return (
    <div className="settings-grid creator-settings-grid">
      <section className="settings-card">
        <div className="settings-card-title">Clip overlays</div>
        <p className="settings-section-desc">Defaults used by Clip Generator when preparing branded clips.</p>
        <div className="form-group">
          <label className="form-label" htmlFor="default-watermark">Watermark/channel name</label>
          <input id="default-watermark" className="form-input" value={draft.defaultWatermark} onChange={event => setField('defaultWatermark', event.target.value)} />
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="default-top">Top banner text</label>
          <input id="default-top" className="form-input" value={draft.defaultTopText} onChange={event => setField('defaultTopText', event.target.value)} />
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="default-bottom">Bottom banner text</label>
          <input id="default-bottom" className="form-input" value={draft.defaultBottomText} onChange={event => setField('defaultBottomText', event.target.value)} />
        </div>
        <div className="form-group">
          <label className="form-label" htmlFor="default-layout">Default clip layout</label>
          <select id="default-layout" className="form-input" value={draft.defaultLayoutMode} onChange={event => setField('defaultLayoutMode', event.target.value as Settings['defaultLayoutMode'])}>
            <option value="blurred_background">Blurred background</option>
            <option value="fill_crop">Fill crop</option>
            <option value="fit_with_bars">Fit with bars</option>
          </select>
        </div>
      </section>

      <section className="settings-card">
        <div className="settings-card-title">Brand assets and colours</div>
        <p className="settings-section-desc">Logo upload is not supported here yet. Colour defaults are stored for supported branded clip rendering.</p>
        <div className="form-group">
          <label className="form-label" htmlFor="logo-asset">Logo/brand asset</label>
          <input id="logo-asset" className="form-input" value={draft.logoAssetName} placeholder="Logo upload unavailable" onChange={event => setField('logoAssetName', event.target.value)} />
        </div>
        <div className="brand-color-grid">
          <ColorField id="default-brand-color" label="Brand colour" value={draft.defaultBrandColor} onChange={value => setField('defaultBrandColor', value)} />
          <ColorField id="default-top-color" label="Top banner colour" value={draft.defaultTopColor} onChange={value => setField('defaultTopColor', value)} />
          <ColorField id="default-bottom-color" label="Bottom banner colour" value={draft.defaultBottomColor} onChange={value => setField('defaultBottomColor', value)} />
        </div>
      </section>
    </div>
  );
}

function ColorField({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="form-group">
      <label className="form-label" htmlFor={id}>{label}</label>
      <div className="color-input-row">
        <input aria-label={`${label} picker`} type="color" value={validColor(value)} onChange={event => onChange(event.target.value)} />
        <input id={id} className="form-input" value={value} onChange={event => onChange(event.target.value)} />
      </div>
    </div>
  );
}

function ComingSoonSettings({ title, description }: { title: string; description: string }) {
  return (
    <section className="settings-card coming-soon-card">
      <div className="coming-soon-badge">Coming soon</div>
      <div className="settings-card-title">{title}</div>
      <p className="settings-section-desc">{description}</p>
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

function navigateWithHistory(event: MouseEvent<HTMLAnchorElement>) {
  event.preventDefault();
  window.history.pushState(null, '', '/connections');
  window.dispatchEvent(new PopStateEvent('popstate'));
}

function validColor(value: string): string {
  return /^#[0-9a-f]{6}$/i.test(value) ? value : '#39c7d6';
}

function settingsSaveMessage(state: SaveState, hasChanges: boolean): string {
  if (state === 'saving') return 'Saving changes.';
  if (state === 'error') return 'Settings could not be saved.';
  if (hasChanges || state === 'dirty') return 'Unsaved changes.';
  if (state === 'saved') return 'Saved in this browser.';
  return 'No unsaved changes.';
}
