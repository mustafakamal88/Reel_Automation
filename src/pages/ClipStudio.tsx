import { useMemo, useState } from 'react';
import {
  ApiError,
  downloadClipStudioZip,
  renderClipStudio,
  type ClipSourceModel,
  type ClipStudioRenderResponse,
} from '../lib/api/client';

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
    if (err.isBackendOffline) return 'Backend offline - run: cd backend && go run ./cmd/api';
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 5, fontSize: 11, color: 'var(--text-muted)' }}>
      <span>{label}</span>
      {children}
    </label>
  );
}

function inputStyle(): React.CSSProperties {
  return {
    width: '100%',
    border: '1px solid var(--border-strong)',
    background: 'var(--bg-subtle)',
    color: 'var(--text-primary)',
    borderRadius: 6,
    padding: '9px 10px',
    fontSize: 12,
  };
}

function Pill({ children }: { children: React.ReactNode }) {
  return (
    <span style={{
      fontSize: 10,
      fontFamily: 'var(--font-mono)',
      color: 'var(--text-dim)',
      background: 'var(--bg-subtle)',
      border: '1px solid var(--border-strong)',
      borderRadius: 3,
      padding: '2px 6px',
      whiteSpace: 'nowrap',
    }}>
      {children}
    </span>
  );
}

export function ClipStudioPage() {
  const [sourceModel, setSourceModel] = useState<ClipSourceModel>('user_upload');
  const [sourceVideoPath, setSourceVideoPath] = useState('');
  const [sourceUrl, setSourceUrl] = useState('');
  const [sourceTitle, setSourceTitle] = useState('');
  const [sourceCreator, setSourceCreator] = useState('');
  const [sourceLicense, setSourceLicense] = useState('');
  const [attributionText, setAttributionText] = useState('');
  const [copyrightOverlayText, setCopyrightOverlayText] = useState('');
  const [platformSource, setPlatformSource] = useState('');
  const [userConfirmedRights, setUserConfirmedRights] = useState(false);

  const [topBannerText, setTopBannerText] = useState('TREND CLIP');
  const [bottomBannerText, setBottomBannerText] = useState('FOLLOW FOR THE FULL STORY');
  const [watermarkText, setWatermarkText] = useState('@trendcortex');
  const [ctaText, setCtaText] = useState('Save this clip');
  const [fontStylePreset, setFontStylePreset] = useState('bold_editorial');
  const [topBannerColor, setTopBannerColor] = useState('#111827');
  const [bottomBannerColor, setBottomBannerColor] = useState('#0f766e');
  const [captions, setCaptions] = useState('');
  const [includeCaptions, setIncludeCaptions] = useState(true);
  const [startSeconds, setStartSeconds] = useState(0);
  const [endSeconds, setEndSeconds] = useState(15);

  const [busy, setBusy] = useState(false);
  const [downloadBusy, setDownloadBusy] = useState(false);
  const [result, setResult] = useState<ClipStudioRenderResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const canRender = useMemo(() => (
    sourceVideoPath.trim() !== '' &&
    endSeconds > startSeconds &&
    (sourceModel === 'public_domain' || userConfirmedRights)
  ), [endSeconds, sourceModel, sourceVideoPath, startSeconds, userConfirmedRights]);

  async function handleRender() {
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      const res = await renderClipStudio({
        source_model: sourceModel,
        source_video_path: sourceVideoPath,
        rights: {
          source_url: sourceUrl,
          source_title: sourceTitle,
          source_creator: sourceCreator,
          source_license: sourceLicense,
          attribution_text: attributionText,
          user_confirmed_rights: userConfirmedRights,
          copyright_overlay_text: copyrightOverlayText,
          platform_source: platformSource,
        },
        branding: {
          top_banner_text: topBannerText,
          bottom_banner_text: bottomBannerText,
          watermark_text: watermarkText,
          cta_text: ctaText,
          font_style_preset: fontStylePreset,
          top_banner_color: topBannerColor,
          bottom_banner_color: bottomBannerColor,
        },
        manual_range: {
          start_seconds: startSeconds,
          end_seconds: endSeconds,
        },
        captions,
        include_captions: includeCaptions,
        ai_highlights: {
          transcription_status: 'not_run',
          suggested_clips_status: 'not_run',
          hook_score_status: 'not_run',
        },
      });
      setResult(res);
      if (!res.success) setError(res.notes || 'Clip render did not complete');
    } catch (err) {
      setError(errMsg(err, 'Clip render failed'));
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
      setError(errMsg(err, 'Clip Studio ZIP download failed'));
    } finally {
      setDownloadBusy(false);
    }
  }

  return (
    <section className="page-section">
      <div className="empty-state" style={{ marginBottom: 16 }}>
        <div className="empty-icon">CS</div>
        <div className="empty-title">Clip Studio</div>
        <div className="empty-desc">Manual clip repurposing for owned, licensed, Creative Commons, public-domain, or rights-confirmed source media.</div>
      </div>

      {error && (
        <div style={{
          fontSize: 12,
          color: 'var(--red)',
          lineHeight: 1.6,
          marginBottom: 16,
          background: 'rgba(232,115,107,0.08)',
          border: '1px solid rgba(232,115,107,0.25)',
          borderRadius: 6,
          padding: '8px 10px',
        }}>
          {error}
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 16, alignItems: 'start' }}>
        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <div style={{ fontWeight: 700, fontSize: 14, color: 'var(--text-primary)' }}>Source & Rights</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>External URLs are metadata only until rights are confirmed and a local source file is provided.</div>
          </div>
          <Field label="Source model">
            <select value={sourceModel} onChange={event => setSourceModel(event.target.value as ClipSourceModel)} style={inputStyle()}>
              {SOURCE_MODELS.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
            </select>
          </Field>
          <Field label="Source video path">
            <input value={sourceVideoPath} onChange={event => setSourceVideoPath(event.target.value)} placeholder="/absolute/path/to/source.mp4" style={inputStyle()} />
          </Field>
          <Field label="Source URL">
            <input value={sourceUrl} onChange={event => setSourceUrl(event.target.value)} placeholder="https://..." style={inputStyle()} />
          </Field>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <Field label="Source title">
              <input value={sourceTitle} onChange={event => setSourceTitle(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="Source creator">
              <input value={sourceCreator} onChange={event => setSourceCreator(event.target.value)} style={inputStyle()} />
            </Field>
          </div>
          <Field label="Source license">
            <input value={sourceLicense} onChange={event => setSourceLicense(event.target.value)} placeholder="CC BY 4.0, public domain, paid license ID..." style={inputStyle()} />
          </Field>
          <Field label="Attribution text">
            <textarea value={attributionText} onChange={event => setAttributionText(event.target.value)} rows={3} style={inputStyle()} />
          </Field>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <Field label="Copyright overlay">
              <input value={copyrightOverlayText} onChange={event => setCopyrightOverlayText(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="Platform source">
              <input value={platformSource} onChange={event => setPlatformSource(event.target.value)} placeholder="YouTube, TikTok, local archive..." style={inputStyle()} />
            </Field>
          </div>
          <label style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: 12, color: 'var(--text-secondary)' }}>
            <input type="checkbox" checked={userConfirmedRights} onChange={event => setUserConfirmedRights(event.target.checked)} />
            I confirm I have rights to render and export this source.
          </label>
        </div>

        <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <div style={{ fontWeight: 700, fontSize: 14, color: 'var(--text-primary)' }}>Layout & Manual Clip</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>Exports vertical 1080x1920 H.264/AAC MP4 with safe banner zones.</div>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <Field label="Start seconds">
              <input type="number" min={0} step={0.1} value={startSeconds} onChange={event => setStartSeconds(Number(event.target.value))} style={inputStyle()} />
            </Field>
            <Field label="End seconds">
              <input type="number" min={0} step={0.1} value={endSeconds} onChange={event => setEndSeconds(Number(event.target.value))} style={inputStyle()} />
            </Field>
          </div>
          <Field label="Top banner text">
            <input value={topBannerText} onChange={event => setTopBannerText(event.target.value)} style={inputStyle()} />
          </Field>
          <Field label="Bottom banner text">
            <input value={bottomBannerText} onChange={event => setBottomBannerText(event.target.value)} style={inputStyle()} />
          </Field>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <Field label="Top banner color">
              <input type="color" value={topBannerColor} onChange={event => setTopBannerColor(event.target.value)} style={{ ...inputStyle(), height: 40, padding: 4 }} />
            </Field>
            <Field label="Bottom banner color">
              <input type="color" value={bottomBannerColor} onChange={event => setBottomBannerColor(event.target.value)} style={{ ...inputStyle(), height: 40, padding: 4 }} />
            </Field>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <Field label="Watermark text">
              <input value={watermarkText} onChange={event => setWatermarkText(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="CTA text">
              <input value={ctaText} onChange={event => setCtaText(event.target.value)} style={inputStyle()} />
            </Field>
          </div>
          <Field label="Font/style preset">
            <select value={fontStylePreset} onChange={event => setFontStylePreset(event.target.value)} style={inputStyle()}>
              <option value="bold_editorial">Bold editorial</option>
              <option value="clean_news">Clean news</option>
              <option value="creator_caption">Creator caption</option>
            </select>
          </Field>
          <Field label="Logo upload">
            <input disabled placeholder="Placeholder - upload pipeline not wired yet" style={{ ...inputStyle(), opacity: 0.6 }} />
          </Field>
          <Field label="Captions">
            <textarea value={captions} onChange={event => setCaptions(event.target.value)} rows={3} style={inputStyle()} />
          </Field>
          <label style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: 12, color: 'var(--text-secondary)' }}>
            <input type="checkbox" checked={includeCaptions} onChange={event => setIncludeCaptions(event.target.checked)} />
            Include captions overlay
          </label>
        </div>
      </div>

      <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
          <div>
            <div style={{ fontWeight: 700, fontSize: 14, color: 'var(--text-primary)' }}>AI Highlight Model</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>Placeholders only; no transcription, suggested clips, or hook scores are generated.</div>
          </div>
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <Pill>transcription: not_run</Pill>
            <Pill>suggested_clips: not_run</Pill>
            <Pill>hook_score: not_run</Pill>
          </div>
        </div>
        {result && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <Pill>render: {result.render_status}</Pill>
            {result.zip_filename && <Pill>{result.zip_filename}</Pill>}
            {result.included_files.map(name => <Pill key={name}>{name}</Pill>)}
          </div>
        )}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="generate-btn idle" onClick={handleRender} disabled={busy || !canRender} type="button">
            <span className="generate-btn-dot" style={{ background: '#15121f' }} />
            {busy ? 'Rendering clip...' : 'Render manual clip'}
          </button>
          {result?.download_url && result.zip_filename && (
            <button className="generate-btn idle" onClick={handleDownload} disabled={downloadBusy} type="button">
              <span className="generate-btn-dot" style={{ background: '#15121f' }} />
              {downloadBusy ? 'Downloading...' : 'Download ZIP'}
            </button>
          )}
        </div>
      </div>
    </section>
  );
}
