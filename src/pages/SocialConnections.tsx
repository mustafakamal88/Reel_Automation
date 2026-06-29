import { useEffect, useState } from 'react';
import { getPlatformConnections, getOAuthStartURL, ApiError } from '../lib/api/client';
import type { PlatformStatus } from '../lib/api/client';

const PLATFORM_META: Record<string, { color: string; bg: string; short: string }> = {
  youtube:   { color: 'var(--yt-color)', bg: 'var(--yt-bg)',   short: 'YT' },
  tiktok:    { color: 'var(--tt-color)', bg: 'var(--tt-bg)',   short: 'TT' },
  instagram: { color: 'var(--ig-color)', bg: 'var(--ig-bg)',   short: 'IG' },
  facebook:  { color: 'var(--fb-color)', bg: 'var(--fb-bg)',   short: 'FB' },
  threads:   { color: 'var(--th-color)', bg: 'var(--th-bg)',   short: 'TH' },
  x:         { color: 'var(--x-color)',  bg: 'var(--x-bg)',    short: 'X'  },
};

function StatusBadge({ status }: { status: PlatformStatus['status'] }) {
  const map: Record<PlatformStatus['status'], { label: string; color: string }> = {
    not_connected:       { label: 'Not connected',       color: 'var(--text-dim)' },
    connected:           { label: 'Connected',           color: 'var(--green)' },
    expired:             { label: 'Token expired',       color: 'var(--yellow)' },
    credentials_missing: { label: 'Credentials missing', color: 'var(--red)' },
  };
  const { label, color } = map[status] ?? { label: status, color: 'var(--text-dim)' };
  return (
    <span style={{
      fontSize: 11,
      fontFamily: 'var(--font-mono)',
      color,
      background: 'var(--bg-subtle)',
      border: `1px solid ${color}33`,
      borderRadius: 4,
      padding: '2px 7px',
    }}>
      {label}
    </span>
  );
}

function ConnectionCard({ conn }: { conn: PlatformStatus }) {
  const meta = PLATFORM_META[conn.platform] ?? { color: 'var(--accent)', bg: 'var(--bg-subtle)', short: conn.platform.toUpperCase().slice(0, 2) };
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleConnect() {
    setConnecting(true);
    setError(null);
    try {
      const res = await getOAuthStartURL(conn.platform);
      window.location.href = res.authorize_url;
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.isCredentialsMissing) {
          setError('Platform app credentials are missing. Add backend environment variables first.');
        } else if (err.isBackendOffline) {
          setError('Backend offline — run: cd backend && go run ./cmd/api');
        } else if (err.isNotImplemented) {
          setError('OAuth not yet wired on backend. PKCE + state storage required before connecting.');
        } else {
          setError(err.message);
        }
      } else {
        setError(err instanceof Error ? err.message : 'Unknown error');
      }
    } finally {
      setConnecting(false);
    }
  }

  return (
    <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          width: 38, height: 38, borderRadius: 9,
          background: meta.bg,
          border: `1px solid ${meta.color}33`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontFamily: 'var(--font-mono)', fontWeight: 700, fontSize: 13,
          color: meta.color, flexShrink: 0,
        }}>
          {meta.short}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--text-primary)' }}>{conn.name}</div>
          {conn.handle && (
            <div style={{ fontSize: 12, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
              {conn.handle}
            </div>
          )}
        </div>
        <StatusBadge status={conn.status} />
      </div>

      <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', lineHeight: 1.6 }}>
        Scopes: {conn.scopes.join(' · ')}
      </div>

      {conn.status === 'credentials_missing' && (
        <div style={{
          fontSize: 11, color: 'var(--red)', lineHeight: 1.6,
          background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
          borderRadius: 6, padding: '8px 10px',
        }}>
          Platform app credentials are missing. Add backend environment variables first.
        </div>
      )}

      {error && (
        <div style={{
          fontSize: 11, color: 'var(--red)', lineHeight: 1.6,
          background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
          borderRadius: 6, padding: '8px 10px',
        }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        {(conn.status === 'not_connected') && (
          <button
            className="btn-int btn-int--primary"
            onClick={handleConnect}
            disabled={connecting}
          >
            {connecting ? 'Opening OAuth…' : 'Connect via OAuth'}
          </button>
        )}
        {conn.status === 'credentials_missing' && (
          <span style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
            Add {conn.platform.toUpperCase()}_CLIENT_ID / {conn.platform.toUpperCase()}_CLIENT_SECRET to .env
          </span>
        )}
        {conn.status === 'connected' && (
          <button className="btn-int" onClick={() => void 0} disabled>
            Disconnect
          </button>
        )}
        {conn.status === 'expired' && (
          <button
            className="btn-int btn-int--primary"
            onClick={handleConnect}
            disabled={connecting}
          >
            Reconnect
          </button>
        )}
      </div>
    </div>
  );
}

type FetchState = 'loading' | 'ok' | 'error';

export function SocialConnectionsPage() {
  const [fetchState, setFetchState] = useState<FetchState>('loading');
  const [platforms, setPlatforms] = useState<PlatformStatus[]>([]);
  const [fetchError, setFetchError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getPlatformConnections()
      .then(res => {
        if (!cancelled) {
          setPlatforms(res.platforms);
          setFetchState('ok');
        }
      })
      .catch(err => {
        if (!cancelled) {
          setFetchError(err instanceof Error ? err.message : 'Failed to load connection status');
          setFetchState('error');
        }
      });
    return () => { cancelled = true; };
  }, []);

  return (
    <section className="page-section">
      <div className="int-section-header">
        <div className="int-section-title">Social Platform Connections</div>
        <div className="int-section-sub">
          Connect your publishing accounts via official OAuth. TrendCortex never asks for
          your social media password — only the platform's own login flow is used.
        </div>
        <div className="security-inline-warning">
          <span className="security-warning-icon">⚠</span>
          <span>
            All OAuth tokens are stored <strong>encrypted server-side</strong> in the Go backend.
            The frontend only receives connection status — never raw access or refresh tokens.
            No platform credentials are stored in the browser or environment variables.
          </span>
        </div>
      </div>

      {fetchState === 'loading' && (
        <div style={{ color: 'var(--text-dim)', fontSize: 13, padding: '24px 0' }}>
          Loading connection status from backend…
        </div>
      )}

      {fetchState === 'error' && (
        <div style={{
          color: 'var(--red)', fontSize: 13, lineHeight: 1.7,
          background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
          borderRadius: 8, padding: '14px 16px', marginTop: 12,
        }}>
          <strong>Cannot reach the Go backend.</strong><br />
          {fetchError}<br />
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>
            Start it with: cd backend &amp;&amp; go run ./cmd/api
          </span>
        </div>
      )}

      {fetchState === 'ok' && (
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
          gap: 14, marginTop: 6,
        }}>
          {platforms.map(conn => (
            <ConnectionCard key={conn.platform} conn={conn} />
          ))}
        </div>
      )}

      <div style={{ marginTop: 28 }}>
        <div className="int-section-title" style={{ marginBottom: 12 }}>OAuth Flow</div>
        <div className="security-endpoints-card">
          {([
            { method: 'GET',  path: '/platforms/connections',       note: 'Connection status for all platforms (no tokens returned)' },
            { method: 'GET',  path: '/oauth/{platform}/start',      note: 'Returns authorize_url → browser redirects to platform login' },
            { method: 'GET',  path: '/oauth/{platform}/callback',   note: 'Receives code, exchanges for tokens, stores encrypted server-side' },
            { method: 'POST', path: '/platforms/{platform}/disconnect', note: 'Revokes token and removes encrypted credentials (501 until implemented)' },
            { method: 'POST', path: '/platforms/{platform}/refresh',    note: 'Refreshes access token via stored refresh token (501 until implemented)' },
            { method: 'POST', path: '/platforms/{platform}/test',       note: 'Live credential probe via provider API (501 until implemented)' },
          ] as const).map(e => (
            <div key={e.path} className="security-endpoint-row">
              <span className={`http-method http-method--${e.method.toLowerCase()}`}>{e.method}</span>
              <code className="int-code security-endpoint-path">{e.path}</code>
              <span className="security-endpoint-note">{e.note}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
