import { useEffect, useMemo, useState } from 'react';
import {
  ApiError,
  createClipStudioSource,
  downloadClipStudioZip,
  generateAIScenes,
  generateClipStudio,
  getAISceneWorkerStatus,
  planAIScenes,
  type AISceneGenerateResponse,
  type AIScenePlanResponse,
  type AISceneWorkerStatusResponse,
  uploadClipStudioSource,
  type ClipSourceModel,
  type ClipStudioGenerateResponse,
  type ClipStudioSourceResponse,
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

function StatusPill({ children }: { children: React.ReactNode }) {
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
  const [sourceUrl, setSourceUrl] = useState('');
  const [source, setSource] = useState<ClipStudioSourceResponse | null>(null);
  const [prompt, setPrompt] = useState('make funny 3-minute clips with top and bottom branding');
  const [clipLength, setClipLength] = useState<'auto' | '15s' | '30s' | '60s' | '3min'>('auto');
  const [clipCount, setClipCount] = useState<1 | 3 | 6>(3);
  const [topText, setTopText] = useState('TREND CLIP');
  const [bottomText, setBottomText] = useState('FOLLOW FOR THE FULL STORY');
  const [watermark, setWatermark] = useState('@trendcortex');
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
  const [downloadBusy, setDownloadBusy] = useState(false);
  const [result, setResult] = useState<ClipStudioGenerateResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [aiTopic, setAiTopic] = useState('');
  const [aiPrompt, setAiPrompt] = useState('Realistic vertical scenes for a fast-paced trend explainer');
  const [aiStyle, setAiStyle] = useState('realistic_editorial');
  const [aiLength, setAiLength] = useState<30 | 60 | 180>(60);
  const [aiBranding, setAiBranding] = useState('TREND CORTEX');
  const [workerStatus, setWorkerStatus] = useState<AISceneWorkerStatusResponse | null>(null);
  const [aiPlan, setAiPlan] = useState<AIScenePlanResponse | null>(null);
  const [aiResult, setAiResult] = useState<AISceneGenerateResponse | null>(null);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiDownloadBusy, setAiDownloadBusy] = useState(false);

  const hasSource = useMemo(() => Boolean(source?.source_id || sourceUrl.trim()), [source, sourceUrl]);
  const canGenerate = hasSource && prompt.trim() !== '' && rightsConfirmed && !busy && !uploadBusy;

  useEffect(() => {
    void refreshWorkerStatus();
  }, []);

  async function refreshWorkerStatus() {
    try {
      setWorkerStatus(await getAISceneWorkerStatus());
    } catch (err) {
      setWorkerStatus({ configured: false, status: 'error', message: errMsg(err, 'Worker status unavailable') });
    }
  }

  async function handlePlanAIScenes() {
    setAiBusy(true);
    setError(null);
    setAiResult(null);
    try {
      const planned = await planAIScenes({
        topic: aiTopic,
        prompt: aiPrompt,
        style_preset: aiStyle,
        target_length_seconds: aiLength,
      });
      setAiPlan(planned);
      await refreshWorkerStatus();
    } catch (err) {
      setError(errMsg(err, 'Scene planning failed'));
    } finally {
      setAiBusy(false);
    }
  }

  async function handleGenerateAIScenes() {
    setAiBusy(true);
    setError(null);
    try {
      const generated = await generateAIScenes({
        topic: aiTopic,
        prompt: aiPrompt,
        style_preset: aiStyle,
        target_length_seconds: aiLength,
        branding: {
          top_banner_text: aiBranding,
          bottom_banner_text: 'FOLLOW FOR MORE',
          watermark_text: watermark,
          cta_text: 'FOLLOW FOR MORE',
          font_style_preset: 'bold_editorial',
          top_banner_color: '#101828',
          bottom_banner_color: '#0f766e',
        },
      });
      setAiResult(generated);
      if (!generated.success) setError(generated.notes || 'Connect local AI worker to generate AI scenes.');
      await refreshWorkerStatus();
    } catch (err) {
      setError(errMsg(err, 'AI scene generation failed'));
    } finally {
      setAiBusy(false);
    }
  }

  async function handleDownloadAIScenes() {
    if (!aiResult?.download_url || !aiResult.zip_filename) return;
    setAiDownloadBusy(true);
    setError(null);
    try {
      await downloadClipStudioZip(aiResult.download_url, aiResult.zip_filename);
    } catch (err) {
      setError(errMsg(err, 'AI scene ZIP download failed'));
    } finally {
      setAiDownloadBusy(false);
    }
  }

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

  async function handleGenerate() {
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      let activeSource = source;
      if (!activeSource && sourceUrl.trim()) {
        activeSource = await createClipStudioSource({
          source_url: sourceUrl.trim(),
          rights_confirmed: rightsConfirmed,
          rights: {
            source_url: sourceUrl.trim(),
            source_title: sourceTitle,
            source_creator: sourceCreator,
            source_license: sourceLicense,
            attribution_text: attributionText,
            user_confirmed_rights: rightsConfirmed,
            copyright_overlay_text: copyrightOverlayText,
            platform_source: platformSource,
          },
        });
        setSource(activeSource);
      }

      const generated = await generateClipStudio({
        source_id: activeSource?.source_id,
        source_url: activeSource ? undefined : sourceUrl.trim(),
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
        },
        rights: {
          source_url: sourceUrl.trim() || activeSource?.metadata.url,
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
      if (!generated.success) setError(generated.notes || activeSource?.message || 'Clip generation did not complete');
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
      <div className="empty-state" style={{ marginBottom: 16 }}>
        <div className="empty-icon">CG</div>
        <div className="empty-title">Clip Generator</div>
        <div className="empty-desc">Paste a video URL or upload a source, describe the clips, then generate a downloadable package.</div>
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

      <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 920, marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <div>
            <div style={{ fontSize: 14, fontWeight: 800, color: 'var(--text-primary)' }}>AI Scene Generator</div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 3 }}>
              {workerStatus?.message || 'Connect local AI worker to generate AI scenes.'}
            </div>
          </div>
          <button className="generate-btn idle" onClick={refreshWorkerStatus} type="button">
            <span className="generate-btn-dot" style={{ background: workerStatus?.configured ? '#0f766e' : '#991b1b' }} />
            Worker Status
          </button>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10 }}>
          <Field label="Topic">
            <input value={aiTopic} onChange={event => setAiTopic(event.target.value)} placeholder="AI search trend, finance news, creator drama..." style={inputStyle()} />
          </Field>
          <Field label="Style preset">
            <select value={aiStyle} onChange={event => setAiStyle(event.target.value)} style={inputStyle()}>
              <option value="realistic_editorial">Realistic editorial</option>
              <option value="cinematic_documentary">Cinematic documentary</option>
              <option value="ugc_phone_camera">UGC phone camera</option>
            </select>
          </Field>
          <Field label="Target length">
            <select value={aiLength} onChange={event => setAiLength(Number(event.target.value) as 30 | 60 | 180)} style={inputStyle()}>
              <option value={30}>30s</option>
              <option value={60}>60s</option>
              <option value={180}>3min</option>
            </select>
          </Field>
          <Field label="Branding text">
            <input value={aiBranding} onChange={event => setAiBranding(event.target.value)} style={inputStyle()} />
          </Field>
        </div>

        <Field label="Prompt">
          <textarea
            value={aiPrompt}
            onChange={event => setAiPrompt(event.target.value)}
            rows={4}
            style={{ ...inputStyle(), resize: 'vertical', minHeight: 104, fontSize: 13 }}
          />
        </Field>

        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="generate-btn idle" onClick={handlePlanAIScenes} disabled={aiBusy || !aiPrompt.trim()} type="button">
            <span className="generate-btn-dot" style={{ background: '#15121f' }} />
            Generate scene plan
          </button>
          <button className="generate-btn idle" onClick={handleGenerateAIScenes} disabled={aiBusy || !aiPrompt.trim()} type="button">
            <span className="generate-btn-dot" style={{ background: workerStatus?.configured ? '#0f766e' : '#991b1b' }} />
            {aiBusy ? 'Generating...' : 'Generate video using local worker'}
          </button>
          <button className="generate-btn idle" onClick={handleDownloadAIScenes} disabled={aiDownloadBusy || !aiResult?.download_url || !aiResult.zip_filename} type="button">
            <span className="generate-btn-dot" style={{ background: '#15121f' }} />
            {aiDownloadBusy ? 'Downloading...' : 'Download AI ZIP'}
          </button>
        </div>

        {(aiPlan || aiResult) && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <StatusPill>{aiResult?.renderer_version || aiPlan?.renderer_version}</StatusPill>
            <StatusPill>worker: {String(aiResult?.worker_url_configured ?? workerStatus?.configured ?? false)}</StatusPill>
            <StatusPill>status: {aiResult?.generation_status || workerStatus?.status || 'planned'}</StatusPill>
            <StatusPill>model: {aiResult?.model_hint || aiPlan?.model_hint || 'auto'}</StatusPill>
            {aiResult?.zip_filename && <StatusPill>{aiResult.zip_filename}</StatusPill>}
            {aiResult?.fallback_reason && <StatusPill>{aiResult.fallback_reason}</StatusPill>}
          </div>
        )}

        {aiPlan?.scenes && (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 8 }}>
            {aiPlan.scenes.map(scene => (
              <div key={scene.scene_id} style={{ border: '1px solid var(--border)', borderRadius: 6, padding: 10, background: 'var(--bg-subtle)' }}>
                <div style={{ fontSize: 11, fontFamily: 'var(--font-mono)', color: 'var(--text-dim)' }}>{scene.scene_id} · {scene.duration_seconds.toFixed(1)}s · {scene.model_hint}</div>
                <div style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.5, marginTop: 6 }}>{scene.visual_prompt}</div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 920 }}>
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 10, alignItems: 'end' }}>
          <Field label="Paste video URL">
            <input
              value={sourceUrl}
              onChange={event => {
                setSourceUrl(event.target.value);
                setSource(null);
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
              style={{ ...inputStyle(), width: 230 }}
            />
          </Field>
        </div>

        {(source || uploadBusy) && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {uploadBusy && <StatusPill>uploading</StatusPill>}
            {source?.source_id && <StatusPill>{source.source_id}</StatusPill>}
            {source?.status && <StatusPill>{source.status}</StatusPill>}
            {source?.message && <StatusPill>{source.message}</StatusPill>}
          </div>
        )}

        <Field label="Prompt / instruction">
          <textarea
            value={prompt}
            onChange={event => setPrompt(event.target.value)}
            rows={4}
            placeholder="make funny 3-minute clips with top and bottom branding"
            style={{ ...inputStyle(), resize: 'vertical', minHeight: 112, fontSize: 13 }}
          />
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
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10 }}>
          <Field label="Top text">
            <input value={topText} onChange={event => setTopText(event.target.value)} style={inputStyle()} />
          </Field>
          <Field label="Bottom text">
            <input value={bottomText} onChange={event => setBottomText(event.target.value)} style={inputStyle()} />
          </Field>
          <Field label="Watermark / channel name">
            <input value={watermark} onChange={event => setWatermark(event.target.value)} style={inputStyle()} />
          </Field>
        </div>

        <details style={{ borderTop: '1px solid var(--border)', paddingTop: 12 }}>
          <summary style={{ cursor: 'pointer', fontSize: 12, fontWeight: 700, color: 'var(--text-secondary)' }}>
            Advanced / Rights & Attribution
          </summary>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10, marginTop: 12 }}>
            <Field label="Source model">
              <select value={sourceModel} onChange={event => setSourceModel(event.target.value as ClipSourceModel)} style={inputStyle()}>
                {SOURCE_MODELS.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </Field>
            <Field label="Source title">
              <input value={sourceTitle} onChange={event => setSourceTitle(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="Source creator">
              <input value={sourceCreator} onChange={event => setSourceCreator(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="License">
              <input value={sourceLicense} onChange={event => setSourceLicense(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="Attribution text">
              <input value={attributionText} onChange={event => setAttributionText(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="Copyright overlay">
              <input value={copyrightOverlayText} onChange={event => setCopyrightOverlayText(event.target.value)} style={inputStyle()} />
            </Field>
            <Field label="Platform source">
              <input value={platformSource} onChange={event => setPlatformSource(event.target.value)} style={inputStyle()} />
            </Field>
          </div>
        </details>

        <label style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: 12, color: 'var(--text-secondary)' }}>
          <input type="checkbox" checked={rightsConfirmed} onChange={event => setRightsConfirmed(event.target.checked)} />
          I confirm I have rights or permission to use this source.
        </label>

        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="generate-btn idle" onClick={handleGenerate} disabled={!canGenerate} type="button">
            <span className="generate-btn-dot" style={{ background: '#15121f' }} />
            {busy ? 'Generating clips...' : 'Generate Clips'}
          </button>
          <button className="generate-btn idle" onClick={handleDownload} disabled={downloadBusy || !result?.download_url || !result.zip_filename} type="button">
            <span className="generate-btn-dot" style={{ background: '#15121f' }} />
            {downloadBusy ? 'Downloading...' : 'Download ZIP'}
          </button>
        </div>

        {result && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <StatusPill>render: {result.render_status}</StatusPill>
            <StatusPill>highlight_detection: {result.highlight_detection}</StatusPill>
            {result.zip_filename && <StatusPill>{result.zip_filename}</StatusPill>}
            {result.generated_clip_jobs?.map(job => <StatusPill key={job.clip_id}>{job.clip_id}: {job.render_status}</StatusPill>)}
          </div>
        )}
      </div>
    </section>
  );
}
