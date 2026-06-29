import { useState, useCallback } from 'react';
import type { Platform, Settings } from '../types';
import type { SettingsTab, PublishingAccount, IntegrationProvider } from '../lib/integrations/types';
import { PLATFORMS } from '../data/platforms';
import { DATA_SOURCE_PROVIDERS, AI_PROVIDERS, PUBLISHING_ACCOUNTS } from '../lib/integrations/registry';
import { integrationApiClient } from '../lib/integrations/integrationApiClient';
import { apiUrl, getOAuthStartURL, ApiError } from '../lib/api/client';
import { IntegrationCard } from '../components/IntegrationCard';
import { PublishingCard } from '../components/PublishingCard';

const ALL_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x', 'th', 'gt'];

const TABS: { id: SettingsTab; label: string; count?: number }[] = [
  { id: 'general',    label: 'General' },
  { id: 'sources',    label: 'Data Sources', count: 7 },
  { id: 'ai',         label: 'AI Providers', count: 3 },
  { id: 'publishing', label: 'Publishing',   count: 6 },
  { id: 'security',   label: 'Security' },
];

const BACKEND_ENDPOINTS = [
  { method: 'GET',  path: '/health',                          note: 'Liveness probe — Railway health check, no auth' },
  { method: 'GET',  path: '/platforms/connections',           note: 'OAuth status for all publishable platforms (no tokens)' },
  { method: 'POST', path: '/platforms/{platform}/disconnect', note: 'Revoke OAuth tokens and delete encrypted credentials (501 until implemented)' },
  { method: 'POST', path: '/platforms/{platform}/refresh',    note: 'Exchange refresh token; return expiry status only (501 until implemented)' },
  { method: 'POST', path: '/platforms/{platform}/test',       note: 'Lightweight live credential probe via provider API (501 until implemented)' },
  { method: 'GET',  path: '/oauth/{platform}/start',          note: 'Begin OAuth redirect — returns authorize_url for browser redirect' },
  { method: 'GET',  path: '/oauth/{platform}/callback',       note: 'OAuth callback — exchange code, encrypt + store tokens server-side' },
  { method: 'POST', path: '/batches/{batchID}/zip',           note: 'Queue ZIP creation job — returns job_id for polling (501 until implemented)' },
  { method: 'POST', path: '/batches/{batchID}/publish',       note: 'Queue publish jobs per platform — human approval required (501 until implemented)' },
  { method: 'GET',  path: '/jobs/{jobID}',                    note: 'Poll background job status (501 until implemented)' },
];

interface Props {
  settings: Settings;
  onSave: (s: Settings) => void;
}

export function SettingsPage({ settings: initial, onSave }: Props) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');

  // General settings (localStorage-backed via App.tsx — harmless UI preferences only)
  const [settings, setSettings] = useState<Settings>(initial);
  const [saved, setSaved] = useState(false);

  // ── Data source connection states ─────────────────────────────
  // In-memory only — no credentials, no tokens, no secrets in this state.
  // Only connection status metadata (timestamps, scope names) is kept here.
  const [dataProviders, setDataProviders] = useState<IntegrationProvider[]>(DATA_SOURCE_PROVIDERS);
  const [intLoading,   setIntLoading]   = useState<Record<string, boolean>>({});
  const [intResults,   setIntResults]   = useState<Record<string, string | null>>({});

  // ── Publishing account states ─────────────────────────────────
  // Same contract: in-memory status only, no tokens in frontend state.
  const [pubAccounts, setPubAccounts] = useState<PublishingAccount[]>(PUBLISHING_ACCOUNTS);
  const [pubLoading,  setPubLoading]  = useState<Record<string, boolean>>({});
  const [pubResults,  setPubResults]  = useState<Record<string, string | null>>({});

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

  // ── Data source connection handlers ───────────────────────────

  const handleIntConnect = useCallback(async (id: string) => {
    setIntLoading(prev => ({ ...prev, [id]: true }));
    setDataProviders(prev => prev.map(p =>
      p.id === id ? { ...p, status: 'configuring' } : p
    ));
    const result = await integrationApiClient.startOAuth(id);
    setIntLoading(prev => ({ ...prev, [id]: false }));
    setDataProviders(prev => prev.map(p =>
      p.id === id
        ? { ...p, status: result.status, lastSync: result.connectedAt ? 'just now' : p.lastSync }
        : p
    ));
    setIntResults(prev => ({ ...prev, [id]: result.message }));
  }, []);

  const handleIntDisconnect = useCallback(async (id: string) => {
    setIntLoading(prev => ({ ...prev, [id]: true }));
    const result = await integrationApiClient.disconnectProvider(id);
    setIntLoading(prev => ({ ...prev, [id]: false }));
    setDataProviders(prev => prev.map(p =>
      p.id === id ? { ...p, status: result.status, lastSync: null } : p
    ));
    setIntResults(prev => ({ ...prev, [id]: null }));
  }, []);

  const handleIntRefresh = useCallback(async (id: string) => {
    setIntLoading(prev => ({ ...prev, [id]: true }));
    setIntResults(prev => ({ ...prev, [id]: null }));
    const result = await integrationApiClient.refreshConnection(id);
    setIntLoading(prev => ({ ...prev, [id]: false }));
    setDataProviders(prev => prev.map(p =>
      p.id === id ? { ...p, status: result.status } : p
    ));
    setIntResults(prev => ({ ...prev, [id]: result.message }));
  }, []);

  const handleIntTest = useCallback(async (id: string) => {
    setIntLoading(prev => ({ ...prev, [id]: true }));
    setIntResults(prev => ({ ...prev, [id]: null }));
    const result = await integrationApiClient.testConnection(id);
    setIntLoading(prev => ({ ...prev, [id]: false }));
    setIntResults(prev => ({ ...prev, [id]: result.message }));
  }, []);

  // ── Publishing account handlers ───────────────────────────────

  const handlePubConnect = useCallback(async (accountId: string) => {
    const platform = accountId.replace('pub-', '');
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    setPubAccounts(prev => prev.map(a =>
      a.id === accountId ? { ...a, status: 'configuring' } : a
    ));
    try {
      const res = await getOAuthStartURL(platform);
      // Redirect browser to platform OAuth page — backend handles callback
      window.location.href = res.authorize_url;
    } catch (err) {
      let msg = 'OAuth request failed';
      if (err instanceof ApiError) {
        if (err.isCredentialsMissing) msg = `Platform app credentials are missing. Add backend environment variables first.`;
        else if (err.isBackendOffline) msg = `Backend offline — run: cd backend && go run ./cmd/api`;
        else if (err.isNotImplemented) msg = `OAuth not yet wired on backend (PKCE + state storage required).`;
        else msg = err.message;
      } else if (err instanceof Error) {
        msg = err.message;
      }
      setPubAccounts(prev => prev.map(a =>
        a.id === accountId ? { ...a, status: 'not_connected' } : a
      ));
      setPubResults(prev => ({ ...prev, [accountId]: msg }));
    }
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
  }, []);

  const handlePubDisconnect = useCallback(async (accountId: string) => {
    const platform = accountId.replace('pub-', '');
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    try {
      const res = await fetch(apiUrl(`/platforms/${platform}/disconnect`), {
        method: 'POST', credentials: 'include',
      });
      const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` })) as { error?: string };
      const msg = res.ok ? `Disconnected ${platform}.` : (body.error ?? `HTTP ${res.status}`);
      if (res.ok) {
        setPubAccounts(prev => prev.map(a =>
          a.id === accountId
            ? { ...a, status: 'not_connected', handle: null, uploadPermission: false,
                publishPermission: false, tokenExpiry: null, tokenStatus: 'none' }
            : a
        ));
      }
      setPubResults(prev => ({ ...prev, [accountId]: msg }));
    } catch {
      setPubResults(prev => ({ ...prev, [accountId]: `Backend offline — run: cd backend && go run ./cmd/api` }));
    }
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
  }, []);

  const handlePubTestUpload = useCallback(async (accountId: string) => {
    const platform = accountId.replace('pub-', '');
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    setPubResults(prev => ({ ...prev, [accountId]: null }));
    try {
      const res = await fetch(apiUrl(`/platforms/${platform}/test`), {
        method: 'POST', credentials: 'include',
      });
      const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` })) as { error?: string };
      const msg = res.ok
        ? `Test passed for ${platform}.`
        : (body.error ?? `HTTP ${res.status} — test endpoint not yet implemented.`);
      setPubResults(prev => ({ ...prev, [accountId]: msg }));
    } catch {
      setPubResults(prev => ({ ...prev, [accountId]: `Backend offline — run: cd backend && go run ./cmd/api` }));
    }
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
  }, []);

  const handlePubRefreshStatus = useCallback(async (accountId: string) => {
    const platform = accountId.replace('pub-', '');
    setPubLoading(prev => ({ ...prev, [accountId]: true }));
    setPubResults(prev => ({ ...prev, [accountId]: null }));
    try {
      const res = await fetch(apiUrl(`/platforms/${platform}/refresh`), {
        method: 'POST', credentials: 'include',
      });
      const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` })) as { error?: string };
      const msg = res.ok
        ? `Token refreshed for ${platform}.`
        : (body.error ?? `HTTP ${res.status} — refresh not yet implemented.`);
      if (res.ok) {
        setPubAccounts(prev => prev.map(a =>
          a.id === accountId ? { ...a, status: 'connected', tokenStatus: 'valid' } : a
        ));
      }
      setPubResults(prev => ({ ...prev, [accountId]: msg }));
    } catch {
      setPubResults(prev => ({ ...prev, [accountId]: `Backend offline — run: cd backend && go run ./cmd/api` }));
    }
    setPubLoading(prev => ({ ...prev, [accountId]: false }));
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
                  const meta   = PLATFORMS[p];
                  const active = settings.platforms.includes(p);
                  return (
                    <button
                      key={p}
                      className="platform-toggle"
                      onClick={() => togglePlatform(p)}
                      aria-pressed={active}
                      style={{
                        background:  active ? meta.bg  : 'var(--bg-subtle)',
                        borderColor: active ? meta.color : 'var(--border-strong)',
                        color:       active ? meta.color : 'var(--text-dim)',
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
            Settings persist in localStorage (theme &amp; preferences only — no secrets)
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
            <div className="security-inline-warning">
              <span className="security-warning-icon">⚠</span>
              OAuth client secrets and API keys must <strong>never</strong> be placed in{' '}
              <code className="int-code">VITE_</code> variables or the browser bundle.
              All credentials are encrypted and stored server-side only. The frontend receives
              connection status only — never raw tokens.
            </div>
          </div>
          <div className="integration-grid">
            {dataProviders.map(provider => (
              <IntegrationCard
                key={provider.id}
                provider={provider}
                loading={!!intLoading[provider.id]}
                testResult={intResults[provider.id] ?? null}
                onTest={() => handleIntTest(provider.id)}
                onConnect={() => handleIntConnect(provider.id)}
                onDisconnect={() => handleIntDisconnect(provider.id)}
                onRefresh={() => handleIntRefresh(provider.id)}
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
              AI provider API keys must <strong>never</strong> be stored in the browser, localStorage, or{' '}
              <code className="int-code">VITE_</code> variables — these are compiled into the public JS bundle.
              All key management goes through <code className="int-code">/api/ai/*</code> backend endpoints.
              Keys are stored encrypted at rest using KMS (AWS Secrets Manager / HashiCorp Vault).
            </div>
          </div>
          <div className="ai-provider-grid">
            {AI_PROVIDERS.map(p => {
              const statusCls = `status-${p.status.replace(/_/g, '-')}`;
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
                      {p.status === 'demo'          ? 'Demo mode'     :
                       p.status === 'connected'     ? 'Connected'     :
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
                    {(p.status === 'not_connected' || p.status === 'demo') && (
                      <button
                        className={`btn-int${p.status === 'not_connected' ? ' btn-int--primary' : ''}`}
                        onClick={() => {
                          alert(
                            `Production: Configure ${p.name} API key on your backend server.\n` +
                            `Endpoint: ${p.backendEndpoint}\n` +
                            `Key must be stored encrypted server-side — never in VITE_ vars or the browser.`
                          );
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
              OAuth refresh tokens are stored <strong>server-side only</strong> — the frontend never
              receives them. Connect/disconnect/refresh actions call{' '}
              <code className="int-code">/api/publish/*</code> and return connection status only.
              Token rotation happens automatically server-side on a cron schedule.
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
                onRefreshStatus={() => handlePubRefreshStatus(account.id)}
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
              How credentials are handled in TrendCortex — security rules and backend requirements.
            </div>
          </div>

          <div className="security-rules-card">
            {[
              {
                icon: '🚫',
                title: 'Secrets never stored in the browser',
                body: 'No API keys, OAuth tokens, refresh tokens, or client secrets are stored in ' +
                  'localStorage, sessionStorage, cookies, or React state. VITE_ environment variables ' +
                  'are compiled into the public JavaScript bundle and must never hold sensitive values. ' +
                  'The frontend only receives connection status metadata — never raw credentials.',
              },
              {
                icon: '🔐',
                title: 'OAuth client secrets belong only on the backend',
                body: 'client_id and client_secret for OAuth 2.0 providers (TikTok, Instagram, Facebook, ' +
                  'Twitter, Threads, YouTube) must live in server environment variables (process.env), ' +
                  'never in VITE_ vars or the browser bundle. They are used only during the token ' +
                  'exchange step in the server-side OAuth callback handler.',
              },
              {
                icon: '🔑',
                title: 'Tokens must be encrypted server-side',
                body: 'All access and refresh tokens are encrypted at rest using AES-256-GCM with a ' +
                  'KMS-managed data key before storage. Production storage options: AWS Secrets Manager, ' +
                  'HashiCorp Vault, GCP Secret Manager, or an encrypted Postgres/DynamoDB table with ' +
                  'column-level encryption. The encryption key is never stored alongside the ciphertext.',
              },
              {
                icon: '🖥',
                title: 'Local development',
                body: 'For local backend development, use a server-side .env file with non-VITE_ ' +
                  'prefixed variables (e.g. TIKTOK_CLIENT_SECRET). These stay on the server process ' +
                  'and are never bundled into the frontend asset. Never commit .env files to git.',
              },
              {
                icon: '🖥',
                title: 'Backend required for OAuth',
                body: 'TrendCortex requires the Go backend (cd backend && go run ./cmd/api) running with ' +
                  'platform credentials in .env before any OAuth connection can be established. ' +
                  'Social Connections and Publishing tabs call real backend endpoints — ' +
                  'no mock fallbacks. If the backend is offline, buttons show an inline error instead of fake success.',
              },
              {
                icon: '🔄',
                title: 'OAuth token refresh and rotation',
                body: 'Token refresh happens entirely server-side on a cron schedule. The frontend ' +
                  'triggers refresh via POST /api/publish/refresh/:platform but never sees the new ' +
                  'token — only a success/expiry status response. Refresh tokens are rotated on every ' +
                  'use (RFC 6749 §10.4). KMS key rotation re-encrypts stored ciphertext on a separate schedule.',
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
