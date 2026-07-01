import { useState } from 'react';
import { requestBatchZip, requestBatchPublish, ApiError } from '../lib/api/client';

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
            No uploads have been attempted. Connect accounts, configure provider keys,
            and create real approved reels before requesting publish jobs.
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
            {uploadState === 'idle'    && 'Request upload jobs'}
            {uploadState === 'pending' && 'Queuing jobs…'}
            {uploadState === 'done'    && `Backend returned ${uploadJobIds.length} job(s)`}
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
            Build a ZIP only from real backend batch artifacts. If no real batch exists,
            the backend returns an error instead of a fake package.
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
          <span style={{ fontSize: 10, fontFamily: 'var(--font-mono)', color: 'var(--text-dim)' }}>0 real videos</span>
        </div>
        <div className="empty-state" style={{ margin: 0, borderRadius: 0 }}>
          <div className="empty-icon">BP</div>
          <div className="empty-title">No batch runs yet.</div>
          <div className="empty-desc">No uploads have been attempted. Connect accounts and configure provider keys first.</div>
        </div>
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
