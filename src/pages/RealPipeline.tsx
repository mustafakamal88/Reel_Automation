import { useCallback, useEffect, useState } from 'react';
import {
  ApiError,
  type TrendSource, type TrendItem, type TopicScore, type DailyBatchV2,
  type ReelPlan, type VideoJob, type ExportJob, type PublishJobV2,
  getTrendSources, getTrends,
  scoreTopics, getTopicScores, createDailyBatch, getDailyBatches, getBatchReels,
  prepareVideoJob, getRenderJobs, renderReel, createBatchExport, getExportJobs, downloadExportZip,
  runRenderExportTest, downloadRenderExportTestZip, type RenderExportTestResponse,
  publishReel, getPublishJobs,
} from '../lib/api/client';

function errMsg(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    if (err.isBackendOffline) return 'Backend offline — run: cd backend && go run ./cmd/api';
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

function Pill({ children, tone = 'neutral' }: { children: React.ReactNode; tone?: 'neutral' | 'good' | 'warn' | 'bad' }) {
  const colors: Record<string, string> = {
    neutral: 'var(--text-dim)',
    good: 'var(--green)',
    warn: 'var(--accent)',
    bad: 'var(--red)',
  };
  return (
    <span style={{
      fontSize: 10, fontFamily: 'var(--font-mono)', color: colors[tone],
      background: 'var(--bg-subtle)', border: '1px solid var(--border-strong)',
      borderRadius: 3, padding: '2px 6px', whiteSpace: 'nowrap',
    }}>
      {children}
    </span>
  );
}

function toneForStatus(status: string): 'neutral' | 'good' | 'warn' | 'bad' {
  if (['scored', 'ready', 'draft_ready', 'connected', 'created', 'queued', 'done', 'completed'].includes(status)) return 'good';
  if ([
    'new', 'draft', 'planned', 'pending_provider_connection', 'video_requested', 'zip_generation_not_implemented',
    'artifact_missing', 'video_artifact_missing', 'thumbnail_artifact_missing', 'media_artifacts_missing',
    'rendering', 'packaging_zip', 'fallback', 'audio_artifact_missing', 'renderer_not_available',
  ].includes(status)) return 'warn';
  if (['rejected', 'failed', 'platform_not_connected', 'provider_not_connected', 'provider_missing'].includes(status)) return 'bad';
  return 'neutral';
}

function Card({ title, sub, children }: { title: string; sub?: string; children: React.ReactNode }) {
  return (
    <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 16 }}>
      <div>
        <div style={{ fontWeight: 700, fontSize: 14, color: 'var(--text-primary)' }}>{title}</div>
        {sub && <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>{sub}</div>}
      </div>
      {children}
    </div>
  );
}

function ActionButton({ label, busyLabel, busy, onClick, disabled }: {
  label: string; busyLabel: string; busy: boolean; onClick: () => void; disabled?: boolean;
}) {
  return (
    <button
      className={`generate-btn idle`}
      onClick={onClick}
      disabled={busy || disabled}
      style={{ alignSelf: 'flex-start' }}
      type="button"
    >
      <span className="generate-btn-dot" style={{ background: '#15121f' }} />
      {busy ? busyLabel : label}
    </button>
  );
}

export function RealPipelinePage() {
  const [sources, setSources] = useState<TrendSource[]>([]);
  const [trends, setTrends] = useState<TrendItem[]>([]);
  const [scores, setScores] = useState<TopicScore[]>([]);
  const [batches, setBatches] = useState<DailyBatchV2[]>([]);
  const [reels, setReels] = useState<ReelPlan[]>([]);
  const [videoJobs, setVideoJobs] = useState<VideoJob[]>([]);
  const [exportJobs, setExportJobs] = useState<ExportJob[]>([]);
  const [publishJobs, setPublishJobs] = useState<PublishJobV2[]>([]);

  const [loadError, setLoadError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionNote, setActionNote] = useState<string | null>(null);
  const [exportTestState, setExportTestState] = useState<'idle' | 'rendering' | 'packaging_zip' | 'ready' | 'fallback' | 'provider_missing' | 'failed'>('idle');
  const [exportTestResult, setExportTestResult] = useState<RenderExportTestResponse | null>(null);

  const today = new Date().toISOString().split('T')[0];
  const currentBatch = batches.find(b => b.batch_date === today) ?? batches[0];

  const loadAll = useCallback(async () => {
    try {
      const [s, t, sc, b, vj, ej, pj] = await Promise.all([
        getTrendSources(), getTrends(), getTopicScores(), getDailyBatches(),
        getRenderJobs(), getExportJobs(), getPublishJobs(),
      ]);
      setSources(s.trend_sources);
      setTrends(t.trend_items);
      setScores(sc.topic_scores);
      setBatches(b.daily_batches);
      setVideoJobs(vj.render_jobs);
      setExportJobs(ej.export_jobs);
      setPublishJobs(pj.publish_jobs);
      setLoadError(null);
    } catch (err) {
      setLoadError(errMsg(err, 'Failed to load pipeline state'));
    }
  }, []);

  useEffect(() => { loadAll(); }, [loadAll]);

  useEffect(() => {
    if (!currentBatch) { setReels([]); return; }
    getBatchReels(currentBatch.id).then(r => setReels(r.reel_plans)).catch(() => setReels([]));
  }, [currentBatch?.id]);

  async function runAction(key: string, fn: () => Promise<void>): Promise<boolean> {
    setBusy(key);
    setActionError(null);
    setActionNote(null);
    try {
      await fn();
      return true;
    } catch (err) {
      setActionError(errMsg(err, 'Action failed'));
      return false;
    } finally {
      setBusy(null);
    }
  }

  async function handleScoreTopics() {
    await runAction('score', async () => {
      const res = await scoreTopics();
      setActionNote(`Scored ${res.scored} topic(s).`);
      await loadAll();
    });
  }

  async function handleCreateBatch() {
    await runAction('batch', async () => {
      const res = await createDailyBatch();
      setActionNote(res.already_existed
        ? `Today's batch already exists (${res.batch.reel_count} reel${res.batch.reel_count === 1 ? '' : 's'}).`
        : `Created today's batch with ${res.batch.reel_count} reel(s).`);
      await loadAll();
    });
  }

  async function handlePrepareVideoJob(reelID: string) {
    await runAction(`video-${reelID}`, async () => {
      await prepareVideoJob(reelID);
      const [vj, r] = await Promise.all([getRenderJobs(), currentBatch ? getBatchReels(currentBatch.id) : null]);
      setVideoJobs(vj.render_jobs);
      if (r) setReels(r.reel_plans);
    });
  }

  async function handleRenderMedia(reelID: string) {
    await runAction(`render-${reelID}`, async () => {
      const res = await renderReel(reelID);
      setActionNote(res.status === 'completed'
        ? 'Render completed: real video.mp4 and thumbnail.png were written.'
        : `Render did not complete: ${res.status}${res.notes ? ` — ${res.notes}` : ''}`);
      const [vj, r] = await Promise.all([getRenderJobs(), currentBatch ? getBatchReels(currentBatch.id) : null]);
      setVideoJobs(vj.render_jobs);
      if (r) setReels(r.reel_plans);
    });
  }

  async function handlePublish(reelID: string) {
    await runAction(`publish-${reelID}`, async () => {
      const job = await publishReel(reelID);
      setActionNote(job.status === 'platform_not_connected'
        ? `Platform "${job.platform}" is not connected — publish recorded as not_connected.`
        : `Publish job created: ${job.status}`);
      const pj = await getPublishJobs();
      setPublishJobs(pj.publish_jobs);
    });
  }

  async function handleExport() {
    if (!currentBatch) return;
    await runAction('export', async () => {
      const res = await createBatchExport(currentBatch.id);
      setActionNote(`Export job created: ${res.export_job.status}`);
      const [ej, r] = await Promise.all([getExportJobs(), getBatchReels(currentBatch.id)]);
      setExportJobs(ej.export_jobs);
      setReels(r.reel_plans);
    });
  }

  async function handleDownloadExport(job: ExportJob) {
    await runAction('download', async () => {
      const filename = `trendcortex-batch-${currentBatch?.batch_date ?? job.daily_batch_id}.zip`;
      await downloadExportZip(job.id, filename);
      setActionNote('ZIP download started.');
    });
  }

  async function handleRenderExportTest() {
    const sourceReel = reels[0];
    setExportTestState('rendering');
    const ok = await runAction('export-test', async () => {
      const result = await runRenderExportTest({
        topic: sourceReel?.title_idea ?? 'TrendCortex end-to-end export test',
        title: sourceReel?.title_idea ?? 'TrendCortex end-to-end export test',
        script: sourceReel?.script_outline ?? 'This is a TrendCortex render plus ZIP export test.',
        caption: sourceReel?.description_draft ?? 'End-to-end render and ZIP export smoke test.',
        target_platforms: ['youtube', 'tiktok', 'instagram', 'facebook', 'x'],
        style: sourceReel?.thumbnail_idea ?? 'vertical editorial social video',
        format: 'vertical_9_16',
        number_of_reels: 1,
        allow_local_fallback: true,
      });
      setExportTestState('packaging_zip');
      setExportTestResult(result);
      if (result.render_status === 'provider_not_connected' || result.render_status === 'renderer_not_available') {
        setExportTestState('provider_missing');
      } else if (result.provider === 'local_ffmpeg_test') {
        setExportTestState('fallback');
      } else if (result.success) {
        setExportTestState('ready');
      } else {
        setExportTestState('failed');
      }
      setActionNote(result.fallback_reason
        ? `Export test ZIP ready using ${result.provider}. Fallback reason: ${result.fallback_reason}`
        : `Export test ZIP ready using ${result.provider}.`);
    });
    if (!ok) {
      setExportTestState('failed');
    }
  }

  async function handleDownloadExportTest() {
    if (!exportTestResult) return;
    await runAction('export-test-download', async () => {
      await downloadRenderExportTestZip(exportTestResult.download_url, exportTestResult.zip_filename);
      setActionNote('Export test ZIP download started.');
    });
  }

  const newTrendCount = trends.filter(t => t.status === 'new').length;
  const scoredCount = trends.filter(t => t.status === 'scored').length;
  const batchExportJob = exportJobs.find(j => j.daily_batch_id === currentBatch?.id);
  const reelPublishJobs = (reelID: string) => publishJobs.filter(j => j.reel_plan_id === reelID);
  const reelVideoJob = (reelID: string) => videoJobs.find(j => j.reel_plan_id === reelID);
  const missingVideoReels = reels.filter(r => !r.video_artifact_path);
  const missingThumbnailReels = reels.filter(r => !r.thumbnail_artifact_path);

  return (
    <section className="page-section">
      <div className="int-section-header">
        <div className="int-section-title">Real Automation Pipeline</div>
        <div className="int-section-sub">
          Trend discovery → deterministic scoring → daily 6-reel batch → video/export/publish job placeholders.
          Every state shown here is read from the Go backend's Postgres database — nothing is faked.
        </div>
      </div>

      {loadError && (
        <div className="security-inline-warning" style={{ marginBottom: 16 }}>
          <span className="security-warning-icon">⚠</span>
          <span>{loadError}</span>
        </div>
      )}
      {actionError && (
        <div style={{
          fontSize: 12, color: 'var(--red)', lineHeight: 1.6, marginBottom: 16,
          background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
          borderRadius: 6, padding: '8px 10px',
        }}>
          {actionError}
        </div>
      )}
      {actionNote && (
        <div style={{
          fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.6, marginBottom: 16,
          background: 'var(--bg-subtle)', border: '1px solid var(--border-strong)',
          borderRadius: 6, padding: '8px 10px',
        }}>
          {actionNote}
        </div>
      )}

      {/* 1. Trend Sources */}
      <Card title="Trend Sources" sub="No live source integration is wired in yet. Manual source rows may exist in the backend, but the frontend does not create sample trend data.">
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 4 }}>
          {sources.length === 0 && <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No trend sources yet.</span>}
          {sources.map(s => (
            <Pill key={s.id} tone={s.source_type === 'manual' ? 'good' : 'bad'}>
              {s.name} · {s.source_type} · conf {s.confidence.toFixed(2)}
            </Pill>
          ))}
        </div>
        <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No real trend collection has run yet.</span>
      </Card>

      {/* 2. Discovered Trends */}
      <Card title="Discovered Trends" sub={`${trends.length} total · ${newTrendCount} new · ${scoredCount} scored`}>
        {trends.length === 0 ? (
          <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No live trend data connected yet.</span>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {trends.slice(0, 12).map(t => (
              <div key={t.id} style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12, padding: '4px 0', borderBottom: '1px solid var(--border-card)' }}>
                <span style={{ flex: 1, color: 'var(--text-primary)' }}>{t.topic}</span>
                <span style={{ color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', fontSize: 11 }}>{t.platform_hint || '—'}</span>
                <span style={{ color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', fontSize: 11 }}>v{t.velocity}</span>
                <Pill tone={toneForStatus(t.status)}>{t.status}</Pill>
              </div>
            ))}
          </div>
        )}
        <ActionButton
          label="Score topics"
          busyLabel="Scoring…"
          busy={busy === 'score'}
          disabled={newTrendCount === 0}
          onClick={handleScoreTopics}
        />
      </Card>

      {/* 3. Topic Scores */}
      <Card title="Topic Scores" sub="Deterministic score: velocity + source confidence + platform fit + safety + watch-time potential + competition ease.">
        {scores.length === 0 ? (
          <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No scored topics yet.</span>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {scores.slice(0, 10).map(s => {
              const t = trends.find(x => x.id === s.trend_item_id);
              return (
                <div key={s.id} style={{ fontSize: 12, padding: '4px 0', borderBottom: '1px solid var(--border-card)' }}>
                  <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                    <span style={{ flex: 1, color: 'var(--text-primary)' }}>{t?.topic ?? s.trend_item_id}</span>
                    <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--accent)', fontWeight: 700 }}>{s.total_score.toFixed(1)}</span>
                  </div>
                  <div style={{ fontSize: 10, color: 'var(--text-dim)', marginTop: 2 }}>{s.reason}</div>
                </div>
              );
            })}
          </div>
        )}
      </Card>

      {/* 4. Daily Batch */}
      <Card title={`Daily Batch — ${today}`} sub="Selects up to the 6 highest-scored eligible topics into reel plans. Calling this again for the same date is a no-op.">
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          {currentBatch ? (
            <>
              <Pill tone={toneForStatus(currentBatch.status)}>{currentBatch.status}</Pill>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{currentBatch.batch_date} · {currentBatch.reel_count} reel(s)</span>
            </>
          ) : (
            <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No batch for today yet.</span>
          )}
        </div>
        <ActionButton
          label="Generate today's batch"
          busyLabel="Generating…"
          busy={busy === 'batch'}
          onClick={handleCreateBatch}
        />
      </Card>

      {/* 5. Six Reel Plans */}
      <Card title="Reel Plans" sub="Draft copy is a placeholder for human review — nothing here is auto-published.">
        {reels.length === 0 ? (
          <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No reel plans for today's batch yet.</span>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 10 }}>
            {reels.map(reel => {
              const vj = reelVideoJob(reel.id);
              const pjs = reelPublishJobs(reel.id);
              const latestPublish = pjs[0];
              return (
                <div key={reel.id} style={{
                  border: '1px solid var(--border-card)', borderRadius: 8, padding: 12,
                  display: 'flex', flexDirection: 'column', gap: 6,
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dimmer)' }}>#{reel.rank}</span>
                    <Pill>{reel.platform || 'unspecified'}</Pill>
                  </div>
                  <div style={{ fontWeight: 600, fontSize: 13, color: 'var(--text-primary)' }}>{reel.title_idea}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', whiteSpace: 'pre-line' }}>{reel.script_outline}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-dim)' }}>{reel.description_draft}</div>
                  <div style={{ fontSize: 11, color: 'var(--accent)', fontFamily: 'var(--font-mono)' }}>{reel.hashtags_draft}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-dim)', fontStyle: 'italic' }}>{reel.thumbnail_idea}</div>

                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 4 }}>
                    <Pill tone={toneForStatus(vj?.status ?? 'not_requested')}>render: {vj?.status ?? 'not_requested'}</Pill>
                    {vj?.provider && <Pill>{vj.provider}</Pill>}
                    <Pill tone={toneForStatus(latestPublish?.status ?? 'not_requested')}>publish: {latestPublish?.status ?? 'not_requested'}</Pill>
                  </div>
                  {vj?.notes && (
                    <div style={{ fontSize: 10, color: 'var(--text-dim)', lineHeight: 1.5 }}>
                      {vj.notes}
                    </div>
                  )}

                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                    <Pill tone={reel.video_artifact_path ? 'good' : 'warn'}>
                      video.mp4: {reel.video_artifact_path ? 'present' : 'missing'}
                    </Pill>
                    <Pill tone={reel.thumbnail_artifact_path ? 'good' : 'warn'}>
                      thumbnail.png: {reel.thumbnail_artifact_path ? 'present' : 'missing'}
                    </Pill>
                  </div>
                  <div style={{ fontSize: 10, color: 'var(--text-dimmer)', fontFamily: 'var(--font-mono)' }}>
                    expected: video.mp4 · thumbnail.png
                  </div>

                  <div style={{ display: 'flex', gap: 8, marginTop: 6, flexWrap: 'wrap' }}>
                    <button
                      className="generate-btn idle"
                      style={{ flex: '1 1 130px', fontSize: 11 }}
                      disabled={busy === `video-${reel.id}` || !!vj}
                      onClick={() => handlePrepareVideoJob(reel.id)}
                      type="button"
                    >
                      <span className="generate-btn-dot" style={{ background: '#15121f' }} />
                      {vj ? 'Video requested' : busy === `video-${reel.id}` ? 'Requesting…' : 'Prepare video job'}
                    </button>
                    <button
                      className="generate-btn idle"
                      style={{ flex: '1 1 130px', fontSize: 11 }}
                      disabled={busy === `render-${reel.id}`}
                      onClick={() => handleRenderMedia(reel.id)}
                      type="button"
                    >
                      <span className="generate-btn-dot" style={{ background: '#15121f' }} />
                      {busy === `render-${reel.id}` ? 'Rendering…' : 'Render media'}
                    </button>
                    <button
                      className="generate-btn idle"
                      style={{ flex: '1 1 130px', fontSize: 11 }}
                      disabled={busy === `publish-${reel.id}`}
                      onClick={() => handlePublish(reel.id)}
                      type="button"
                    >
                      <span className="generate-btn-dot" style={{ background: '#15121f' }} />
                      {busy === `publish-${reel.id}` ? 'Requesting…' : 'Request publish'}
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Card>

      {/* 6. Export Job Status */}
      <Card title="Phase 4D End-to-End Test" sub="Runs one backend request that tries provider rendering, clearly falls back to local FFmpeg test media when allowed, packages the result into a ZIP, and returns a download link.">
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <Pill tone={toneForStatus(exportTestState)}>{exportTestState}</Pill>
          {exportTestResult && <Pill tone={toneForStatus(exportTestResult.render_status)}>render: {exportTestResult.render_status}</Pill>}
          {exportTestResult && <Pill tone={toneForStatus(exportTestResult.export_status)}>zip: {exportTestResult.export_status}</Pill>}
          {exportTestResult?.provider && <Pill>{exportTestResult.provider}</Pill>}
        </div>
        {exportTestResult?.fallback_reason && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)', lineHeight: 1.5 }}>
            {exportTestResult.fallback_reason}
          </div>
        )}
        {exportTestResult?.zip_filename && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
            {exportTestResult.zip_filename} · {exportTestResult.included_files.length} file(s)
          </div>
        )}
        {exportTestResult && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {exportTestResult.included_files.slice(0, 10).map(name => <Pill key={name}>{name}</Pill>)}
            {exportTestResult.included_files.length > 10 && <Pill>+{exportTestResult.included_files.length - 10} more</Pill>}
          </div>
        )}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <ActionButton
            label="Run render + ZIP test"
            busyLabel="Rendering + packaging ZIP…"
            busy={busy === 'export-test'}
            onClick={handleRenderExportTest}
          />
          {exportTestResult?.download_url && (
            <ActionButton
              label="Download test ZIP"
              busyLabel="Downloading…"
              busy={busy === 'export-test-download'}
              onClick={handleDownloadExportTest}
            />
          )}
        </div>
      </Card>

      <Card title="Export Job (ZIP)" sub="A ZIP is only built when every reel has a real video.mp4 and thumbnail.png on disk. Otherwise this honestly reports exactly which reels are missing which file — never a fake download.">
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          {batchExportJob ? (
            <Pill tone={toneForStatus(batchExportJob.status)}>{batchExportJob.status}</Pill>
          ) : (
            <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No export job for today's batch yet.</span>
          )}
        </div>

        {batchExportJob?.status === 'completed' && batchExportJob.zip_path && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
            {batchExportJob.zip_path.split('/').pop()}
          </div>
        )}

        {missingVideoReels.length > 0 && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>
            Missing video.mp4 for reel(s): {missingVideoReels.map(r => `#${r.rank}`).join(', ')}
          </div>
        )}
        {missingThumbnailReels.length > 0 && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>
            Missing thumbnail.png for reel(s): {missingThumbnailReels.map(r => `#${r.rank}`).join(', ')}
          </div>
        )}

        <div style={{ display: 'flex', gap: 8 }}>
          <ActionButton
            label="Request export"
            busyLabel="Requesting…"
            busy={busy === 'export'}
            disabled={!currentBatch}
            onClick={handleExport}
          />
          {batchExportJob?.status === 'completed' && (
            <ActionButton
              label="Download ZIP"
              busyLabel="Downloading…"
              busy={busy === 'download'}
              onClick={() => handleDownloadExport(batchExportJob)}
            />
          )}
        </div>
      </Card>

      {/* 7. Publishing Job Status */}
      <Card title="Publish Jobs" sub="Publishing never pretends to succeed. Without a connected OAuth credential for the target platform, every request is honestly recorded as platform_not_connected.">
        {publishJobs.length === 0 ? (
          <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>No publish jobs yet.</span>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {publishJobs.slice(0, 10).map(j => (
              <div key={j.id} style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12, padding: '4px 0', borderBottom: '1px solid var(--border-card)' }}>
                <span style={{ flex: 1, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: 11 }}>{j.id.slice(0, 8)}</span>
                <Pill>{j.platform}</Pill>
                <Pill tone={toneForStatus(j.status)}>{j.status}</Pill>
              </div>
            ))}
          </div>
        )}
      </Card>
    </section>
  );
}
