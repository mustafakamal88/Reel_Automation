import { useEffect, useMemo, useState } from 'react';
import { getPlatformConnections, type PlatformStatus } from '../lib/api/client';

const PUBLISH_PLATFORMS = [
  { platform: 'youtube', name: 'YouTube Shorts', short: 'YT', color: 'var(--yt-color)', bg: 'var(--yt-bg)' },
  { platform: 'tiktok', name: 'TikTok', short: 'TT', color: 'var(--tt-color)', bg: 'var(--tt-bg)' },
  { platform: 'instagram', name: 'Instagram Reels', short: 'IG', color: 'var(--ig-color)', bg: 'var(--ig-bg)' },
  { platform: 'facebook', name: 'Facebook Reels', short: 'FB', color: 'var(--fb-color)', bg: 'var(--fb-bg)' },
  { platform: 'x', name: 'X', short: 'X', color: 'var(--x-color)', bg: 'var(--x-bg)' },
];

function statusLabel(status?: PlatformStatus['status']) {
  if (status === 'connected') return 'Connected';
  return 'Not connected';
}

export function PublishPage() {
  const [platforms, setPlatforms] = useState<PlatformStatus[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    getPlatformConnections()
      .then(res => {
        if (!cancelled) setPlatforms(res.platforms);
      })
      .catch(() => {
        if (!cancelled) setPlatforms([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, []);

  const byPlatform = useMemo(() => new Map(platforms.map(p => [p.platform, p])), [platforms]);

  return (
    <section className="page-section">
      <div style={{ maxWidth: 780, marginBottom: 18 }}>
        <div style={{ fontSize: 15, fontWeight: 800, color: 'var(--text-primary)' }}>Publishing readiness</div>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6, marginTop: 6 }}>
          Direct uploads stay disabled until OAuth and platform API publishing are fully configured. Download ZIP packages from Exports for manual posting.
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: 12 }}>
        {PUBLISH_PLATFORMS.map(meta => {
          const conn = byPlatform.get(meta.platform);
          const connected = conn?.status === 'connected';
          return (
            <div key={meta.platform} className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div style={{ width: 38, height: 38, borderRadius: 8, background: meta.bg, border: `1px solid ${meta.color}33`, display: 'grid', placeItems: 'center', fontFamily: 'var(--font-mono)', fontWeight: 800, color: meta.color }}>
                  {meta.short}
                </div>
                <div>
                  <div style={{ fontSize: 14, fontWeight: 800, color: 'var(--text-primary)' }}>{meta.name}</div>
                  <div style={{ fontSize: 12, color: connected ? 'var(--green)' : 'var(--text-dim)', marginTop: 3 }}>
                    {loading ? 'Checking status...' : statusLabel(conn?.status)}
                  </div>
                </div>
              </div>

              <div style={{ display: 'grid', gap: 8, fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.5 }}>
                <div>Upload disabled until OAuth/API setup exists.</div>
                <div>Manual download available from Exports.</div>
                {conn?.handle && <div style={{ color: 'var(--text-secondary)' }}>{conn.handle}</div>}
              </div>

              <button className="generate-btn idle" type="button" disabled style={{ justifyContent: 'center', opacity: 0.55, cursor: 'not-allowed' }}>
                Upload disabled
              </button>
            </div>
          );
        })}
      </div>
    </section>
  );
}
