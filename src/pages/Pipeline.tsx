import { useState } from 'react';
import {
  ApiError,
  downloadRenderExportTestZip,
  runRenderExportTest,
  type RenderExportTestResponse,
} from '../lib/api/client';

function errMsg(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    if (err.isBackendOffline) return 'Backend offline - run: cd backend && go run ./cmd/api';
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
  if (['ready', 'completed'].includes(status)) return 'good';
  if (['idle', 'rendering', 'packaging_zip', 'fallback', 'renderer_not_available'].includes(status)) return 'warn';
  if (['failed', 'provider_not_connected', 'provider_missing'].includes(status)) return 'bad';
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

type ExportTestState = 'idle' | 'rendering' | 'packaging_zip' | 'ready' | 'fallback' | 'provider_missing' | 'failed';

export function PipelinePage() {
  const [busy, setBusy] = useState(false);
  const [downloadBusy, setDownloadBusy] = useState(false);
  const [state, setState] = useState<ExportTestState>('idle');
  const [result, setResult] = useState<RenderExportTestResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);

  async function handleRun() {
    setBusy(true);
    setError(null);
    setNote(null);
    setState('rendering');
    try {
      const res = await runRenderExportTest({
        topic: 'TrendCortex end-to-end export test',
        title: 'TrendCortex end-to-end export test',
        script: 'This is a TrendCortex render plus ZIP export test.',
        caption: 'End-to-end render and ZIP export smoke test.',
        target_platforms: ['youtube', 'tiktok', 'instagram', 'facebook', 'x'],
        style: 'vertical editorial social video',
        format: 'vertical_9_16',
        number_of_reels: 1,
        allow_local_fallback: true,
      });
      setState('packaging_zip');
      setResult(res);
      if (res.render_status === 'provider_not_connected' || res.render_status === 'renderer_not_available') {
        setState('provider_missing');
      } else if (res.provider === 'local_ffmpeg_test') {
        setState('fallback');
      } else if (res.success) {
        setState('ready');
      } else {
        setState('failed');
      }
      setNote(res.fallback_reason
        ? `Export test ZIP ready using ${res.provider}. Fallback reason: ${res.fallback_reason}`
        : `Export test ZIP ready using ${res.provider}.`);
    } catch (err) {
      setState('failed');
      setError(errMsg(err, 'Render + ZIP test failed'));
    } finally {
      setBusy(false);
    }
  }

  async function handleDownload() {
    if (!result) return;
    setDownloadBusy(true);
    setError(null);
    try {
      await downloadRenderExportTestZip(result.download_url, result.zip_filename);
      setNote('Export test ZIP download started.');
    } catch (err) {
      setError(errMsg(err, 'ZIP download failed'));
    } finally {
      setDownloadBusy(false);
    }
  }

  return (
    <section className="page-section">
      <div className="empty-state" style={{ marginBottom: 16 }}>
        <div className="empty-icon">PL</div>
        <div className="empty-title">No real reels generated yet.</div>
        <div className="empty-desc">Run the Phase 4D render + ZIP test to generate the first local export.</div>
      </div>

      {error && (
        <div style={{
          fontSize: 12, color: 'var(--red)', lineHeight: 1.6, marginBottom: 16,
          background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
          borderRadius: 6, padding: '8px 10px',
        }}>
          {error}
        </div>
      )}
      {note && (
        <div style={{
          fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.6, marginBottom: 16,
          background: 'var(--bg-subtle)', border: '1px solid var(--border-strong)',
          borderRadius: 6, padding: '8px 10px',
        }}>
          {note}
        </div>
      )}

      <Card title="Phase 4D End-to-End Test" sub="Runs one backend request that tries provider rendering, clearly falls back to local FFmpeg test media when allowed, packages the result into a ZIP, and returns a download link.">
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <Pill tone={toneForStatus(state)}>{state}</Pill>
          {result && <Pill tone={toneForStatus(result.render_status)}>render: {result.render_status}</Pill>}
          {result && <Pill tone={toneForStatus(result.export_status)}>zip: {result.export_status}</Pill>}
          {result?.provider && <Pill>{result.provider}</Pill>}
        </div>
        {result?.fallback_reason && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)', lineHeight: 1.5 }}>
            {result.fallback_reason}
          </div>
        )}
        {result?.zip_filename && (
          <div style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
            {result.zip_filename} · {result.included_files.length} file(s)
          </div>
        )}
        {result && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {result.included_files.slice(0, 10).map(name => <Pill key={name}>{name}</Pill>)}
            {result.included_files.length > 10 && <Pill>+{result.included_files.length - 10} more</Pill>}
          </div>
        )}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="generate-btn idle" onClick={handleRun} disabled={busy} type="button">
            <span className="generate-btn-dot" style={{ background: '#15121f' }} />
            {busy ? 'Rendering + packaging ZIP...' : 'Run render + ZIP test'}
          </button>
          {result?.download_url && (
            <button className="generate-btn idle" onClick={handleDownload} disabled={downloadBusy} type="button">
              <span className="generate-btn-dot" style={{ background: '#15121f' }} />
              {downloadBusy ? 'Downloading...' : 'Download test ZIP'}
            </button>
          )}
        </div>
      </Card>
    </section>
  );
}
