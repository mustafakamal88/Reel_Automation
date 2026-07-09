import { useMemo, useState } from 'react';
import {
  ApiError,
  downloadClipStudioZip,
  generateClipStudio,
  importClipStudioURL,
  uploadClipStudioSource,
  type ClipCTASize,
  type ClipLayoutMode,
  type ClipSourceModel,
  type ClipStudioGenerateResponse,
  type ClipStudioSourceResponse,
} from '../lib/api/client';
import {
  clipGenerateDisabledReason,
  clipPackageReady,
  getClipSourceStatus,
  sourceCanGenerate,
} from './ClipStudioState';

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

export function ClipStudioPage() {
  const [sourceUrl, setSourceUrl] = useState('');
  const [source, setSource] = useState<ClipStudioSourceResponse | null>(null);
  const [prompt, setPrompt] = useState('Make short branded clips with a strong hook and clear takeaway.');
  const [clipLength, setClipLength] = useState<'auto' | '15s' | '30s' | '60s' | '3min'>('auto');
  const [clipCount, setClipCount] = useState<1 | 3 | 6>(3);
  const [topText, setTopText] = useState('TREND CLIP');
  const [bottomText, setBottomText] = useState('FOLLOW FOR MORE');
  const [watermark, setWatermark] = useState('@trendcortex');
  const [layoutMode, setLayoutMode] = useState<ClipLayoutMode>('blurred_background');
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
  const [result, setResult] = useState<ClipStudioGenerateResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const packageReady = clipPackageReady(result);
  const sourceStatus = useMemo(() => getClipSourceStatus(source, sourceUrl), [source, sourceUrl]);
  const generateDisabledReason = clipGenerateDisabledReason({ source, rightsConfirmed, prompt, busy, uploadBusy, urlImportBusy });
  const canGenerate = generateDisabledReason === null;
  const clipStatusMessage = useMemo(() => {
    if (packageReady) return 'Package ready. Download the ZIP when you are ready.';
    return generateDisabledReason || sourceStatus.message;
  }, [generateDisabledReason, packageReady, sourceStatus.message]);

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
    } catch (err) {
      setError(errMsg(err, 'Clip ZIP download failed'));
    } finally {
      setDownloadBusy(false);
    }
  }

  return (
    <section className="page-section">
      <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 980 }}>
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
        </div>

        {result?.notes && <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.6 }}>{result.notes}</div>}
        {(result?.included_files?.length ?? 0) > 0 && (
          <div style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', lineHeight: 1.6 }}>
            ZIP includes {result?.included_files.length} file(s), including clip folders, thumbnails, attribution, and manifest when generation completes.
          </div>
        )}
      </div>
    </section>
  );
}
