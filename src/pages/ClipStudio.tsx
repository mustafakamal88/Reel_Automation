import { cloneElement, isValidElement, useEffect, useId, useMemo, useState, type ReactElement, type ReactNode } from 'react';
import {
  ApiError,
  archiveContentProjectOutput,
  downloadClipStudioZip,
  downloadContentProjectOutput,
  generateClipStudio,
  getContentProject,
  getPlatformConnections,
  importClipStudioURL,
  listContentProjectOutputs,
  retryContentProjectOutput,
  uploadClipStudioSource,
  type ClipCTASize,
  type ClipLayoutMode,
  type ClipSourceModel,
  type ClipStudioGenerateResponse,
  type ClipStudioSourceResponse,
  type ContentProject,
  type ContentProjectOutput,
  type PlatformStatus,
} from '../lib/api/client';
import { ConfirmationDialog } from '../components/ConfirmationDialog';
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
    if (err.isBackendOffline) return 'Clip generation is temporarily unavailable.';
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  const id = useId();
  const control = isValidElement(children)
    ? cloneElement(children as ReactElement<{ id?: string }>, { id: (children as ReactElement<{ id?: string }>).props.id ?? id })
    : children;
  return (
    <div className="form-group clip-field">
      <label className="form-label" htmlFor={id}>{label}</label>
      {control}
    </div>
  );
}

export function ClipStudioPage({ onNavigate }: Props) {
  const savedSettings = storage.getSettings();
  const handoff = storage.getClipGeneratorHandoff();
  const queryProjectID = typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('project_id')?.trim() || '' : '';
  const activeProjectID = queryProjectID || handoff?.projectId || '';
  const [sourceUrl, setSourceUrl] = useState('');
  const [source, setSource] = useState<ClipStudioSourceResponse | null>(null);
  const [prompt, setPrompt] = useState(handoff?.script ? `${handoff.title}\n\n${handoff.hook || ''}\n\n${handoff.script}`.trim() : 'Make short branded clips with a strong hook and clear takeaway.');
  const [clipLength, setClipLength] = useState<'auto' | '15s' | '30s' | '60s' | '3min'>('auto');
  const [clipCount, setClipCount] = useState<1 | 3 | 6>(3);
  const [topText, setTopText] = useState(savedSettings.defaultTopText);
  const [bottomText, setBottomText] = useState(savedSettings.defaultBottomText);
  const [watermark, setWatermark] = useState(savedSettings.defaultWatermark);
  const [layoutMode, setLayoutMode] = useState<ClipLayoutMode>(savedSettings.defaultLayoutMode);
  const [captionText, setCaptionText] = useState(handoff?.caption || '');
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
  const [project, setProject] = useState<ContentProject | null>(null);
  const [projectLoading, setProjectLoading] = useState(false);
  const [projectError, setProjectError] = useState<string | null>(null);
  const [outputs, setOutputs] = useState<ContentProjectOutput[]>([]);
  const [outputsLoading, setOutputsLoading] = useState(false);
  const [outputsError, setOutputsError] = useState<string | null>(null);
  const [outputActionID, setOutputActionID] = useState<string | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<ContentProjectOutput | null>(null);

  const packageReady = clipPackageReady(result);
  const projectOutputCount = outputs.filter(output => output.status !== 'archived').length;
  const handoffSceneTotal = handoff?.totalPlannedDurationSeconds || handoff?.scenes?.reduce((sum, scene) => sum + scene.planned_duration_seconds, 0) || 0;
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

  async function refreshOutputs(projectID = activeProjectID) {
    if (!projectID) return;
    setOutputsLoading(true);
    setOutputsError(null);
    try {
      const response = await listContentProjectOutputs(projectID);
      setOutputs(response.outputs);
    } catch (err) {
      setOutputsError(errMsg(err, 'Project outputs could not be loaded.'));
    } finally {
      setOutputsLoading(false);
    }
  }

  useEffect(() => {
    if (!activeProjectID) {
      setProject(null);
      setOutputs([]);
      return;
    }
    let cancelled = false;
    setProjectLoading(true);
    setProjectError(null);
    getContentProject(activeProjectID)
      .then(response => {
        if (!cancelled) setProject(response);
      })
      .catch(err => {
        if (!cancelled) setProjectError(errMsg(err, 'Project context could not be loaded.'));
      })
      .finally(() => {
        if (!cancelled) setProjectLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activeProjectID]);

  useEffect(() => {
    if (!activeProjectID) return;
    let cancelled = false;
    setOutputsLoading(true);
    setOutputsError(null);
    listContentProjectOutputs(activeProjectID)
      .then(response => {
        if (!cancelled) setOutputs(response.outputs);
      })
      .catch(err => {
        if (!cancelled) setOutputsError(errMsg(err, 'Project outputs could not be loaded.'));
      })
      .finally(() => {
        if (!cancelled) setOutputsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activeProjectID]);

  useEffect(() => {
    if (!activeProjectID || !outputs.some(output => output.status === 'queued' || output.status === 'processing')) return;
    let cancelled = false;
    const timer = window.setInterval(() => {
      listContentProjectOutputs(activeProjectID)
        .then(response => {
          if (!cancelled) setOutputs(response.outputs);
        })
        .catch(err => {
          if (!cancelled) setOutputsError(errMsg(err, 'Project outputs could not be refreshed.'));
        });
    }, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeProjectID, outputs]);

  async function handleUpload(file: File | undefined) {
    if (!file) return;
    setUploadBusy(true);
    setError(null);
    setResult(null);
    try {
      const uploaded = await uploadClipStudioSource(file, activeProjectID || undefined);
      setSource(uploaded);
      setSourceUrl('');
      setSourceModel('user_upload');
      if (uploaded.project_output) await refreshOutputs();
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
        project_id: activeProjectID || undefined,
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
      if (imported.project_output) await refreshOutputs();
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
        project_id: activeProjectID || undefined,
        output_scope: 'project',
        idempotency_key: activeProjectID ? `project-${activeProjectID}-${Date.now()}` : undefined,
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
      if (generated.project_output) await refreshOutputs();
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

  async function handleOutputDownload(output: ContentProjectOutput) {
    if (!activeProjectID || !output.download_url) return;
    setOutputActionID(output.id);
    setError(null);
    try {
      await downloadContentProjectOutput(activeProjectID, output.id, output.display_name);
    } catch (err) {
      setError(errMsg(err, 'Project output download failed'));
      await refreshOutputs();
    } finally {
      setOutputActionID(null);
    }
  }

  async function handleRetryOutput(output: ContentProjectOutput) {
    if (!activeProjectID || !output.retryable) return;
    setOutputActionID(output.id);
    setError(null);
    try {
      const retry = await retryContentProjectOutput(activeProjectID, output.id);
      setResult(retry);
      await refreshOutputs();
    } catch (err) {
      setError(errMsg(err, 'Project output retry failed'));
      await refreshOutputs();
    } finally {
      setOutputActionID(null);
    }
  }

  async function handleArchiveOutput(output = archiveTarget) {
    if (!activeProjectID) return;
    if (!output) return;
    setOutputActionID(output.id);
    setError(null);
    try {
      await archiveContentProjectOutput(activeProjectID, output.id);
      setArchiveTarget(null);
      await refreshOutputs();
    } catch (err) {
      setError(errMsg(err, 'Project output archive failed'));
    } finally {
      setOutputActionID(null);
    }
  }

  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Content</div>
          <h1>Clip Generator</h1>
          <p>Import a real source, confirm rights, set clip direction, and download generated packages when the backend returns output.</p>
        </div>
      </div>

      {activeProjectID && (
        <ProjectContextCard
          project={project}
          loading={projectLoading}
          error={projectError}
          sceneCount={handoff?.scenes?.length || 0}
          plannedDuration={handoffSceneTotal}
          outputCount={projectOutputCount}
        />
      )}

      <div className="clip-workspace">
        <div className="clip-workspace-main settings-card">
          <div className="clip-section-header">
            <div>
              <div className="settings-card-title">Source import</div>
              <p>Upload a source video or provide a direct downloadable video URL. Generation stays disabled until a valid source and rights confirmation are present.</p>
            </div>
          </div>

          {error && (
            <div className="clip-error">
              {error}
            </div>
          )}

          <div className="clip-source-grid">
            <Field label="Direct video URL or reference URL">
              <input
                value={sourceUrl}
                onChange={event => {
                  setSourceUrl(event.target.value);
                  setSource(null);
                  setResult(null);
                }}
                placeholder="https://example.com/source.mp4"
                className="form-input"
              />
            </Field>
            <Field label="Upload video">
              <input
                type="file"
                accept="video/mp4,video/quicktime,video/webm,.mp4,.mov,.webm"
                onChange={event => void handleUpload(event.target.files?.[0])}
                className="form-input"
              />
            </Field>
          </div>

          <div className="clip-action-row compact">
            <button className="generate-btn idle" onClick={handleImportURL} disabled={!sourceUrl.trim() || urlImportBusy} type="button">
              {urlImportBusy ? 'Checking URL...' : 'Import URL'}
            </button>
          </div>

          <div className="clip-status-grid">
            <StatusPill label="Source ready" active={Boolean(source && sourceCanGenerate(source))} />
            <StatusPill label="Clips generated" active={Boolean(result?.success)} />
            <StatusPill label="Package ready" active={packageReady} />
            <StatusPill label={connectedAccountPlatforms.length > 0 ? 'Social account connected' : 'Social accounts not connected'} active={connectedAccountPlatforms.length > 0} neutral />
          </div>

          <div className={`clip-readiness-note ${sourceStatus.tone}`}>
            <div>{sourceStatus.label}</div>
            <p>{clipStatusMessage}</p>
          </div>
        </div>

        <div className="clip-workspace-main settings-card">
          <div className="clip-section-header">
            <div>
              <div className="settings-card-title">Clip direction</div>
              <p>Set the creative prompt, clip count, layout, captions, and brand overlays that will be sent to the existing generator.</p>
            </div>
          </div>

          {activeProjectID && (
            <div className="clip-readiness-note ready">
              <div>Project handoff active</div>
              <p>This clip package is linked to {project?.title || handoff?.title || 'the active project'}.</p>
            </div>
          )}

          {handoff?.projectId && (handoff.scenes?.length ?? 0) > 0 && (
            <div className="clip-scene-summary">
              <div className="clip-scene-summary-header">
                <strong>Scene plan received</strong>
                <span>{handoff.scenes?.length} scene(s) · {handoffSceneTotal}s planned{handoff.targetDurationSeconds ? ` · ${handoff.targetDurationSeconds}s target` : ''}</span>
              </div>
              <ol>
                {handoff.scenes?.slice(0, 6).map((scene, index) => (
                  <li key={scene.id}>
                    <span>{index + 1}. {scene.title || 'Untitled scene'}</span>
                    <small>{scene.planned_duration_seconds}s</small>
                  </li>
                ))}
              </ol>
            </div>
          )}

        <Field label="Prompt / instruction">
          <textarea className="form-textarea clip-prompt-textarea" value={prompt} onChange={event => setPrompt(event.target.value)} rows={4} placeholder="Describe the clips you want." />
        </Field>

        <div className="form-grid four">
          <Field label="Clip length">
            <select className="form-input" value={clipLength} onChange={event => setClipLength(event.target.value as typeof clipLength)}>
              <option value="auto">Auto</option>
              <option value="15s">15s</option>
              <option value="30s">30s</option>
              <option value="60s">60s</option>
              <option value="3min">3min</option>
            </select>
          </Field>
          <Field label="Number of clips">
            <select className="form-input" value={clipCount} onChange={event => setClipCount(Number(event.target.value) as 1 | 3 | 6)}>
              <option value={1}>1</option>
              <option value={3}>3</option>
              <option value={6}>6</option>
            </select>
          </Field>
          <Field label="Layout mode">
            <select className="form-input" value={layoutMode} onChange={event => setLayoutMode(event.target.value as ClipLayoutMode)}>
              <option value="blurred_background">Blurred background</option>
              <option value="fill_crop">Fill crop</option>
              <option value="fit_with_bars">Fit with bars</option>
            </select>
          </Field>
          <Field label="CTA size">
            <select className="form-input" value={ctaSize} onChange={event => setCtaSize(event.target.value as ClipCTASize)}>
              <option value="small">Small</option>
              <option value="medium">Medium</option>
              <option value="large">Large</option>
            </select>
          </Field>
        </div>

        <div className="form-grid three">
          <Field label="Top text"><input className="form-input" value={topText} onChange={event => setTopText(event.target.value)} /></Field>
          <Field label="Bottom text"><input className="form-input" value={bottomText} onChange={event => setBottomText(event.target.value)} /></Field>
          <Field label="Watermark / channel name"><input className="form-input" value={watermark} onChange={event => setWatermark(event.target.value)} /></Field>
        </div>

        <Field label="Caption text optional">
          <textarea className="form-textarea clip-caption-textarea" value={captionText} onChange={event => setCaptionText(event.target.value)} rows={2} placeholder="Leave blank to hide captions." />
        </Field>

        <label className="rights-check">
          <input type="checkbox" checked={rightsConfirmed} onChange={event => setRightsConfirmed(event.target.checked)} />
          I confirm I have rights or permission to use this source.
        </label>

        <details className="advanced-details">
          <summary>
            Rights & attribution
          </summary>
          <div className="form-grid three rights-grid">
            <Field label="Source model">
              <select className="form-input" value={sourceModel} onChange={event => setSourceModel(event.target.value as ClipSourceModel)}>
                {SOURCE_MODELS.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </Field>
            <Field label="Source title"><input className="form-input" value={sourceTitle} onChange={event => setSourceTitle(event.target.value)} /></Field>
            <Field label="Source creator"><input className="form-input" value={sourceCreator} onChange={event => setSourceCreator(event.target.value)} /></Field>
            <Field label="License"><input className="form-input" value={sourceLicense} onChange={event => setSourceLicense(event.target.value)} /></Field>
            <Field label="Attribution text"><input className="form-input" value={attributionText} onChange={event => setAttributionText(event.target.value)} /></Field>
            <Field label="Copyright overlay"><input className="form-input" value={copyrightOverlayText} onChange={event => setCopyrightOverlayText(event.target.value)} /></Field>
            <Field label="Platform source"><input className="form-input" value={platformSource} onChange={event => setPlatformSource(event.target.value)} /></Field>
          </div>
        </details>

        <div className="clip-action-row">
          <button className="generate-btn idle" onClick={handleGenerate} disabled={!canGenerate} title={generateDisabledReason || undefined} type="button">
            {busy ? 'Generating clips...' : 'Generate Clips'}
          </button>
          <button className="generate-btn secondary" onClick={handleDownload} disabled={downloadBusy || !packageReady} type="button">
            {downloadBusy ? 'Downloading...' : 'Download ZIP'}
          </button>
          <button className="generate-btn secondary" onClick={() => setPublishOpen(true)} disabled={!packageReady} title={packageReady ? undefined : 'Generate clips first.'} type="button">
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
      </div>
      {activeProjectID && (
        <ProjectOutputsPanel
          outputs={outputs}
          loading={outputsLoading}
          error={outputsError}
          busyOutputID={outputActionID}
          onRetry={handleRetryOutput}
          onDownload={handleOutputDownload}
          onArchive={setArchiveTarget}
        />
      )}
      <ConfirmationDialog
        open={Boolean(archiveTarget)}
        title="Archive output?"
        description="This removes the output record from the active project list. Generated files are not deleted by this action."
        confirmLabel="Archive output"
        intent="destructive"
        pending={Boolean(archiveTarget && outputActionID === archiveTarget.id)}
        onConfirm={() => void handleArchiveOutput()}
        onCancel={() => setArchiveTarget(null)}
      />
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

function ProjectContextCard({
  project,
  loading,
  error,
  sceneCount,
  plannedDuration,
  outputCount,
}: {
  project: ContentProject | null;
  loading: boolean;
  error: string | null;
  sceneCount: number;
  plannedDuration: number;
  outputCount: number;
}) {
  return (
    <div className="settings-card clip-project-context">
      <div className="clip-section-header">
        <div>
          <div className="settings-card-title">{loading ? 'Loading project...' : project?.title || 'Project context'}</div>
          <p>{error || 'Outputs created here are attached to this Content Project.'}</p>
        </div>
      </div>
      <div className="clip-project-stats">
        <StatusMetric label="Stage" value={project ? readableStage(project.current_stage) : 'Loading'} />
        <StatusMetric label="Target" value={project ? `${project.target_duration_seconds}s` : 'Loading'} />
        <StatusMetric label="Script" value={project?.main_script || project?.hook ? 'Available' : 'Not available'} />
        <StatusMetric label="Scenes" value={`${sceneCount}`} />
        <StatusMetric label="Planned" value={plannedDuration > 0 ? `${plannedDuration}s` : 'Not available'} />
        <StatusMetric label="Outputs" value={`${outputCount}`} />
      </div>
    </div>
  );
}

function ProjectOutputsPanel({
  outputs,
  loading,
  error,
  busyOutputID,
  onRetry,
  onDownload,
  onArchive,
}: {
  outputs: ContentProjectOutput[];
  loading: boolean;
  error: string | null;
  busyOutputID: string | null;
  onRetry: (output: ContentProjectOutput) => void;
  onDownload: (output: ContentProjectOutput) => void;
  onArchive: (output: ContentProjectOutput) => void;
}) {
  return (
    <div className="settings-card clip-project-outputs">
      <div className="clip-section-header">
        <div>
          <div className="settings-card-title">Project outputs</div>
          <p>Persistent records for generated, uploaded, and imported Clip Generator outputs.</p>
        </div>
      </div>
      {loading && <div className="neutral-callout">Loading project outputs...</div>}
      {error && <div className="clip-error">{error}</div>}
      {!loading && !error && outputs.length === 0 && (
        <div className="empty-panel">
          <div className="empty-title">No project outputs yet</div>
          <div className="empty-desc">Start a real upload, import, or generation to create the first output record.</div>
        </div>
      )}
      <div className="clip-output-list">
        {outputs.map(output => (
          <article className="clip-output-card" key={output.id}>
            <div className="clip-output-main">
              <div>
                <strong>{output.display_name}</strong>
                <span>{readableOutputTypeLabel(output.output_type)} · {output.output_scope === 'scene' ? output.scene_label || 'Scene output' : 'Project output'}</span>
              </div>
              <span className={`clip-output-status ${output.status}`}>{readableOutputStatus(output.status)}</span>
            </div>
            <div className="clip-output-meta">
              <span>{output.completed_at ? `Completed ${formatDateTime(output.completed_at)}` : `Created ${formatDateTime(output.created_at)}`}</span>
              {typeof output.file_size_bytes === 'number' && <span>{formatBytes(output.file_size_bytes)}</span>}
              {typeof output.duration_seconds === 'number' && <span>{formatDuration(output.duration_seconds)}</span>}
              {output.width && output.height && <span>{output.width}x{output.height}</span>}
              {output.failure_message && <span>{output.failure_message}</span>}
            </div>
            <div className="clip-output-actions">
              <button className="mini-copy-btn" type="button" disabled={!output.download_url || busyOutputID === output.id} onClick={() => onDownload(output)}>
                {busyOutputID === output.id ? 'Working...' : 'Download'}
              </button>
              <button className="mini-copy-btn" type="button" disabled={!output.retryable || busyOutputID === output.id} onClick={() => onRetry(output)}>Retry</button>
              <button className="mini-copy-btn danger" type="button" disabled={busyOutputID === output.id} onClick={() => onArchive(output)}>Archive</button>
            </div>
          </article>
        ))}
      </div>
    </div>
  );
}

function StatusMetric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function readableOutputStatus(status: ContentProjectOutput['status']): string {
  switch (status) {
    case 'queued': return 'Queued';
    case 'processing': return 'Rendering';
    case 'completed': return 'Ready';
    case 'failed': return 'Failed';
    case 'unavailable': return 'File unavailable';
    case 'archived': return 'Archived';
  }
}

function readableOutputTypeLabel(type: ContentProjectOutput['output_type']): string {
  switch (type) {
    case 'uploaded_clip': return 'Uploaded clip';
    case 'imported_clip': return 'Imported clip';
    case 'rendered_video': return 'Rendered video';
    case 'generated_clip': return 'Generated clip';
  }
}

function readableStage(stage: string): string {
  return stage.replace(/_/g, ' ').replace(/\b\w/g, char => char.toUpperCase());
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
}

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDuration(value: number): string {
  return `${Math.round(value)}s`;
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
            <h2>Publish generated clips</h2>
            <p>Choose where you want to publish this package.</p>
          </div>
          <button className="modal-close" type="button" onClick={onClose} aria-label="Close publish modal">x</button>
        </div>

        {!connectionsLoaded && (
          <div className="neutral-callout">Loading account status...</div>
        )}

        {connectionsLoaded && !hasConnected && (
          <>
            <div className="neutral-callout">
              <strong>Connect social accounts first</strong>
              <div>Connect accounts to publish directly, or download the ZIP for manual posting.</div>
            </div>
            <div className="publish-actions">
              <button className="generate-btn idle" type="button" onClick={onConnections}>Go to Connections</button>
              <button className="generate-btn idle secondary" type="button" onClick={onDownload} disabled={downloadDisabled}>{downloadLabel}</button>
              <button className="generate-btn idle secondary" type="button" onClick={onClose}>Cancel</button>
            </div>
          </>
        )}

        {connectionsLoaded && hasConnected && (
          <>
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
