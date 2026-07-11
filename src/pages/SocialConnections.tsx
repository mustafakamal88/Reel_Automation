import { useEffect, useState } from 'react';
import { ApiError, getOAuthStartURL, getPlatformConnections, type PlatformStatus } from '../lib/api/client';

const PUBLISH_PLATFORMS = ['youtube', 'tiktok', 'instagram', 'facebook', 'x'];

const PLATFORM_META: Record<string, { color: string; bg: string; short: string; label: string }> = {
  youtube: { color: 'var(--yt-color)', bg: 'var(--yt-bg)', short: 'YT', label: 'YouTube Shorts' },
  tiktok: { color: 'var(--tt-color)', bg: 'var(--tt-bg)', short: 'TT', label: 'TikTok' },
  instagram: { color: 'var(--ig-color)', bg: 'var(--ig-bg)', short: 'IG', label: 'Instagram Reels' },
  facebook: { color: 'var(--fb-color)', bg: 'var(--fb-bg)', short: 'FB', label: 'Facebook Reels' },
  x: { color: 'var(--x-color)', bg: 'var(--x-bg)', short: 'X', label: 'X' },
};

const FALLBACK_CONNECTIONS: PlatformStatus[] = PUBLISH_PLATFORMS.map(platform => ({
  platform,
  name: PLATFORM_META[platform].label,
  status: 'credentials_missing',
  scopes: [],
  can_publish: false,
}));

function neutralStatus(status: PlatformStatus['status']): string {
  if (status === 'connected') return 'Connected';
  if (status === 'credentials_missing') return 'Setup needed';
  if (status === 'expired') return 'Reconnect needed';
  return 'Not connected';
}

function ConnectionCard({ conn }: { conn: PlatformStatus }) {
  const meta = PLATFORM_META[conn.platform];
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const canAttemptConnect = conn.status === 'not_connected' || conn.status === 'expired';

  async function handleConnect() {
    setConnecting(true);
    setError(null);
    try {
      const res = await getOAuthStartURL(conn.platform);
      window.location.href = res.authorize_url;
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.isCredentialsMissing) {
          setError('Connection setup is not ready yet.');
        } else if (err.isBackendOffline) {
          setError('Connections are temporarily unavailable.');
        } else if (err.isNotImplemented) {
          setError('Direct account connection is not available yet.');
        } else {
          setError(err.message);
        }
      } else {
        setError(err instanceof Error ? err.message : 'Connection failed.');
      }
    } finally {
      setConnecting(false);
    }
  }

  return (
    <div className="settings-card connection-card">
      <div className="connection-card-top">
        <div className="connection-icon" style={{ background: meta.bg, borderColor: `${meta.color}33`, color: meta.color }}>
          {meta.short}
        </div>
        <div>
          <div className="connection-name">{meta.label}</div>
          <div className={conn.status === 'connected' ? 'connection-status connected' : 'connection-status'}>
            {neutralStatus(conn.status)}
          </div>
        </div>
      </div>

      <button className="generate-btn idle" type="button" onClick={handleConnect} disabled={!canAttemptConnect || conn.status === 'credentials_missing' || connecting}>
        {connecting ? 'Opening OAuth...' : conn.status === 'credentials_missing' ? 'Setup required' : 'Connect'}
      </button>

      {error && <div className="neutral-callout">{error}</div>}
    </div>
  );
}

type FetchState = 'loading' | 'ok' | 'error';

export function SocialConnectionsPage() {
  const [fetchState, setFetchState] = useState<FetchState>('loading');
  const [platforms, setPlatforms] = useState<PlatformStatus[]>([]);

  useEffect(() => {
    let cancelled = false;
    getPlatformConnections()
      .then(res => {
        if (!cancelled) {
          setPlatforms(res.platforms.filter(conn => PUBLISH_PLATFORMS.includes(conn.platform)));
          setFetchState('ok');
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFetchState('error');
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Connections</div>
          <h1>Connect accounts to publish directly from Clip Generator.</h1>
          <p>OAuth setup is kept separate from clip creation. Sensitive credentials stay server-side.</p>
        </div>
      </div>

      {fetchState === 'loading' && <div className="muted-note">Loading connection status...</div>}

      {fetchState === 'error' && (
        <>
          <div className="neutral-callout">
            Connections are temporarily unavailable. Showing setup cards only.
          </div>
          <div className="connection-grid" style={{ marginTop: 14 }}>
            {FALLBACK_CONNECTIONS.map(conn => <ConnectionCard key={conn.platform} conn={conn} />)}
          </div>
        </>
      )}

      {fetchState === 'ok' && (
        <div className="connection-grid">
          {platforms.map(conn => <ConnectionCard key={conn.platform} conn={conn} />)}
        </div>
      )}
    </section>
  );
}
