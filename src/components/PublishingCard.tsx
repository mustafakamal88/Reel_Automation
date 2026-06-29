import type { PublishingAccount, TokenStatus, IntegrationStatus } from '../lib/integrations/types';
import { PLATFORMS } from '../data/platforms';

function tokenLabel(t: TokenStatus): string {
  const labels: Record<TokenStatus, string> = {
    valid:    'Token valid',
    expiring: 'Expiring soon',
    expired:  'Token expired',
    none:     '—',
  };
  return labels[t];
}

function connectionLabel(s: IntegrationStatus): string {
  const labels: Partial<Record<IntegrationStatus, string>> = {
    connected:       'Connected',
    configuring:     'Connecting…',
    needs_attention: 'Needs attention',
    error:           'Error',
    not_connected:   'Not connected',
  };
  return labels[s] ?? s;
}

interface Props {
  account: PublishingAccount;
  loading: boolean;
  testResult: string | null;
  onConnect: () => void;
  onDisconnect: () => void;
  onTestUpload: () => void;
  onRefreshStatus: () => void;
  onDismissResult: () => void;
}

export function PublishingCard({
  account,
  loading,
  testResult,
  onConnect,
  onDisconnect,
  onTestUpload,
  onRefreshStatus,
  onDismissResult,
}: Props) {
  const platform       = PLATFORMS[account.platformKey as keyof typeof PLATFORMS];
  const isConnected    = account.status === 'connected';
  const isConfiguring  = account.status === 'configuring';
  const needsAttention = account.status === 'needs_attention';
  const isError        = account.status === 'error';
  const statusCls      = `status-${account.status.replace(/_/g, '-')}`;

  return (
    <div className={`pub-card${isConnected ? ' pub-card--connected' : ''}`}>
      <div className="pub-card-header">
        <div
          className="pub-icon"
          style={{
            background: platform?.bg ?? 'var(--bg-subtle)',
            color: platform?.color ?? 'var(--text-dim)',
          }}
        >
          {platform?.short ?? '?'}
        </div>

        <div className="pub-card-info">
          <div className="pub-card-name">{account.name}</div>
          <div className="pub-card-handle">
            {isConnected ? account.handle : 'No account connected'}
          </div>
        </div>

        <span className={`status-badge ${statusCls}`}>
          {connectionLabel(account.status)}
        </span>
      </div>

      <div className="pub-permissions">
        <div className="pub-perm">
          <span
            className="pub-perm-dot"
            style={{ background: account.uploadPermission ? 'var(--green)' : 'var(--text-dimmer)' }}
          />
          Upload
        </div>
        <div className="pub-perm">
          <span
            className="pub-perm-dot"
            style={{ background: account.publishPermission ? 'var(--green)' : 'var(--text-dimmer)' }}
          />
          Publish
        </div>
        {isConnected && (
          <>
            <div className="pub-perm" style={{ marginLeft: 'auto' }}>
              <span className={`pub-token-status pub-token--${account.tokenStatus}`}>
                {tokenLabel(account.tokenStatus)}
              </span>
            </div>
            {account.tokenExpiry && (
              <div className="pub-perm" style={{ color: 'var(--text-dim)', fontSize: 11 }}>
                exp {account.tokenExpiry}
              </div>
            )}
          </>
        )}
      </div>

      {isConnected && account.defaultHashtags.length > 0 && (
        <div className="pub-defaults">
          <span style={{ color: 'var(--text-dimmer)' }}>Default hashtags: </span>
          {account.defaultHashtags.join(' ')}
          {'  ·  '}
          <span style={{ color: 'var(--text-dimmer)' }}>Privacy: </span>
          {account.defaultPrivacy}
        </div>
      )}

      {testResult && (
        <div className="integration-result">
          <span className="integration-result-text">{testResult}</span>
          <button className="integration-result-close" onClick={onDismissResult}>✕</button>
        </div>
      )}

      <div className="pub-card-footer">
        {loading ? (
          <span className="btn-int btn-int--loading">
            <span className="int-spinner" />
            {isConfiguring ? 'Connecting…' : 'Working…'}
          </span>
        ) : isConnected ? (
          <>
            <button className="btn-int" onClick={onTestUpload}>Test upload</button>
            <button className="btn-int btn-int--danger" onClick={onDisconnect}>Disconnect</button>
          </>
        ) : needsAttention ? (
          <>
            <button className="btn-int btn-int--primary" onClick={onRefreshStatus}>Refresh status</button>
            <button className="btn-int btn-int--danger" onClick={onDisconnect}>Disconnect</button>
          </>
        ) : isError ? (
          <>
            <button className="btn-int btn-int--primary" onClick={onConnect}>Reconnect</button>
            <button className="btn-int btn-int--danger" onClick={onDisconnect}>Disconnect</button>
          </>
        ) : (
          <button className="btn-int btn-int--primary" onClick={onConnect}>
            Connect account
          </button>
        )}
      </div>
    </div>
  );
}
