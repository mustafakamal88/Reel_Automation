import { useState, useCallback } from 'react';
import type { Platform, Settings } from '../types';
import type { SettingsTab, PublishingAccount } from '../lib/integrations/types';
import { PLATFORMS } from '../data/platforms';
import { DATA_SOURCE_PROVIDERS, AI_PROVIDERS, PUBLISHING_ACCOUNTS } from '../lib/integrations/registry';
import { mockIntegrationService } from '../lib/integrations/mockIntegrationService';
import { mockPublishingService } from '../lib/integrations/mockPublishingService';
import { IntegrationCard } from '../components/IntegrationCard';
import { PublishingCard } from '../components/PublishingCard';

const ALL_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x', 'th', 'gt'];

const TABS: { id: SettingsTab; label: string; count?: number }[] = [
  { id: 'general', label: 'General' },
  { id: 'sources', label: 'Data Sources', count: 7 },
  { id: 'ai', label: 'AI Providers', count: 3 },
  { id: 'publishing', label: 'Publishing', count: 6 },
  { id: 'security', label: 'Security' },
];

const BACKEND_ENDPOINTS = [
  { method: 'POST', path: '/api/integrations/connect/:provider', note: 'Initiate OAuth or validate API key' },
  { method: 'POST', path: '/api/integrations/disconnect/:provider', note: 'Revoke tokens and remove credentials' },
  { method: 'GET', path: '/api/integrations/status/:provider', note: 'Return current connection status' },
  { method: 'GET', path: '/api/integrations/test/:provider', note: 'Lightweight live credential check' },
  { method: 'POST', path: '/api/publish/connect/:platform', note: 'Start OAuth for upload + publish scopes' },
  { method: 'POST', path: '/api/publish/disconnect/:platform', note: 'Revoke platform OAuth tokens' },
  { method: 'POST', path: '/api/publish/test/:platform', note: 'Upload 1s silent test video' },
  { method: 'POST', path: '/api/publish/upload', note: 'Upload and schedule video post' },
  { method: 'POST', path: '/api/ai/generate', note: 'Generate scripts/captions via AI provider' },
];

interface Props {
  settings: Settings;
  onSave: (s: Settings) => void;
}

export function SettingsPage({ settings: initial, onSave }: Props) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');

  // General settings (localStorage-backed via App.tsx)
  const [settings, setSettings] = useState<Settings>(initial);
  const [saved, setSaved] = useState(false);

  // Integration test states (in-memory — no credentials in storage)
  const [intLoading, setIntLoading] = useState<Record<string, boolean>>({});
  const [intResults, setIntResults] = useState<Record<string, string | null>>({});

  // Publishing account states (in-memory — no tokens in storage)
  const [pubAccounts, setPubAccounts] = useState<PublishingAccount[]>(PUBLISHING_ACCOUNTS);
  const [pubLoading, setPubLoading] = useState<Record<string, boolean>>({});
  const [pubResults, setPubResults] = useState<Record<string, string | null>>({});

  // ── General settings handlers ─────────────────────────────────

  function setField<K extends keyof Settings>(key: K, value: Settings[K]) {
    setSettings(prev => ({ ...prev, [key]: value }));
    setSaved(false);
  }

  function togglePlatform(p: Platform) {
    const next = settings.platforms.includes(p)
      ? settings.platforms.filter(x => x !== p)
      : [...settings.platforms, p];
    setField('platforms', next);
  }

  function handleSave() {
    onSave(settings);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  // ── Integration test handler ──────────────────────────────────

  const handleIntTest = useCallback(async (id: string) => {
    setIntLoading(prev => ({ ...prev, [id]: true }));
    setIntResults(prev => ({ ...prev, [id]: null }));
    const result = await mockIntegrationService.testConnection(id);
    setIntLoading(prev => ({ ...prev, [id]: false }));
    setIntResults(prev => ({ ...prev, [id]: result.message }));
  }, []);

  // ── Publishing account handlers ───────────────────────────────

  const handlePubConnect = useCallback(async (accountId: string) => {
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    setPubAccounts(prev => prev.map(a =>
      a.id === accountId ? { ...a, status: 'configuring' } : a
    ));
    const result = await mockPublishingService.connect(accountId);
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
    setPubAccounts(prev => prev.map(a =>
      a.id === accountId
        ? {
            ...a,
            status: result.status,
            handle: result.handle ?? null,
            uploadPermission: result.uploadPermission ?? false,
            publishPermission: result.publishPermission ?? false,
            tokenExpiry: result.tokenExpiry ?? null,
            tokenStatus: result.tokenStatus ?? 'none',
          }
        : a
    ));
    setPubResults(prev => ({ ...prev, [accountId]: result.message }));
  }, []);

  const handlePubDisconnect = useCallback(async (accountId: string) => {
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    const result = await mockPublishingService.disconnect(accountId);
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
    setPubAccounts(prev => prev.map(a =>
      a.id === accountId
        ? {
            ...a,
            status: result.status,
            handle: null,
            uploadPermission: false,
            publishPermission: false,
            tokenExpiry: null,
            tokenStatus: 'none',
          }
        : a
    ));
    setPubResults(prev => ({ ...prev, [accountId]: null }));
  }, []);

  const handlePubTestUpload = useCallback(async (accountId: string) => {
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    setPubResults(prev => ({ ...prev, [accountId]: null }));
    const result = await mockPublishingService.testUpload(accountId);
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
    setPubResults(prev => ({ ...prev, [accountId]: result.message }));
  }, []);

  // ── Render ────────────────────────────────────────────────────

  return (
    <section className="page-section">
      {/* Tab bar */}
      <div className="settings-tabs-bar">
        {TABS.map(t => (
          <button
            key={t.id}
            className={`settings-tab${activeTab === t.id ? ' active' : ''}`}
            onClick={() => setActiveTab(t.id)}
          >
            {t.label}
            {t.count !== undefined && (
              <span className="settings-tab-count">{t.count}</span>
            )}
          </button>
        ))}
      </div>

      {/* ── General ── */}
      {activeTab === 'general' && (
        <div className="settings-grid">
          <div className="settings-card">
            <div className="settings-card-title">Content &amp; Niche</div>
            <div className="form-group">
              <label className="form-label" htmlFor="niche">Your niche</label>
              <input
                id="niche"
                type="text"
                className="form-input"
                value={settings.niche}
                onChange={e => setField('niche', e.target.value)}
                placeholder="e.g. AI tools & creator workflow"
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="content-style">Content style</label>
              <select
                id="content-style"
                className="form-select"
                value={settings.contentStyle}
                onChange={e => setField('contentStyle', e.target.value)}
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
                onChange={e => setField('brandVoice', e.target.value)}
                placeholder="e.g. Direct, confident, no fluff. First-person."
              />
            </div>
          </div>

          <div className="settings-card">
            <div className="settings-card-title">Distribution &amp; Region</div>
            <div className="form-group">
              <label className="form-label" htmlFor="region">Target region</label>
              <select
                id="region"
                className="form-select"
                value={settings.region}
                onChange={e => setField('region', e.target.value)}
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
                      <span style={{
                        width: 6, height: 6, borderRadius: '50%',
                        background: active ? meta.color : 'var(--text-dimmer)',
                        display: 'inline-block',
                      }} />
                      {meta.short} — {meta.name}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="settings-card">
            <div className="settings-card-title">Risk &amp; Strategy</div>
            <div className="form-group">
              <label className="form-label" htmlFor="risk-tolerance">Risk tolerance</label>
              <select
                id="risk-tolerance"
                className="form-select"
                value={settings.riskTolerance}
                onChange={e => setField('riskTolerance', e.target.value as Settings['riskTolerance'])}
              >
                <option value="low">Low — only safe, fact-checked topics</option>
                <option value="medium">Medium — balanced (default)</option>
                <option value="high">High — include higher-risk trending topics</option>
              </select>
            </div>
            <div style={{ marginTop: 16, fontSize: 12, color: 'var(--text-dim)', lineHeight: 1.6 }}>
              Risk score thresholds:
              <br />
              <span style={{ color: '#5fd39a', fontFamily: 'var(--font-mono)', fontSize: 11 }}>Low</span> = 0–29 ·{' '}
              <span style={{ color: '#e3b54e', fontFamily: 'var(--font-mono)', fontSize: 11 }}>Med</span> = 30–54 ·{' '}
              <span style={{ color: '#e8736b', fontFamily: 'var(--font-mono)', fontSize: 11 }}>High</span> = 55+
            </div>
          </div>
        </div>
      )}
      {activeTab === 'general' && (
        <div style={{ marginTop: 20, display: 'flex', alignItems: 'center', gap: 14 }}>
          <button className="save-btn" onClick={handleSave}>
            {saved ? '✓ Saved' : 'Save settings'}
          </button>
          <span className="demo-badge">
            <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
            Settings persist in localStorage
          </span>
        </div>
      )}

      {/* ── Data Sources ── */}
      {activeTab === 'sources' && (
        <>
          <div className="int-section-header">
            <div className="int-section-title">Data Sources</div>
            <div className="int-section-sub">
              Connected platforms supply trend signals for scoring and topic generation.
              Credentials are managed server-side via <code className="int-code">/api/integrations/*</code>.
            </div>
          </div>
          <div className="integration-grid">
            {DATA_SOURCE_PROVIDERS.map(provider => (
              <IntegrationCard
                key={provider.id}
                provider={provider}
                loading={!!intLoading[provider.id]}
                testResult={intResults[provider.id] ?? null}
                onTest={() => handleIntTest(provider.id)}
                onDismissResult={() => setIntResults(prev => ({ ...prev, [provider.id]: null }))}
              />
            ))}
          </div>
        </>
      )}

      {/* ── AI Providers ── */}
      {activeTab === 'ai' && (
        <>
          <div className="int-section-header">
            <div className="int-section-title">AI Providers</div>
            <div className="int-section-sub">
              Language models used for script generation, hook writing, and caption creation.
            </div>
            <div className="security-inline-warning">
              <span className="security-warning-icon">⚠</span>
              AI provider API keys must <strong>never</strong> be stored in the browser, localStorage, or <code className="int-code">VITE_</code> variables — these are bundled into the public JS file. All key management goes through <code className="int-code">/api/ai/*</code> backend endpoints.
            </div>
          </div>
          <div className="ai-provider-grid">
            {AI_PROVIDERS.map(p => {
              const statusCls = `status-${p.status.replace('_', '-')}`;
              return (
                <div key={p.id} className="ai-provider-card">
                  <div className="ai-card-top">
                    <div className="ai-card-icon">
                      {p.id === 'anthropic' ? 'CL' : p.id === 'openai' ? 'OA' : 'LM'}
                    </div>
                    <div className="ai-card-info">
                      <div className="ai-card-name">{p.name}</div>
                      <div className="ai-card-model">
                        {p.model}
                        {p.contextWindow !== 'N/A' && (
                          <span className="ai-card-ctx"> · {p.contextWindow}</span>
                        )}
                      </div>
                    </div>
                    <span className={`status-badge ${statusCls}`}>
                      {p.status === 'demo' ? 'Demo' :
                       p.status === 'connected' ? 'Connected' :
                       p.status === 'not_connected' ? 'Not connected' : p.status}
                    </span>
                  </div>

                  <div className="ai-cost-warning">
                    <span style={{ flexShrink: 0, fontSize: 13 }}>
                      {p.secretStorageMode === 'none' ? 'ℹ' : '🔐'}
                    </span>
                    <span>{p.costNote}</span>
                  </div>

                  <div className="ai-card-footer">
                    <code className="int-code int-code--endpoint">{p.backendEndpoint}</code>
                    {p.status === 'not_connected' && (
                      <button
                        className="btn-int btn-int--primary"
                        onClick={() => {
                          alert(`Production: Configure ${p.name} API key on your backend server.\nEndpoint: ${p.backendEndpoint}\nKey must be stored encrypted — never in VITE_ vars or the browser.`);
                        }}
                      >
                        Configure
                      </button>
                    )}
                    {p.status === 'demo' && (
                      <button
                        className="btn-int"
                        onClick={() => {
                          alert(`Running in demo mode.\nTo switch to live: add ${p.name} API key to your backend environment and point ${p.backendEndpoint} at the real API.`);
                        }}
                      >
                        Configure
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </>
      )}

      {/* ── Publishing ── */}
      {activeTab === 'publishing' && (
        <>
          <div className="int-section-header">
            <div className="int-section-title">Publishing Accounts</div>
            <div className="int-section-sub">
              OAuth connections for direct video upload and scheduling.
            </div>
            <div className="security-inline-warning">
              <span className="security-warning-icon">⚠</span>
              OAuth refresh tokens are stored <strong>server-side only</strong> — the frontend never receives them. Connect/disconnect actions call <code className="int-code">/api/publish/*</code> and receive connection status only.
            </div>
          </div>
          <div className="pub-grid">
            {pubAccounts.map(account => (
              <PublishingCard
                key={account.id}
                account={account}
                loading={!!pubLoading[account.id]}
                testResult={pubResults[account.id] ?? null}
                onConnect={() => handlePubConnect(account.id)}
                onDisconnect={() => handlePubDisconnect(account.id)}
                onTestUpload={() => handlePubTestUpload(account.id)}
                onDismissResult={() => setPubResults(prev => ({ ...prev, [account.id]: null }))}
              />
            ))}
          </div>
        </>
      )}

      {/* ── Security ── */}
      {activeTab === 'security' && (
        <>
          <div className="int-section-header">
            <div className="int-section-title">Secret Management</div>
            <div className="int-section-sub">
              How credentials are handled in SIGNAL — current and production rules.
            </div>
          </div>

          <div className="security-rules-card">
            {[
              {
                icon: '🚫',
                title: 'Secrets never stored in the browser',
                body: 'No API keys, OAuth tokens, refresh tokens, or client secrets are stored in localStorage, sessionStorage, cookies, or React state. VITE_ environment variables are compiled into the public JavaScript bundle and must never hold sensitive values.',
              },
              {
                icon: '🔐',
                title: 'Production secrets → encrypted backend storage',
                body: 'All credentials must be stored server-side, encrypted at rest (e.g. AWS Secrets Manager, HashiCorp Vault, or an encrypted DB column). The frontend only receives connection status — never raw tokens.',
              },
              {
                icon: '🖥',
                title: 'Local development',
                body: 'For local backend development, use a server-side .env file with non-VITE_ prefixed variables. These stay on the server process and are never bundled into the frontend asset.',
              },
              {
                icon: '🎭',
                title: 'Current demo mode',
                body: 'SIGNAL is running with mock providers only. No real API calls are made. All connection statuses shown in the Integrations UI are simulated for demonstration. No real credentials are needed or accepted at this stage.',
              },
              {
                icon: '🔄',
                title: 'OAuth token refresh',
                body: 'Token refresh happens entirely server-side on a cron schedule. The frontend triggers refresh via POST /api/publish/refresh/:platform but never sees the new token — only a success/expiry status response.',
              },
            ].map(rule => (
              <div key={rule.title} className="security-rule">
                <div className="security-rule-icon">{rule.icon}</div>
                <div className="security-rule-body">
                  <div className="security-rule-title">{rule.title}</div>
                  <div className="security-rule-desc">{rule.body}</div>
                </div>
              </div>
            ))}
          </div>

          <div className="int-section-title" style={{ marginTop: 28, marginBottom: 14 }}>
            Backend API Surface
          </div>
          <div className="security-endpoints-card">
            {BACKEND_ENDPOINTS.map(e => (
              <div key={e.path} className="security-endpoint-row">
                <span className={`http-method http-method--${e.method.toLowerCase()}`}>{e.method}</span>
                <code className="int-code security-endpoint-path">{e.path}</code>
                <span className="security-endpoint-note">{e.note}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </section>
  );
}
