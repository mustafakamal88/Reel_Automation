import { useEffect, useMemo, useState } from 'react';
import {
  ApiError,
  downloadClipStudioZip,
  generateClipStudio,
  getPlatformConnections,
  importClipStudioURL,
  uploadClipStudioSource,
  type ClipCTASize,
  type ClipLayoutMode,
  type ClipSourceModel,
  type ClipStudioGenerateResponse,
  type ClipStudioSourceResponse,
  type PlatformStatus,
} from '../lib/api/client';
import { storage } from '../lib/storage';
import {
  clipGenerateDisabledReason,
  clipPackageReady,
  getClipSourceStatus,
  sourceCanGenerate,
} from './ClipStudioState';

interface Props {
  onNavigate?: (view: 'connections') => void;
}

const PUBLISH_PLATFORMS = ['youtube', 'tiktok', 'instagram', 'facebook', 'x'] as const;
type PublishPlatform = typeof PUBLISH_PLATFORMS[number];
const DIRECT_PUBLISHING_ENABLED = false;

const PLATFORM_LABELS: Record<string, string> = {
  youtube: 'YouTube Shorts',
  tiktok: 'TikTok',
  instagram: 'Instagram Reels',
  facebook: 'Facebook Reels',
  x: 'X',
};

const SOURCE_MODELS: { value: ClipSourceModel; label: string }[] = [
  { value: 'user_upload', label: 'User upload' },
  { value: 'own_channel_source', label: 'Own channel' },
  { value: 'creative_commons', label: 'Creative Commons' },
  { value: 'public_domain', label: 'Public domain' },
  { value: 'licensed_source', label: 'Licensed source' },
  { value: 'external_url_pending_rights_confirmation', label: 'External URL pending rights' },
];

function errMsg(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    if (err.isBackendOffline) return 'Backend offline. Start the Go backend or check VITE_API_BASE_URL.';
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

function inputStyle(): React.CSSProperties {
  return {
    width: '100%',
    border: '1px solid var(--border-strong)',
    background: 'var(--bg-subtle)',
    color: 'var(--text-primary)',
    borderRadius: 6,
    padding: '10px 11px',
    fontSize: 12,
  };
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 5, fontSize: 11, color: 'var(--text-muted)' }}>
      <span>{label}</span>
      {children}
    </label>
  );
}

export function ClipStudioPage({ onNavigate }: Props) {
  const savedSettings = storage.getSettings();
  const [sourceUrl, setSourceUrl] = useState('');
  const [source, setSource] = useState<ClipStudioSourceResponse | null>(null);
  const [prompt, setPrompt] = useState('Make short branded clips with a strong hook and clear takeaway.');
  const [clipLength, setClipLength] = useState<'auto' | '15s' | '30s' | '60s' | '3min'>('auto');
  const [clipCount, setClipCount] = useState<1 | 3 | 6>(3);
  const [topText, setTopText] = useState(savedSettings.defaultTopText);
  const [bottomText, setBottomText] = useState(savedSettings.defaultBottomText);
  const [watermark, setWatermark] = useState(savedSettings.defaultWatermark);
  const [layoutMode, setLayoutMode] = useState<ClipLayoutMode>(savedSettings.defaultLayoutMode);
  const [captionText, setCaptionText] = useState('');
  const [ctaSize, setCtaSize] = useState<ClipCTASize>('small');
  const [rightsConfirmed, setRightsConfirmed] = useState(false);
  const [sourceModel, setSourceModel] = useState<ClipSourceModel>('user_upload');
  const [sourceTitle, setSourceTitle] = useState('');
  const [sourceCreator, setSourceCreator] = useState('');
  const [sourceLicense, setSourceLicense] = useState('');
  const [attributionText, setAttributionText] = useState('');
  const [copyrightOverlayText, setCopyrightOverlayText] = useState('');
  const [platformSource, setPlatformSource] = useState('');

  const [busy, setBusy] = useState(false);
  const [uploadBusy, setUploadBusy] = useState(false);
  const [urlImportBusy, setUrlImportBusy] = useState(false);
  const [downloadBusy, setDownloadBusy] = useState(false);
  const [publishOpen, setPublishOpen] = useState(false);
  const [connections, setConnections] = useState<PlatformStatus[]>([]);
  const [connectionsLoaded, setConnectionsLoaded] = useState(false);
  const [result, setResult] = useState<ClipStudioGenerateResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const packageReady = clipPackageReady(result);
  const connectedAccountPlatforms = connections.filter(conn => PUBLISH_PLATFORMS.includes(conn.platform as PublishPlatform) && conn.status === 'connected');
  const sourceStatus = useMemo(() => getClipSourceStatus(source, sourceUrl), [source, sourceUrl]);
  const generateDisabledReason = clipGenerateDisabledReason({ source, rightsConfirmed, prompt, busy, uploadBusy, urlImportBusy });
  const canGenerate = generateDisabledReason === null;
  const clipStatusMessage = useMemo(() => {
    if (packageReady) return 'Package ready. Download the ZIP when you are ready.';
    return generateDisabledReason || sourceStatus.message;
  }, [generateDisabledReason, packageReady, sourceStatus.message]);

  useEffect(() => {
    let cancelled = false;
    getPlatformConnections()
      .then(res => {
        if (!cancelled) setConnections(res.platforms);
      })
      .catch(() => {
        if (!cancelled) setConnections([]);
      })
      .finally(() => {
        if (!cancelled) setConnectionsLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleUpload(file: File | undefined) {
    if (!file) return;
    setUploadBusy(true);
    setError(null);
    setResult(null);
    try {
      const uploaded = await uploadClipStudioSource(file);
      setSource(uploaded);
      setSourceUrl('');
      setSourceModel('user_upload');
    } catch (err) {
      setError(errMsg(err, 'Upload failed'));
    } finally {
      setUploadBusy(false);
    }
  }

  async function handleImportURL() {
    const trimmedURL = sourceUrl.trim();
    if (!trimmedURL) return;
    setUrlImportBusy(true);
    setError(null);
    setResult(null);
    try {
      const imported = await importClipStudioURL({
        source_url: trimmedURL,
        rights_confirmed: rightsConfirmed,
        rights: {
          source_url: trimmedURL,
          source_title: sourceTitle,
          source_creator: sourceCreator,
          source_license: sourceLicense,
          attribution_text: attributionText,
          user_confirmed_rights: rightsConfirmed,
          copyright_overlay_text: copyrightOverlayText,
          platform_source: platformSource,
        },
      });
      setSource(imported);
      setSourceModel(imported.metadata.source_model || 'external_url_pending_rights_confirmation');
    } catch (err) {
      setError(errMsg(err, 'Source URL import failed'));
    } finally {
      setUrlImportBusy(false);
    }
  }

  async function handleGenerate() {
    if (!source || !sourceCanGenerate(source)) {
      setError(sourceStatus.message);
      return;
    }
    const activeSource = source;
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      const generated = await generateClipStudio({
        source_id: activeSource.source_id,
        prompt,
        clip_length: clipLength,
        clip_count: clipCount,
        branding: {
          top_banner_text: topText,
          bottom_banner_text: bottomText,
          watermark_text: watermark,
          cta_text: bottomText,
          font_style_preset: 'bold_editorial',
          top_banner_color: '#111827',
          bottom_banner_color: '#0f766e',
          cta_size: ctaSize,
        },
        caption_text: captionText,
        layout_mode: layoutMode,
        rights: {
          source_url: sourceUrl.trim() || activeSource.metadata.url,
          source_title: sourceTitle,
          source_creator: sourceCreator,
          source_license: sourceLicense,
          attribution_text: attributionText,
          user_confirmed_rights: rightsConfirmed,
          copyright_overlay_text: copyrightOverlayText,
          platform_source: platformSource,
        },
        rights_confirmed: rightsConfirmed,
        advanced: {
          source_model: sourceModel,
          source_title: sourceTitle,
          source_creator: sourceCreator,
          source_license: sourceLicense,
          attribution_text: attributionText,
          copyright_overlay_text: copyrightOverlayText,
          platform_source: platformSource,
        },
      });
      setResult(generated);
      if (generated.success) {
        storage.updateActivity(current => ({
          ...current,
          clipsGenerated: current.clipsGenerated + (generated.generated_clip_jobs?.length || clipCount),
          latestClipPackageGenerated: new Date().toISOString(),
        }));
      }
      if (!generated.success) setError(generated.notes || activeSource.message || 'Clip generation did not complete.');
    } catch (err) {
      setError(errMsg(err, 'Clip generation failed'));
    } finally {
      setBusy(false);
    }
  }

  async function handleDownload() {
    if (!result?.download_url || !result.zip_filename) return;
    setDownloadBusy(true);
    setError(null);
    try {
      await downloadClipStudioZip(result.download_url, result.zip_filename);
      storage.updateActivity(current => ({
        ...current,
        packagesDownloaded: current.packagesDownloaded + 1,
      }));
    } catch (err) {
      setError(errMsg(err, 'Clip ZIP download failed'));
    } finally {
      setDownloadBusy(false);
    }
  }

  return (
    <section className="page-section">
      <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 1040 }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 800, color: 'var(--text-primary)' }}>Clip from Video</div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>
            Upload a source video or provide a direct downloadable video URL, then export a branded ZIP package.
          </div>
        </div>

        {error && (
          <div style={{ fontSize: 12, color: 'var(--red)', lineHeight: 1.6, background: 'rgba(232,115,107,0.08)', border: '1px solid rgba(232,115,107,0.25)', borderRadius: 6, padding: '8px 10px' }}>
            {error}
          </div>
        )}

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 10, alignItems: 'end' }}>
          <Field label="Direct video URL or reference URL">
            <input
              value={sourceUrl}
              onChange={event => {
                setSourceUrl(event.target.value);
                setSource(null);
                setResult(null);
              }}
              placeholder="https://example.com/source.mp4"
              style={inputStyle()}
            />
          </Field>
          <Field label="Upload video">
            <input
              type="file"
              accept="video/mp4,video/quicktime,video/webm,.mp4,.mov,.webm"
              onChange={event => void handleUpload(event.target.files?.[0])}
              style={inputStyle()}
            />
          </Field>
        </div>

        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <button className="generate-btn idle" onClick={handleImportURL} disabled={!sourceUrl.trim() || urlImportBusy} type="button">
            {urlImportBusy ? 'Checking URL...' : 'Import URL'}
          </button>
        </div>

        <div className="clip-status-grid">
          <StatusPill label="Source ready" active={Boolean(source && sourceCanGenerate(source))} />
          <StatusPill label="Clips generated" active={Boolean(result?.success)} />
          <StatusPill label="Package ready" active={packageReady} />
          <StatusPill label={connectedAccountPlatforms.length > 0 ? 'Social account connected' : 'Social accounts not connected'} active neutral />
        </div>

        <div style={{ fontSize: 12, color: sourceStatus.tone === 'danger' ? 'var(--red)' : sourceStatus.tone === 'ready' ? 'var(--green)' : 'var(--text-muted)', background: sourceStatus.tone === 'danger' ? 'rgba(232,115,107,0.08)' : sourceStatus.tone === 'ready' ? 'rgba(95,211,154,0.08)' : 'var(--bg-subtle)', border: sourceStatus.tone === 'danger' ? '1px solid rgba(232,115,107,0.25)' : sourceStatus.tone === 'ready' ? '1px solid rgba(95,211,154,0.25)' : '1px solid var(--border)', borderRadius: 6, padding: '8px 10px' }}>
          <div style={{ fontWeight: 800, color: 'inherit', marginBottom: 3 }}>{sourceStatus.label}</div>
          <div>{clipStatusMessage}</div>
        </div>

        <div style={{ fontSize: 12, color: 'var(--text-muted)', background: 'var(--bg-subtle)', border: '1px solid var(--border)', borderRadius: 6, padding: '8px 10px', lineHeight: 1.5 }}>
          Platform watch URLs are saved as reference only. Upload the source file or provide a direct downloadable video URL to generate clips.
        </div>

        <Field label="Prompt / instruction">
          <textarea value={prompt} onChange={event => setPrompt(event.target.value)} rows={4} placeholder="Describe the clips you want." style={{ ...inputStyle(), resize: 'vertical', minHeight: 112, fontSize: 13 }} />
        </Field>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 10 }}>
          <Field label="Clip length">
            <select value={clipLength} onChange={event => setClipLength(event.target.value as typeof clipLength)} style={inputStyle()}>
              <option value="auto">Auto</option>
              <option value="15s">15s</option>
              <option value="30s">30s</option>
              <option value="60s">60s</option>
              <option value="3min">3min</option>
            </select>
          </Field>
          <Field label="Number of clips">
            <select value={clipCount} onChange={event => setClipCount(Number(event.target.value) as 1 | 3 | 6)} style={inputStyle()}>
              <option value={1}>1</option>
              <option value={3}>3</option>
              <option value={6}>6</option>
            </select>
          </Field>
          <Field label="Layout mode">
            <select value={layoutMode} onChange={event => setLayoutMode(event.target.value as ClipLayoutMode)} style={inputStyle()}>
              <option value="blurred_background">Blurred background</option>
              <option value="fill_crop">Fill crop</option>
              <option value="fit_with_bars">Fit with bars</option>
            </select>
          </Field>
          <Field label="CTA size">
            <select value={ctaSize} onChange={event => setCtaSize(event.target.value as ClipCTASize)} style={inputStyle()}>
              <option value="small">Small</option>
              <option value="medium">Medium</option>
              <option value="large">Large</option>
            </select>
          </Field>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10 }}>
          <Field label="Top text"><input value={topText} onChange={event => setTopText(event.target.value)} style={inputStyle()} /></Field>
          <Field label="Bottom text"><input value={bottomText} onChange={event => setBottomText(event.target.value)} style={inputStyle()} /></Field>
          <Field label="Watermark / channel name"><input value={watermark} onChange={event => setWatermark(event.target.value)} style={inputStyle()} /></Field>
        </div>

        <Field label="Caption text optional">
          <textarea value={captionText} onChange={event => setCaptionText(event.target.value)} rows={2} placeholder="Leave blank to hide captions." style={{ ...inputStyle(), resize: 'vertical', minHeight: 70, fontSize: 13 }} />
        </Field>

        <label style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: 12, color: 'var(--text-secondary)' }}>
          <input type="checkbox" checked={rightsConfirmed} onChange={event => setRightsConfirmed(event.target.checked)} />
          I confirm I have rights or permission to use this source.
        </label>

        <details style={{ borderTop: '1px solid var(--border)', paddingTop: 12 }}>
          <summary style={{ cursor: 'pointer', fontSize: 12, fontWeight: 700, color: 'var(--text-secondary)' }}>
            Rights & attribution
          </summary>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10, marginTop: 12 }}>
            <Field label="Source model">
              <select value={sourceModel} onChange={event => setSourceModel(event.target.value as ClipSourceModel)} style={inputStyle()}>
                {SOURCE_MODELS.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </Field>
            <Field label="Source title"><input value={sourceTitle} onChange={event => setSourceTitle(event.target.value)} style={inputStyle()} /></Field>
            <Field label="Source creator"><input value={sourceCreator} onChange={event => setSourceCreator(event.target.value)} style={inputStyle()} /></Field>
            <Field label="License"><input value={sourceLicense} onChange={event => setSourceLicense(event.target.value)} style={inputStyle()} /></Field>
            <Field label="Attribution text"><input value={attributionText} onChange={event => setAttributionText(event.target.value)} style={inputStyle()} /></Field>
            <Field label="Copyright overlay"><input value={copyrightOverlayText} onChange={event => setCopyrightOverlayText(event.target.value)} style={inputStyle()} /></Field>
            <Field label="Platform source"><input value={platformSource} onChange={event => setPlatformSource(event.target.value)} style={inputStyle()} /></Field>
          </div>
        </details>

        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="generate-btn idle" onClick={handleGenerate} disabled={!canGenerate} title={generateDisabledReason || undefined} type="button">
            {busy ? 'Generating clips...' : 'Generate Clips'}
          </button>
          <button className="generate-btn idle" onClick={handleDownload} disabled={downloadBusy || !packageReady} type="button" style={{ opacity: packageReady ? 1 : 0.55 }}>
            {downloadBusy ? 'Downloading...' : 'Download ZIP'}
          </button>
          <button className="generate-btn idle" onClick={() => setPublishOpen(true)} disabled={!packageReady} title={packageReady ? undefined : 'Generate clips first.'} type="button" style={{ opacity: packageReady ? 1 : 0.55 }}>
            Publish
          </button>
        </div>

        {result?.notes && <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>{result.notes}</div>}
        {(result?.included_files?.length ?? 0) > 0 && (
          <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', lineHeight: 1.6 }}>
            ZIP includes {result?.included_files.length} file(s), including clip folders, thumbnails, attribution, and manifest when generation completes.
          </div>
        )}
      </div>
      {publishOpen && (
        <PublishModal
          connections={connections}
          connectionsLoaded={connectionsLoaded}
          onDownload={handleDownload}
          downloadDisabled={downloadBusy || !packageReady}
          downloadLabel={downloadBusy ? 'Downloading...' : 'Download ZIP'}
          onClose={() => setPublishOpen(false)}
          onConnections={() => {
            setPublishOpen(false);
            onNavigate?.('connections');
          }}
        />
      )}
    </section>
  );
}

function StatusPill({ label, active, neutral = false }: { label: string; active: boolean; neutral?: boolean }) {
  return (
    <div className={`clip-status-pill${active ? ' active' : ''}${neutral ? ' neutral' : ''}`}>
      <span />
      {label}
    </div>
  );
}

function PublishModal({
  connections,
  connectionsLoaded,
  onDownload,
  downloadDisabled,
  downloadLabel,
  onClose,
  onConnections,
}: {
  connections: PlatformStatus[];
  connectionsLoaded: boolean;
  onDownload: () => void;
  downloadDisabled: boolean;
  downloadLabel: string;
  onClose: () => void;
  onConnections: () => void;
}) {
  const visibleConnections = PUBLISH_PLATFORMS.map(platform => connections.find(conn => conn.platform === platform) || {
    platform,
    name: PLATFORM_LABELS[platform],
    status: 'not_connected',
    scopes: [],
    can_publish: false,
  } as PlatformStatus);
  const connectedPlatforms = visibleConnections.filter(conn => conn.status === 'connected');
  const hasConnected = connectedPlatforms.length > 0;
  const selectablePlatforms = visibleConnections.filter(conn => conn.status === 'connected' && conn.can_publish);
  const [selected, setSelected] = useState<PublishPlatform[]>([]);
  const selectedPublishableCount = selected.filter(platform => selectablePlatforms.some(conn => conn.platform === platform)).length;
  const allSelectableSelected = selectablePlatforms.length > 0 && selectablePlatforms.every(conn => selected.includes(conn.platform as PublishPlatform));

  function togglePlatform(platform: PublishPlatform) {
    setSelected(current => current.includes(platform) ? current.filter(item => item !== platform) : [...current, platform]);
  }

  function toggleAll() {
    setSelected(allSelectableSelected ? [] : selectablePlatforms.map(conn => conn.platform as PublishPlatform));
  }

  return (
    <div className="modal-layer" role="dialog" aria-modal="true" aria-label="Publish clips">
      <button className="modal-backdrop" type="button" aria-label="Close publish modal" onClick={onClose} />
      <div className="publish-modal">
        <div className="modal-header">
          <div>
            <div className="page-eyebrow">Publish</div>
            <h2>Publish generated clips</h2>
            <p>Choose where you want to publish this package.</p>
          </div>
          <button className="modal-close" type="button" onClick={onClose}>x</button>
        </div>

        {!hasConnected && (
          <>
            {!connectionsLoaded && <div className="muted-note">Loading account status...</div>}
            <div className="neutral-callout">
              <strong>No social accounts connected yet.</strong>
              <div>Connect accounts to publish directly, or download the ZIP for manual posting.</div>
            </div>
            <div className="publish-actions">
              <button className="generate-btn idle" type="button" onClick={onConnections}>Go to Connections</button>
              <button className="generate-btn idle secondary" type="button" onClick={onDownload} disabled={downloadDisabled}>{downloadLabel}</button>
              <button className="generate-btn idle secondary" type="button" onClick={onClose}>Cancel</button>
            </div>
          </>
        )}

        {hasConnected && (
          <>
            {!connectionsLoaded && <div className="muted-note">Loading account status...</div>}
            <div className="publish-selection">
              <label className="publish-checkbox select-all">
                <input type="checkbox" checked={allSelectableSelected} onChange={toggleAll} disabled={selectablePlatforms.length === 0} />
                Select all
              </label>
              {visibleConnections.map(conn => {
                const platform = conn.platform as PublishPlatform;
                const connected = conn.status === 'connected';
                const uploadEnabled = connected && conn.can_publish;
                return (
                  <label key={conn.platform} className={`publish-checkbox platform-row${uploadEnabled ? '' : ' disabled'}`}>
                    <input
                      type="checkbox"
                      checked={selected.includes(platform)}
                      onChange={() => togglePlatform(platform)}
                      disabled={!uploadEnabled}
                    />
                    <span className="publish-platform-name">{PLATFORM_LABELS[conn.platform] || conn.name}</span>
                    {!connected && <span className="status-badge">Connect required</span>}
                    {connected && !conn.can_publish && <span className="status-badge">Upload not enabled yet</span>}
                    {uploadEnabled && <span className="status-badge connected">Connected</span>}
                  </label>
                );
              })}
            </div>
            <div className="neutral-callout">Direct publishing is not enabled yet. Download ZIP for manual posting.</div>
            <div className="publish-actions">
              <button className="generate-btn idle" type="button" disabled={selectedPublishableCount === 0 || !DIRECT_PUBLISHING_ENABLED}>Publish selected</button>
              <button className="generate-btn idle secondary" type="button" onClick={onDownload} disabled={downloadDisabled}>{downloadLabel}</button>
              <button className="generate-btn idle secondary" type="button" onClick={onClose}>Cancel</button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
