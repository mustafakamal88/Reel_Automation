import { useState } from 'react';
import type { BatchVideo, BatchStatus } from '../types';
import { requestBatchZip, requestBatchPublish, ApiError } from '../lib/api/client';

// Empty-state placeholder rows — shown when no real batch is available from the backend.
// These are NOT real videos and do NOT represent successful operations.
const EMPTY_STATE_ROWS: BatchVideo[] = [
  { id: 'placeholder-1', rank: 1, title: '— pending batch generation —', status: 'draft', platforms: [] },
  { id: 'placeholder-2', rank: 2, title: '— pending batch generation —', status: 'draft', platforms: [] },
  { id: 'placeholder-3', rank: 3, title: '— pending batch generation —', status: 'draft', platforms: [] },
];

const STATUS_LABEL: Record<BatchStatus, { label: string; color: string }> = {
  draft:        { label: 'Draft',        color: 'var(--text-dim)' },
  rendering:    { label: 'Rendering…',   color: 'var(--accent)' },
  ready:        { label: 'Ready',        color: 'var(--green)' },
  zipped:       { label: 'Zipped',       color: 'var(--green)' },
  queued:       { label: 'Queued',       color: 'var(--accent)' },
  publishing:   { label: 'Publishing…',  color: 'var(--accent)' },
  published:    { label: 'Published',    color: 'var(--green)' },
  failed:       { label: 'Failed',       color: 'var(--red)' },
  needs_review: { label: 'Needs review', color: 'var(--yellow)' },
};

const PLATFORM_LABELS: Record<string, string> = {
  yt: 'YT', tt: 'TT', ig: 'IG', fb: 'FB', th: 'TH', x: 'X',
};

function VideoRow({ video }: { video: BatchVideo }) {
  const s = STATUS_LABEL[video.status] ?? { label: video.status, color: 'var(--text-dim)' };
  const isPlaceholder = video.id.startsWith('placeholder-');
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 12,
      padding: '12px 16px', borderBottom: '1px solid var(--border-card)',
      opacity: isPlaceholder ? 0.4 : 1,
    }}>
      <span style={{
        fontFamily: 'var(--font-mono)', fontSize: 11,
        color: 'var(--text-dimmer)', width: 20, flexShrink: 0,
      }}>
        {String(video.rank).padStart(2, '0')}
      </span>
      <span style={{ flex: 1, fontSize: 13, color: 'var(--text-primary)', fontWeight: 500 }}>
        {video.title}
      </span>
      <div style={{ display: 'flex', gap: 4, flexShrink: 0 }}>
        {video.platforms.map(p => (
          <span key={p} style={{
            fontSize: 10, fontFamily: 'var(--font-mono)',
            color: 'var(--text-muted)', background: 'var(--bg-subtle)',
            border: '1px solid var(--border-strong)', borderRadius: 3, padding: '1px 5px',
          }}>
            {PLATFORM_LABELS[p] ?? p.toUpperCase()}
          </span>
        ))}
      </div>
      <span style={{
        fontSize: 11, fontFamily: 'var(--font-mono)',
        color: s.color, flexShrink: 0, width: 80, textAlign: 'right',
      }}>
        {s.label}
      </span>
    </div>
  );
}

type ActionState = 'idle' | 'pending' | 'done' | 'error';

export function DailyBatchPage() {
  const [uploadState, setUploadState] = useState<ActionState>('idle');
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadJobIds, setUploadJobIds] = useState<string[]>([]);

  const [zipState, setZipState] = useState<ActionState>('idle');
  const [zipError, setZipError] = useState<string | null>(null);
  const [zipJobId, setZipJobId] = useState<string | null>(null);

  const today = new Date().toISOString().split('T')[0];

  async function handleAutoUpload() {
    setUploadState('pending');
    setUploadError(null);
    setUploadJobIds([]);
    try {
      const res = await requestBatchPublish(today);
      setUploadJobIds(res.job_ids ?? []);
      setUploadState('done');
    } catch (err) {
      let msg = 'Publish request failed';
      if (err instanceof ApiError) {
        if (err.isBackendOffline) msg = 'Backend offline — run: cd backend && go run ./cmd/api';
        else if (err.isNotImplemented) msg = 'Publish not yet implemented on backend (worker queue required).';
        else msg = err.message;
      } else if (err instanceof Error) {
        msg = err.message;
      }
      setUploadError(msg);
      setUploadState('error');
    }
  }

  async function handleDownloadZip() {
    setZipState('pending');
    setZipError(null);
    setZipJobId(null);
    try {
      const res = await requestBatchZip(today);
      setZipJobId(res.job_id ?? null);
      setZipState('done');
    } catch (err) {
      let msg = 'ZIP request failed';
      if (err instanceof ApiError) {
        if (err.isBackendOffline) msg = 'Backend offline — run: cd backend && go run ./cmd/api';
        else if (err.isNotImplemented) msg = 'ZIP creation not yet implemented on backend (worker queue required).';
        else msg = err.message;
      } else if (err instanceof Error) {
        msg = err.message;
      }
      setZipError(msg);
      setZipState('error');
    }
  }

  return (
    <section className="page-section">
      <div className="int-section-header">
        <div className="int-section-title">Daily Batch — {today}</div>
        <div className="int-section-sub">
          Auto-upload to all connected platforms, or download a ZIP package for manual publishing.
        </div>
      </div>

      <div className="security-inline-warning" style={{ marginBottom: 16 }}>
        <span className="security-warning-icon">⚠</span>
        <span>
          <strong>Compliance required before publishing:</strong> AI-generated content disclosure,
          no impersonation, no copyrighted music without rights, no fake engagement.
          Human approval is required before auto-publish is enabled.
        </span>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
        gap: 14, marginBottom: 24,
      }}>
        {/* Auto-upload card */}
        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--text-primary)' }}>
            Auto-upload
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>
            Queue all ready videos for upload to every connected platform.
            Jobs run via the Go backend — no browser tab needs to stay open.
          </div>
          <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
            POST /batches/{today}/publish
          </div>
          <button
            className={`generate-btn${uploadState === 'done' ? ' done' : ' idle'}`}
            onClick={handleAutoUpload}
            disabled={uploadState === 'pending'}
            style={{ width: '100%' }}
          >
            <span className="generate-btn-dot" style={{
              background: uploadState === 'done' ? 'var(--green)'
                : uploadState === 'error' ? 'var(--red)'
                : '#15121f',
            }} />
            {uploadState === 'idle'    && 'Auto-upload to all platforms'}
            {uploadState === 'pending' && 'Queuing jobs…'}
            {uploadState === 'done'    && `Jobs queued (${uploadJobIds.length})`}
            {uploadState === 'error'   && 'Request failed'}
          </button>
          {uploadError && (
            <div style={{
              fontSize: 11, color: 'var(--red)', lineHeight: 1.6,
              background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
              borderRadius: 6, padding: '8px 10px',
            }}>
              {uploadError}
            </div>
          )}
          {uploadState === 'done' && uploadJobIds.length > 0 && (
            <div style={{ fontSize: 11, color: 'var(--green)', fontFamily: 'var(--font-mono)' }}>
              Job IDs: {uploadJobIds.join(', ')}
            </div>
          )}
          <div style={{ fontSize: 11, color: 'var(--text-dim)', lineHeight: 1.5 }}>
            Requires: connected platform accounts · human approval gate · Go backend running
          </div>
        </div>

        {/* Download ZIP card */}
        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--text-primary)' }}>
            Download ZIP
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>
            Download all videos as a structured ZIP package for manual review and publishing.
            Includes metadata, captions, thumbnails, and platform-specific JSON.
          </div>
          <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
            POST /batches/{today}/zip → poll GET /jobs/{'{jobID}'}
          </div>
          <button
            className={`generate-btn${zipState === 'done' ? ' done' : ' idle'}`}
            onClick={handleDownloadZip}
            disabled={zipState === 'pending'}
            style={{ width: '100%' }}
          >
            <span className="generate-btn-dot" style={{
              background: zipState === 'done' ? 'var(--green)'
                : zipState === 'error' ? 'var(--red)'
                : '#15121f',
            }} />
            {zipState === 'idle'    && 'Build ZIP package'}
            {zipState === 'pending' && 'Building ZIP…'}
            {zipState === 'done'    && 'ZIP job queued'}
            {zipState === 'error'   && 'Request failed'}
          </button>
          {zipError && (
            <div style={{
              fontSize: 11, color: 'var(--red)', lineHeight: 1.6,
              background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)',
              borderRadius: 6, padding: '8px 10px',
            }}>
              {zipError}
            </div>
          )}
          {zipState === 'done' && zipJobId && (
            <div style={{ fontSize: 11, color: 'var(--green)', fontFamily: 'var(--font-mono)' }}>
              Job ID: {zipJobId} — poll GET /jobs/{zipJobId}
            </div>
          )}
          <div style={{ fontSize: 11, color: 'var(--text-dim)', lineHeight: 1.5 }}>
            ZIP includes: video.mp4 · thumbnail.jpg · title.txt · description.txt ·
            hashtags.txt · captions.srt · platforms.json · compliance-checklist.json
          </div>
        </div>
      </div>

      {/* Video list — empty state pending backend batch pipeline */}
      <div className="settings-card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{
          padding: '12px 16px', borderBottom: '1px solid var(--border-card)',
          fontWeight: 600, fontSize: 13, color: 'var(--text-secondary)',
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          Today's videos
          <span style={{
            fontSize: 10, fontFamily: 'var(--font-mono)',
            color: 'var(--text-dim)', background: 'var(--bg-subtle)',
            border: '1px solid var(--border-strong)', borderRadius: 3, padding: '1px 6px',
          }}>
            empty state — pending backend batch pipeline
          </span>
        </div>
        {EMPTY_STATE_ROWS.map(v => <VideoRow key={v.id} video={v} />)}
      </div>

      <div style={{ marginTop: 16 }}>
        <div style={{
          fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)',
          display: 'inline-flex', alignItems: 'center', gap: 6,
          background: 'var(--bg-subtle)', border: '1px solid var(--border-strong)',
          borderRadius: 4, padding: '3px 8px',
        }}>
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: 'var(--text-dim)', display: 'inline-block' }} />
          No batch ready — start the Go backend and run the batch generation pipeline
        </div>
      </div>
    </section>
  );
}
