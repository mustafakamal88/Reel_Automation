import type { IntegrationProvider, IntegrationStatus, AuthType } from '../lib/integrations/types';
import { PLATFORMS } from '../data/platforms';

function statusLabel(s: IntegrationStatus): string {
  const labels: Record<IntegrationStatus, string> = {
    demo: 'Demo',
    not_connected: 'Not connected',
    connected: 'Connected',
    error: 'Error',
    configuring: 'Connecting…',
  };
  return labels[s];
}

function authLabel(a: AuthType): string {
  const labels: Record<AuthType, string> = {
    oauth2: 'OAuth 2.0',
    api_key: 'API Key',
    server_adapter: 'Server',
    none: 'None',
  };
  return labels[a];
}

function authClass(a: AuthType): string {
  const map: Record<AuthType, string> = {
    oauth2: 'auth-oauth2',
    api_key: 'auth-api-key',
    server_adapter: 'auth-server',
    none: 'auth-none',
  };
  return map[a];
}

function secretLabel(mode: IntegrationProvider['secretStorageMode']): string {
  if (mode === 'backend_encrypted') return 'Secrets: backend-encrypted';
  if (mode === 'env_local_dev') return 'Secrets: .env (local dev only)';
  return 'No secrets required';
}

interface Props {
  provider: IntegrationProvider;
  loading: boolean;
  testResult: string | null;
  onTest: () => void;
  onDismissResult: () => void;
}

export function IntegrationCard({ provider, loading, testResult, onTest, onDismissResult }: Props) {
  const platform = PLATFORMS[provider.platformKey as keyof typeof PLATFORMS];
  const statusCls = `status-${provider.status.replace('_', '-')}`;

  return (
    <div className="integration-card">
      <div className="integration-card-header">
        <div
          className="integration-icon"
          style={{
            background: platform?.bg ?? 'var(--bg-subtle)',
            color: platform?.color ?? 'var(--text-dim)',
          }}
        >
          {platform?.short ?? '?'}
        </div>

        <div className="integration-info">
          <div className="integration-name">{provider.name}</div>
          <div className="integration-subtitle">
            {provider.lastSync ? `Last sync: ${provider.lastSync}` : 'Not synced'}
          </div>
        </div>

        <div className="integration-badges">
          <span className={`status-badge ${statusCls}`}>{statusLabel(provider.status)}</span>
          <span className={`auth-badge ${authClass(provider.authType)}`}>{authLabel(provider.authType)}</span>
        </div>
      </div>

      {provider.scopes.length > 0 && (
        <div className="integration-scopes">
          {provider.scopes.map(scope => (
            <span
              key={scope.name}
              className={`scope-pill${scope.required ? ' scope-required' : ''}`}
              title={scope.description}
            >
              {scope.name}{!scope.required && ' (opt)'}
            </span>
          ))}
        </div>
      )}

      {testResult && (
        <div className="integration-result">
          <span className="integration-result-text">{testResult}</span>
          <button className="integration-result-close" onClick={onDismissResult}>✕</button>
        </div>
      )}

      <div className="integration-footer">
        <span className="integration-secret-mode">{secretLabel(provider.secretStorageMode)}</span>
        <div className="integration-actions">
          {loading ? (
            <span className="btn-int btn-int--loading">
              <span className="int-spinner" />
              Testing…
            </span>
          ) : (
            <button className="btn-int" onClick={onTest}>
              Test
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
