import { useEffect, useState } from 'react';
import {
  ApiError,
  createDailyPackage,
  downloadDailyPackageZip,
  downloadExportZip,
  getExportJobs,
  type DailyPackageResponse,
  type ExportJob,
} from '../lib/api/client';

type ActionState = 'idle' | 'pending' | 'ready' | 'error';

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    if (err.isBackendOffline) return 'Backend offline. Start the Go backend or check VITE_API_BASE_URL.';
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

export function ExportsPage() {
  const [jobs, setJobs] = useState<ExportJob[]>([]);
  const [loadingJobs, setLoadingJobs] = useState(true);
  const [state, setState] = useState<ActionState>('idle');
  const [downloadState, setDownloadState] = useState<ActionState>('idle');
  const [pkg, setPkg] = useState<DailyPackageResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const today = new Date().toISOString().split('T')[0];

  useEffect(() => {
    let cancelled = false;
    getExportJobs()
      .then(res => {
        if (!cancelled) setJobs(res.export_jobs);
      })
      .catch(() => {
        if (!cancelled) setJobs([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingJobs(false);
      });
    return () => { cancelled = true; };
  }, []);

  async function handleCreateDailyPackage() {
    setState('pending');
    setDownloadState('idle');
    setError(null);
    setPkg(null);
    try {
      const res = await createDailyPackage({
        date: today,
        region: 'US',
        language: 'en-US',
        platform_targets: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
        duration_target: '30s',
        tone_style: 'concise, factual, high-retention',
      });
      setPkg(res);
      setState('ready');
    } catch (err) {
      setError(errorMessage(err, 'Daily export package failed'));
      setState('error');
    }
  }

  async function handleDownloadDailyPackage() {
    if (!pkg) return;
    setDownloadState('pending');
    setError(null);
    try {
      await downloadDailyPackageZip(pkg.download_url, pkg.zip_filename);
      setDownloadState('ready');
    } catch (err) {
      setError(errorMessage(err, 'ZIP download failed'));
      setDownloadState('error');
    }
  }

  const hasExports = jobs.length > 0 || Boolean(pkg);

  return (
    <section className="page-section">
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 320px), 1fr))', gap: 14, alignItems: 'start' }}>
        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <div style={{ fontSize: 15, fontWeight: 800, color: 'var(--text-primary)' }}>Daily ZIP package</div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6, marginTop: 6 }}>
              Create a real trend-backed ZIP package for manual publishing. No social upload is triggered.
            </div>
          </div>
          <button className={`generate-btn${state === 'ready' ? ' done' : ' idle'}`} type="button" onClick={handleCreateDailyPackage} disabled={state === 'pending'} style={{ justifyContent: 'center' }}>
            {state === 'pending' ? 'Creating package...' : state === 'ready' ? 'Package ready' : 'Create daily ZIP'}
          </button>
          <button className={`generate-btn${downloadState === 'ready' ? ' done' : ' idle'}`} type="button" onClick={handleDownloadDailyPackage} disabled={!pkg || downloadState === 'pending'} style={{ justifyContent: 'center', opacity: pkg ? 1 : 0.55 }}>
            {downloadState === 'pending' ? 'Downloading...' : 'Download ZIP'}
          </button>
          {pkg && (
            <div style={{ fontSize: 11, color: 'var(--green)', fontFamily: 'var(--font-mono)', lineHeight: 1.5 }}>
              {pkg.zip_filename} · {pkg.included_files.length} file(s)
            </div>
          )}
          {error && (
            <div style={{ fontSize: 12, color: 'var(--red)', lineHeight: 1.6, background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)', borderRadius: 6, padding: '8px 10px' }}>
              {error}
            </div>
          )}
        </div>

        <div className="settings-card" style={{ display: 'grid', gap: 12 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
            <div>
              <div style={{ fontSize: 15, fontWeight: 800, color: 'var(--text-primary)' }}>Export packages</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 5 }}>Generated clip packages and backend export ZIPs.</div>
            </div>
            <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>{loadingJobs ? 'loading' : `${jobs.length} job(s)`}</div>
          </div>

          {!loadingJobs && !hasExports && (
            <div className="empty-state system-state is-empty" style={{ margin: 0, padding: '48px 16px' }} role="status">
              <div className="empty-icon" aria-hidden="true">i</div>
              <div className="empty-title">No exports yet. Generate clips to create your first package.</div>
            </div>
          )}

          {pkg && (
            <div style={{ border: '1px solid var(--border-card)', borderRadius: 8, padding: 12, background: 'var(--bg-subtle)' }}>
              <div style={{ fontSize: 13, fontWeight: 800, color: 'var(--text-primary)' }}>{pkg.zip_filename}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 5 }}>{pkg.message}</div>
            </div>
          )}

          {jobs.map(job => (
            <div key={job.id} style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 10, alignItems: 'center', border: '1px solid var(--border-card)', borderRadius: 8, padding: 12, background: 'var(--bg-subtle)' }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 13, fontWeight: 800, color: 'var(--text-primary)' }}>Batch export</div>
                <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', marginTop: 4 }}>
                  {job.status} · {job.completed_at || job.created_at}
                </div>
                {job.error_message && <div style={{ fontSize: 12, color: 'var(--red)', marginTop: 5 }}>{job.error_message}</div>}
              </div>
              <button
                className="generate-btn idle"
                type="button"
                disabled={job.status !== 'completed'}
                onClick={() => void downloadExportZip(job.id, `trendcortex-export-${job.id}.zip`)}
                style={{ opacity: job.status === 'completed' ? 1 : 0.55 }}
              >
                Download
              </button>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
