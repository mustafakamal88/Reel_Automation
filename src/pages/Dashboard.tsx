import { useEffect, useState } from 'react';
import { getPlatformConnections } from '../lib/api/client';
import { storage, type StoredScriptPackage, type ActivityState } from '../lib/storage';

interface Props {
  latestScript: StoredScriptPackage | null;
  onNavigate: (view: 'trendFinder' | 'scriptStudio' | 'clipStudio' | 'connections') => void;
}

function formatActivityTime(value: string | null): string {
  if (!value) return 'No activity yet';
  return new Date(value).toLocaleString();
}

export function DashboardPage({ latestScript, onNavigate }: Props) {
  const [activity, setActivity] = useState<ActivityState>(() => storage.getActivity());
  const [connectedPlatforms, setConnectedPlatforms] = useState(0);
  const [connectionStatus, setConnectionStatus] = useState<'checking' | 'ok' | 'offline'>('checking');

  useEffect(() => {
    setActivity(storage.getActivity());
    let cancelled = false;
    getPlatformConnections()
      .then(res => {
        if (cancelled) return;
        setConnectedPlatforms(res.platforms.filter(platform => platform.status === 'connected').length);
        setConnectionStatus('ok');
      })
      .catch(() => {
        if (!cancelled) setConnectionStatus('offline');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const kpis = [
    { label: 'Trends found today', value: activity.trendsFoundToday > 0 ? String(activity.trendsFoundToday) : 'No activity yet' },
    { label: 'Scripts generated', value: activity.scriptsGenerated > 0 ? String(activity.scriptsGenerated) : 'No activity yet' },
    { label: 'Clips generated', value: activity.clipsGenerated > 0 ? String(activity.clipsGenerated) : 'No activity yet' },
    { label: 'Packages downloaded', value: activity.packagesDownloaded > 0 ? String(activity.packagesDownloaded) : 'No activity yet' },
    { label: 'Connected platforms', value: connectionStatus === 'offline' ? 'Backend offline' : String(connectedPlatforms) },
    { label: 'Publishing readiness', value: connectedPlatforms > 0 ? 'Accounts connected' : 'Connect required' },
  ];

  return (
    <section className="page-section">
      <div className="page-hero">
        <div>
          <div className="page-eyebrow">Creator research workspace</div>
          <h1>Find demand, write scripts, generate clips.</h1>
          <p>TrendCortex is organized around creator research and local clip packaging. Publishing starts from Clip Generator once real accounts are connected.</p>
        </div>
      </div>

      <div className="kpi-grid">
        {kpis.map(kpi => (
          <div key={kpi.label} className="kpi-card">
            <div className="kpi-label">{kpi.label}</div>
            <div className="kpi-value">{kpi.value}</div>
          </div>
        ))}
      </div>

      <div className="dashboard-grid">
        <div className="settings-card">
          <div className="settings-card-title">Recent Activity</div>
          <div className="status-list">
            <StatusRow label="Latest trend pulled" value={formatActivityTime(activity.latestTrendPulled)} />
            <StatusRow label="Latest script generated" value={latestScript ? formatActivityTime(latestScript.savedAt) : formatActivityTime(activity.latestScriptGenerated)} />
            <StatusRow label="Latest clip package generated" value={formatActivityTime(activity.latestClipPackageGenerated)} />
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Workflow</div>
          <div className="quick-actions">
            <button className="generate-btn idle" type="button" onClick={() => onNavigate('trendFinder')}>Open Trend Finder</button>
            <button className="generate-btn idle" type="button" onClick={() => onNavigate('scriptStudio')}>Open Script Studio</button>
            <button className="generate-btn idle" type="button" onClick={() => onNavigate('clipStudio')}>Open Clip Generator</button>
            <button className="generate-btn idle" type="button" onClick={() => onNavigate('connections')}>Open Connections</button>
          </div>
          <div className="muted-note">
            No fake publishing or connected-account data is shown. Download ZIPs manually until OAuth and upload APIs are configured.
          </div>
        </div>
      </div>
    </section>
  );
}

function StatusRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="status-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
