import { useEffect, useState } from 'react';
import type { View } from '../types';
import { getPlatformConnections } from '../lib/api/client';
import { storage, type StoredScriptPackage, type ActivityState } from '../lib/storage';

interface Props {
  latestScript: StoredScriptPackage | null;
  onNavigate: (view: View) => void;
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
    { label: 'Clip packages generated', value: activity.clipsGenerated > 0 ? String(activity.clipsGenerated) : 'No activity yet' },
    { label: 'YouTube analyses run', value: activity.youtubeAnalysesRun > 0 ? String(activity.youtubeAnalysesRun) : 'No activity yet' },
    { label: 'Channel analyses run', value: activity.channelAnalysesRun > 0 ? String(activity.channelAnalysesRun) : 'No activity yet' },
    { label: 'Connected platforms', value: connectionStatus === 'offline' ? 'Unavailable' : String(connectedPlatforms) },
  ];

  return (
    <section className="page-section">
      <div className="page-hero">
        <div>
          <div className="page-eyebrow">Creator research workspace</div>
          <h1>Find demand, shape scripts, package clips.</h1>
          <p>A calm workspace for moving from real trend data to usable short-form clip packages. Publishing stays gated until real accounts are connected.</p>
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
            <StatusRow label="Latest YouTube video analysis" value={formatActivityTime(activity.latestYouTubeAnalysis)} />
            <StatusRow label="Latest channel analysis" value={formatActivityTime(activity.latestChannelAnalysis)} />
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-title">Quick actions</div>
          <div className="quick-actions">
            <button className="generate-btn idle" type="button" onClick={() => onNavigate('trendingKeywords')}>Open Keyword Search</button>
            <button className="generate-btn secondary" type="button" onClick={() => onNavigate('scriptStudio')}>Open Script Studio</button>
            <button className="generate-btn secondary" type="button" onClick={() => onNavigate('clipStudio')}>Open Clip Generator</button>
            <button className="generate-btn secondary" type="button" onClick={() => onNavigate('connections')}>Open Connections</button>
          </div>
          <div className="neutral-callout" style={{ marginTop: 16 }}>
            Direct publishing appears after real social accounts are connected. Until then, generated packages can be downloaded as ZIP files.
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
