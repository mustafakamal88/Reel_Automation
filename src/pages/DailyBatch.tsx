import { useEffect, useState } from 'react';
import {
  ApiError,
  createDailyPackage,
  downloadDailyPackageZip,
  getDailyPackageRenderJob,
  renderDailyPackageReel,
  type DailyPackageRenderJob,
  type DailyPackageResponse,
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

function toneForRender(status: string): string {
  if (status === 'completed' || status === 'rendered') return 'var(--green)';
  if (status === 'rendering') return 'var(--accent)';
  if (status === 'provider_not_connected' || status === 'renderer_not_available') return '#eab86a';
  return 'var(--red)';
}

export function DailyBatchPage() {
  const [state, setState] = useState<ActionState>('idle');
  const [downloadState, setDownloadState] = useState<ActionState>('idle');
  const [error, setError] = useState<string | null>(null);
  const [pkg, setPkg] = useState<DailyPackageResponse | null>(null);
  const [renderJob, setRenderJob] = useState<DailyPackageRenderJob | null>(null);
  const [renderState, setRenderState] = useState<ActionState>('idle');

  const today = new Date().toISOString().split('T')[0];

  async function handleGenerateTodaySix() {
    setState('pending');
    setDownloadState('idle');
    setRenderState('idle');
    setRenderJob(null);
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
      setError(errorMessage(err, 'Daily package generation failed'));
      setState('error');
    }
  }

  async function handleDownloadZip() {
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

  async function handleRenderReel01() {
    if (!pkg) return;
    setRenderState('pending');
    setError(null);
    try {
      const job = await renderDailyPackageReel('reel-01');
      setRenderJob(job);
    } catch (err) {
      setError(errorMessage(err, 'Render failed to start'));
      setRenderState('error');
    }
  }

  useEffect(() => {
    if (!renderJob || renderJob.status !== 'rendering') return undefined;
    let cancelled = false;
    const timer = window.setInterval(async () => {
      try {
        const job = await getDailyPackageRenderJob(renderJob.id);
        if (cancelled) return;
        setRenderJob(job);
        if (job.status === 'completed') {
          if (job.package) setPkg(job.package);
          setRenderState('ready');
          window.clearInterval(timer);
        }
        if (job.status === 'failed') {
          setRenderState('error');
          setError(job.render_error || job.message);
          window.clearInterval(timer);
        }
      } catch (err) {
        if (cancelled) return;
        setRenderState('error');
        setError(errorMessage(err, 'Render status check failed'));
        window.clearInterval(timer);
      }
    }, 2500);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [renderJob]);

  const renderFailures = pkg?.reels.filter((reel) => !reel.has_video) ?? [];
  const reel01 = pkg?.reels.find((reel) => reel.rank === 1);

  return (
    <section className="page-section">
      <div className="int-section-header">
        <div className="int-section-title">Today's 6 — {today}</div>
        <div className="int-section-sub">
          Generate six real trend-backed reel packages and download one ZIP for manual publishing.
        </div>
      </div>

      <div className="security-inline-warning" style={{ marginBottom: 16 }}>
        <span className="security-warning-icon">!</span>
        <span>
          Platform auto-upload is disabled. Connect real OAuth accounts and add a human approval gate
          before enabling publishing.
        </span>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'minmax(320px, 420px) minmax(0, 1fr)',
        gap: 14,
        alignItems: 'start',
        marginBottom: 24,
      }}>
        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--text-primary)' }}>
            Real daily package
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>
            Uses the configured real trend discovery provider, OpenAI script generation, and the real renderer
            when available. No mock trends, fake videos, uploads, or seeded content are created.
          </div>
          <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
            POST /api/daily-package
          </div>
          <button
            className={`generate-btn${state === 'ready' ? ' done' : ' idle'}`}
            onClick={handleGenerateTodaySix}
            disabled={state === 'pending'}
            style={{ width: '100%' }}
          >
            <span className="generate-btn-dot" style={{
              background: state === 'ready' ? 'var(--green)'
                : state === 'error' ? 'var(--red)'
                : '#15121f',
            }} />
            {state === 'idle' && "Generate Today's 6"}
            {state === 'pending' && 'Generating 6 reels...'}
            {state === 'ready' && 'Package ready'}
            {state === 'error' && 'Generation failed'}
          </button>
          <button
            className={`generate-btn${downloadState === 'ready' ? ' done' : ' idle'}`}
            onClick={handleDownloadZip}
            disabled={!pkg || downloadState === 'pending'}
            style={{
              width: '100%',
              opacity: pkg ? 1 : 0.55,
              cursor: pkg ? 'pointer' : 'not-allowed',
            }}
          >
            <span className="generate-btn-dot" style={{
              background: downloadState === 'ready' ? 'var(--green)'
                : downloadState === 'error' ? 'var(--red)'
                : '#15121f',
            }} />
            {downloadState === 'pending' ? 'Downloading...' : 'Download ZIP'}
          </button>
          <button
            className={`generate-btn${renderState === 'ready' ? ' done' : ' idle'}`}
            onClick={handleRenderReel01}
            disabled={!pkg || renderState === 'pending' || reel01?.has_video}
            style={{
              width: '100%',
              opacity: pkg && !reel01?.has_video ? 1 : 0.55,
              cursor: pkg && !reel01?.has_video ? 'pointer' : 'not-allowed',
            }}
          >
            <span className="generate-btn-dot" style={{
              background: renderState === 'ready' ? 'var(--green)'
                : renderState === 'error' ? 'var(--red)'
                : renderState === 'pending' ? 'var(--accent)'
                : '#15121f',
            }} />
            {renderState === 'pending' && 'Rendering reel-01...'}
            {renderState === 'ready' && 'reel-01 rendered'}
            {renderState === 'error' && 'Render failed'}
            {renderState === 'idle' && (reel01?.has_video ? 'reel-01 rendered' : 'Render reel-01')}
          </button>
          {renderJob && (
            <div style={{ fontSize: 11, color: renderState === 'error' ? 'var(--red)' : 'var(--text-dim)', fontFamily: 'var(--font-mono)', lineHeight: 1.5 }}>
              {renderJob.status} · {renderJob.message}
              {renderJob.render_error && (
                <>
                  <br />
                  render_error: {renderJob.render_error}
                </>
              )}
            </div>
          )}
          {pkg && (
            <div style={{ fontSize: 11, color: 'var(--green)', fontFamily: 'var(--font-mono)' }}>
              {pkg.zip_filename} · {pkg.included_files.length} file(s)
            </div>
          )}
          {error && (
            <div style={{
              fontSize: 11,
              color: 'var(--red)',
              lineHeight: 1.6,
              background: 'rgba(232,115,107,0.08)',
              border: '1px solid rgba(232,115,107,0.25)',
              borderRadius: 6,
              padding: '8px 10px',
            }}>
              {error}
            </div>
          )}
        </div>

        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, alignItems: 'center' }}>
            <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--text-primary)' }}>
              Package status
            </div>
            <span style={{
              fontSize: 10,
              fontFamily: 'var(--font-mono)',
              color: pkg?.status === 'ready' ? 'var(--green)' : 'var(--text-dim)',
            }}>
              {pkg?.status ?? (state === 'pending' ? 'generating' : 'not_started')}
            </span>
          </div>

          {state === 'pending' && (
            <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>
              Generating OpenAI script packages and writing the ZIP. Rendering stays deferred until you start reel-01.
            </div>
          )}

          {!pkg && state !== 'pending' && (
            <div className="empty-state" style={{ margin: 0 }}>
              <div className="empty-icon">6</div>
              <div className="empty-title">No daily ZIP generated yet.</div>
              <div className="empty-desc">Run Generate Today's 6 to build the real package.</div>
            </div>
          )}

          {pkg && (
            <>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>
                {pkg.message}
              </div>
              {renderFailures.length > 0 && (
                <div style={{
                  fontSize: 11,
                  color: '#eab86a',
                  lineHeight: 1.6,
                  background: 'rgba(234,184,106,0.08)',
                  border: '1px solid rgba(234,184,106,0.25)',
                  borderRadius: 6,
                  padding: '8px 10px',
                }}>
                  Video rendering did not complete for reel(s): {renderFailures.map((reel) => reel.rank).join(', ')}.
                  Text assets and trend evidence are included without fake video files.
                </div>
              )}
              <div style={{ display: 'grid', gap: 8 }}>
                {pkg.reels.map((reel) => (
                  <div
                    key={`${reel.rank}-${reel.candidate_id}`}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '34px minmax(0, 1fr) auto',
                      gap: 10,
                      alignItems: 'center',
                      border: '1px solid var(--border-card)',
                      borderRadius: 6,
                      padding: '9px 10px',
                      background: 'var(--bg-subtle)',
                    }}
                  >
                    <div style={{
                      width: 24,
                      height: 24,
                      borderRadius: 6,
                      display: 'grid',
                      placeItems: 'center',
                      fontFamily: 'var(--font-mono)',
                      fontSize: 11,
                      color: 'var(--text-primary)',
                      background: '#15121f',
                    }}>
                      {reel.rank}
                    </div>
                    <div style={{ minWidth: 0 }}>
                      <div style={{
                        fontSize: 12,
                        fontWeight: 650,
                        color: 'var(--text-primary)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}>
                        {reel.title}
                      </div>
                      <div style={{
                        fontSize: 10,
                        color: 'var(--text-dim)',
                        fontFamily: 'var(--font-mono)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}>
                      {reel.source} · {reel.candidate_id}
                        {reel.has_video && reel.resolution && (
                          <> · {reel.resolution}{reel.duration_seconds ? ` · ${Math.round(reel.duration_seconds)}s` : ''}</>
                        )}
                      </div>
                      {reel.render_error && (
                        <div style={{
                          fontSize: 10,
                          color: 'var(--red)',
                          fontFamily: 'var(--font-mono)',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}>
                          render_error: {reel.render_error}
                        </div>
                      )}
                    </div>
                    <div style={{
                      fontSize: 10,
                      fontFamily: 'var(--font-mono)',
                      color: toneForRender(reel.render_status),
                      whiteSpace: 'nowrap',
                    }}>
                      {reel.has_video ? (reel.video_file ?? 'video.mp4') : reel.render_status}
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </section>
  );
}
