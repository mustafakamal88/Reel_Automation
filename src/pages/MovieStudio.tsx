import { useEffect, useMemo, useRef, useState } from 'react';
import {
  apiUrl,
  autoDraftMovieEdit,
  createMovieEdit,
  duplicateMovieEdit,
  getMovieEdit,
  getMovieRender,
  listAssets,
  listContentProjects,
  listMovieEdits,
  renderMovieEdit,
  setActiveMovieEdit,
  setActiveProjectVideo,
  updateMovieEdit,
  uploadMovieMedia,
  type ContentProject,
  type MediaAsset,
  type MovieEdit,
  type MovieRender,
  type MovieScene,
} from '../lib/api/client';
import type { View } from '../types';

interface MovieStudioPageProps {
  onNavigate: (view: View, projectID?: string) => void;
}

const FIT_MODES = [
  ['fill_crop', 'Fill / crop'],
  ['fit_background', 'Fit with background'],
  ['original', 'Original framing'],
] as const;

const MOTIONS = [
  ['none', 'None'],
  ['slow_zoom_in', 'Slow zoom in'],
  ['slow_zoom_out', 'Slow zoom out'],
  ['pan_left', 'Pan left'],
  ['pan_right', 'Pan right'],
  ['pan_up', 'Pan up'],
  ['pan_down', 'Pan down'],
] as const;

const TRANSITIONS = [
  ['cut', 'Cut'],
  ['crossfade', 'Crossfade'],
  ['fade_black', 'Fade through black'],
  ['slide', 'Slide'],
] as const;

export function MovieStudioPage({ onNavigate }: MovieStudioPageProps) {
  const initialProjectID = useMemo(() => new URLSearchParams(window.location.search).get('project_id') || '', []);
  const [projects, setProjects] = useState<ContentProject[]>([]);
  const [projectID, setProjectID] = useState(initialProjectID);
  const [edit, setEdit] = useState<MovieEdit | null>(null);
  const [assets, setAssets] = useState<MediaAsset[]>([]);
  const [selectedSceneID, setSelectedSceneID] = useState('');
  const [selectedAssetID, setSelectedAssetID] = useState('');
  const [leftTab, setLeftTab] = useState<'scenes' | 'assets' | 'audio' | 'brand'>('scenes');
  const [rightTab, setRightTab] = useState<'scene' | 'captions' | 'audio' | 'export'>('scene');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [rendering, setRendering] = useState(false);
  const [renderJob, setRenderJob] = useState<MovieRender | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([listContentProjects(), listMovieEdits(initialProjectID || undefined)])
      .then(([projectResp, editResp]) => {
        if (cancelled) return;
        setProjects(projectResp.projects);
        if (editResp.edits[0]) {
          setEdit(editResp.edits[0]);
          setProjectID(editResp.edits[0].content_project_id || initialProjectID);
          setSelectedSceneID(editResp.edits[0].scenes[0]?.id || '');
        }
      })
      .catch(err => setError(friendlyMovieError(err, 'Movie Studio is unavailable. Check that the workspace backend is running.')))
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [initialProjectID]);

  useEffect(() => {
    if (!projectID) {
      setAssets([]);
      return;
    }
    let cancelled = false;
    listAssets({ project_id: projectID, limit: 80 })
      .then(resp => { if (!cancelled) setAssets(resp.assets.filter(asset => asset.status === 'ready')); })
      .catch(() => { if (!cancelled) setAssets([]); });
    return () => { cancelled = true; };
  }, [projectID]);

  useEffect(() => {
    if (!renderJob || !['queued', 'preparing_assets', 'probing_media', 'rendering_scenes', 'mixing_audio', 'encoding_final', 'uploading'].includes(renderJob.status)) return undefined;
    const id = window.setInterval(() => {
      getMovieRender(renderJob.id)
        .then(resp => {
          setRenderJob(resp.render);
          if (resp.render.status === 'completed') {
            setRendering(false);
            if (edit) getMovieEdit(edit.id).then(next => setEdit(next.edit)).catch(() => {});
          }
          if (resp.render.status === 'failed') setRendering(false);
        })
        .catch(() => {});
    }, 2500);
    return () => window.clearInterval(id);
  }, [edit, renderJob]);

  const selectedProject = projects.find(project => project.id === projectID);
  const selectedScene = edit?.scenes.find(scene => scene.id === selectedSceneID) || edit?.scenes[0];
  const selectedVisual = assets.find(asset => asset.id === selectedScene?.visual_asset_id);
  const completedRender = edit?.renders?.find(render => render.status === 'completed' && render.output_asset_id);
  const totalDuration = useMemo(() => edit?.scenes.reduce((sum, scene) => sum + scene.duration_seconds, 0) || 0, [edit]);
  const missingVisuals = edit?.scenes.filter(scene => !scene.visual_asset_id).length || 0;
  const visualAssets = assets.filter(asset => asset.asset_type.includes('video') || asset.mime_type?.startsWith('image/') || asset.asset_type === 'thumbnail');
  const audioAssets = assets.filter(asset => asset.asset_type === 'audio' || asset.asset_type === 'voiceover');

  async function chooseProject(nextProjectID: string) {
    setProjectID(nextProjectID);
    setError(null);
    setMessage(null);
    if (!nextProjectID) return;
    window.history.replaceState(null, '', `/movie-studio?project_id=${encodeURIComponent(nextProjectID)}`);
    const editResp = await listMovieEdits(nextProjectID);
    if (editResp.edits[0]) {
      setEdit(editResp.edits[0]);
      setSelectedSceneID(editResp.edits[0].scenes[0]?.id || '');
      return;
    }
    const project = projects.find(item => item.id === nextProjectID);
    const created = await createMovieEdit({ content_project_id: nextProjectID, name: `${project?.title || 'Movie'} draft`, caption_settings: { enabled: true, mode: 'phrase' } });
    setEdit(created.edit);
    setSelectedSceneID(created.edit.scenes[0]?.id || '');
  }

  async function createDraftFromProject() {
    if (!projectID) return;
    setSaving(true);
    setError(null);
    try {
      const project = projects.find(item => item.id === projectID);
      const created = edit || (await createMovieEdit({ content_project_id: projectID, name: `${project?.title || 'Movie'} draft`, caption_settings: { enabled: true, mode: 'phrase' } })).edit;
      const drafted = await autoDraftMovieEdit(created.id);
      setEdit(drafted.edit);
      setSelectedSceneID(drafted.edit.scenes[0]?.id || '');
      setMessage('Automatic draft saved.');
    } catch (err) {
      setError(friendlyMovieError(err, 'Automatic draft failed. Your project was not changed.'));
    } finally {
      setSaving(false);
    }
  }

  async function saveEdit(nextEdit = edit) {
    if (!nextEdit) return;
    setSaving(true);
    setError(null);
    try {
      const saved = await updateMovieEdit(nextEdit.id, {
        name: nextEdit.name,
        quality_preset: nextEdit.quality_preset,
        voiceover_asset_id: nextEdit.voiceover_asset_id,
        music_asset_id: nextEdit.music_asset_id,
        caption_settings: nextEdit.caption_settings,
        branding_settings: nextEdit.branding_settings,
        scenes: nextEdit.scenes,
      });
      setEdit(saved.edit);
      setMessage('Saved.');
    } catch (err) {
      setError(friendlyMovieError(err, 'Save failed. Your edit is still open in the browser.'));
    } finally {
      setSaving(false);
    }
  }

  async function startRender() {
    if (!edit) return;
    if (missingVisuals > 0) {
      setError(`${missingVisuals === 1 ? 'One scene needs' : `${missingVisuals} scenes need`} a visual before rendering.`);
      return;
    }
    setRendering(true);
    setError(null);
    try {
      const saved = await updateMovieEdit(edit.id, {
        name: edit.name,
        quality_preset: edit.quality_preset,
        voiceover_asset_id: edit.voiceover_asset_id,
        music_asset_id: edit.music_asset_id,
        caption_settings: edit.caption_settings,
        branding_settings: edit.branding_settings,
        scenes: edit.scenes,
      });
      setEdit(saved.edit);
      const resp = await renderMovieEdit(saved.edit.id, `${saved.edit.id}-${Date.now()}`);
      setRenderJob(resp.render);
      if (resp.render.status === 'completed') {
        setRendering(false);
        const refreshed = await getMovieEdit(saved.edit.id);
        setEdit(refreshed.edit);
      }
    } catch (err) {
      setRendering(false);
      setError(friendlyMovieError(err, 'Render failed to start. Your edit has been saved and can be retried.'));
    }
  }

  function patchEdit(patch: Partial<MovieEdit>) {
    if (!edit) return;
    setEdit({ ...edit, ...patch });
  }

  function patchScene(sceneID: string | undefined, patch: Partial<MovieScene>) {
    if (!edit || !sceneID) return;
    setEdit({
      ...edit,
      scenes: edit.scenes.map(scene => scene.id === sceneID ? { ...scene, ...patch } : scene),
    });
  }

  function assignAsset(assetID = selectedAssetID) {
    if (!selectedScene || !assetID) return;
    patchScene(selectedScene.id, { visual_asset_id: assetID });
  }

  async function uploadMedia(file?: File | null) {
    if (!file) return;
    setSaving(true);
    setError(null);
    try {
      const resp = await uploadMovieMedia(file, projectID || undefined);
      setAssets(current => [resp.asset, ...current]);
      setSelectedAssetID(resp.asset.id);
      if (selectedScene && !resp.asset.mime_type?.startsWith('audio/')) patchScene(selectedScene.id, { visual_asset_id: resp.asset.id });
    } catch (err) {
      setError(friendlyMovieError(err, 'Upload failed. Choose a supported video, image, or audio file.'));
    } finally {
      setSaving(false);
    }
  }

  async function duplicateCurrentEdit() {
    if (!edit) return;
    const resp = await duplicateMovieEdit(edit.id);
    setEdit(resp.edit);
    setSelectedSceneID(resp.edit.scenes[0]?.id || '');
  }

  async function markActive() {
    if (!edit?.content_project_id) return;
    await setActiveMovieEdit(edit.content_project_id, edit.id);
    if (completedRender?.output_asset_id) await setActiveProjectVideo(edit.content_project_id, completedRender.output_asset_id);
    setMessage('Marked as active project video.');
  }

  function playPreview() {
    const video = videoRef.current;
    if (video && selectedVisual?.preview_url && selectedVisual.mime_type?.startsWith('video/')) {
      if (previewPlaying) video.pause();
      else void video.play();
      setPreviewPlaying(!previewPlaying);
      return;
    }
    setPreviewPlaying(!previewPlaying);
  }

  return (
    <div className="movie-studio-page">
      <section className="movie-studio-topbar">
        <div>
          <div className="page-eyebrow">CONTENT</div>
          <h1>Movie Studio</h1>
          <p>Assemble a persistent vertical video edit from project scenes, narration, visuals, captions, music, and rendered versions.</p>
        </div>
        <div className="movie-actions">
          <select value={projectID} onChange={event => void chooseProject(event.target.value)} aria-label="Content project">
            <option value="">Choose project</option>
            {projects.map(project => <option key={project.id} value={project.id}>{project.title}</option>)}
          </select>
          <button className="generate-btn secondary" type="button" onClick={() => onNavigate('voiceStudio', projectID)} disabled={!projectID}>Voice</button>
          <button className="generate-btn secondary" type="button" onClick={() => void saveEdit()} disabled={!edit || saving}>{saving ? 'Saving...' : 'Save'}</button>
          <button className="generate-btn" type="button" onClick={() => void startRender()} disabled={!edit || rendering}>{rendering ? 'Rendering...' : 'Render Video'}</button>
        </div>
      </section>

      {loading && <div className="movie-state">Loading Movie Studio...</div>}
      {error && <div className="confirmation-error" role="alert">{error}</div>}
      {message && <div className="movie-state success">{message}</div>}

      {!loading && !edit && (
        <section className="movie-empty-state">
          <h2>Start with a Content Project</h2>
          <p>Choose a saved project to build the first editable movie draft from its scene plan, active narration, and linked visuals.</p>
          <button className="generate-btn" type="button" onClick={() => void createDraftFromProject()} disabled={!projectID || saving}>Create Movie Draft</button>
        </section>
      )}

      {edit && (
        <section className="movie-workspace">
          <aside className="movie-panel movie-left-panel">
            <div className="movie-tabs" role="tablist" aria-label="Movie tools">
              {(['scenes', 'assets', 'audio', 'brand'] as const).map(tab => <button key={tab} className={leftTab === tab ? 'active' : ''} type="button" onClick={() => setLeftTab(tab)}>{tab}</button>)}
            </div>
            {leftTab === 'scenes' && (
              <div className="movie-scene-list">
                <button className="generate-btn secondary" type="button" onClick={() => void createDraftFromProject()} disabled={saving}>Refresh auto-draft</button>
                {edit.scenes.map(scene => (
                  <button key={scene.id || scene.position} className={`movie-scene-card${selectedScene?.id === scene.id ? ' active' : ''}`} type="button" onClick={() => setSelectedSceneID(scene.id || '')}>
                    <span>Scene {scene.position}</span>
                    <strong>{scene.title || 'Untitled scene'}</strong>
                    <small>{formatDuration(scene.duration_seconds)} · {scene.visual_asset_id ? 'visual assigned' : 'needs visual'}</small>
                  </button>
                ))}
              </div>
            )}
            {leftTab === 'assets' && (
              <div className="movie-asset-list">
                <label className="movie-upload">
                  <span>Upload media</span>
                  <input type="file" accept="video/*,image/*,audio/*" onChange={event => void uploadMedia(event.target.files?.[0])} />
                </label>
                <select value={selectedAssetID} onChange={event => setSelectedAssetID(event.target.value)} aria-label="Visual asset">
                  <option value="">Choose visual</option>
                  {visualAssets.map(asset => <option key={asset.id} value={asset.id}>{asset.display_name}</option>)}
                </select>
                <button className="generate-btn secondary" type="button" onClick={() => assignAsset()} disabled={!selectedScene || !selectedAssetID}>Assign to scene</button>
                {visualAssets.slice(0, 18).map(asset => (
                  <button key={asset.id} className="movie-asset-row" type="button" onClick={() => { setSelectedAssetID(asset.id); assignAsset(asset.id); }}>
                    <span>{asset.display_name}</span>
                    <small>{asset.asset_type} · {asset.duration_seconds ? formatDuration(asset.duration_seconds) : asset.mime_type}</small>
                  </button>
                ))}
              </div>
            )}
            {leftTab === 'audio' && (
              <div className="movie-field-stack">
                <label>
                  <span>Narration</span>
                  <select value={edit.voiceover_asset_id || ''} onChange={event => patchEdit({ voiceover_asset_id: event.target.value })}>
                    <option value="">No narration</option>
                    {audioAssets.map(asset => <option key={asset.id} value={asset.id}>{asset.display_name}</option>)}
                  </select>
                </label>
                <label>
                  <span>Music</span>
                  <select value={edit.music_asset_id || ''} onChange={event => patchEdit({ music_asset_id: event.target.value })}>
                    <option value="">No music</option>
                    {audioAssets.filter(asset => asset.asset_type !== 'voiceover').map(asset => <option key={asset.id} value={asset.id}>{asset.display_name}</option>)}
                  </select>
                </label>
              </div>
            )}
            {leftTab === 'brand' && (
              <div className="movie-field-stack">
                <label><span>Watermark text</span><input value={String(edit.branding_settings.text || '')} onChange={event => patchEdit({ branding_settings: { ...edit.branding_settings, text: event.target.value } })} /></label>
                <label><span>Position</span><select value={String(edit.branding_settings.position || 'top_right')} onChange={event => patchEdit({ branding_settings: { ...edit.branding_settings, position: event.target.value } })}><option value="top_right">Top right</option><option value="bottom_right">Bottom right</option></select></label>
              </div>
            )}
          </aside>

          <main className="movie-preview-column">
            <div className="movie-preview-shell">
              <div className="movie-preview-canvas">
                {selectedVisual?.mime_type?.startsWith('video/') && selectedVisual.preview_url ? (
                  <video ref={videoRef} src={apiUrl(selectedVisual.preview_url)} muted playsInline />
                ) : selectedVisual?.preview_url ? (
                  <img src={apiUrl(selectedVisual.preview_url)} alt="" />
                ) : (
                  <div className="movie-preview-empty">Scene needs a visual</div>
                )}
                <div className="movie-safe-area" aria-hidden="true" />
                {edit.caption_settings.enabled !== false && <div className="movie-caption-preview">{String(selectedScene?.caption_text || selectedScene?.script_text || selectedProject?.caption || '')}</div>}
                {Boolean(edit.branding_settings.text) && <div className="movie-brand-preview">{String(edit.branding_settings.text)}</div>}
              </div>
              <div className="movie-preview-controls">
                <button type="button" onClick={playPreview}>{previewPlaying ? 'Pause' : 'Play'}</button>
                <span>{formatDuration(totalDuration)} · 1080x1920 · {edit.frame_rate} fps</span>
                <span>{missingVisuals ? `${missingVisuals} missing visuals` : 'Ready to render'}</span>
              </div>
            </div>
            <div className="movie-timeline" aria-label="Scene timeline">
              {edit.scenes.map(scene => (
                <button key={scene.id || scene.position} type="button" className={selectedScene?.id === scene.id ? 'active' : ''} onClick={() => setSelectedSceneID(scene.id || '')}>
                  <span>{scene.position}</span>
                  <strong>{formatDuration(scene.duration_seconds)}</strong>
                </button>
              ))}
            </div>
          </main>

          <aside className="movie-panel movie-right-panel">
            <div className="movie-tabs" role="tablist" aria-label="Scene controls">
              {(['scene', 'captions', 'audio', 'export'] as const).map(tab => <button key={tab} className={rightTab === tab ? 'active' : ''} type="button" onClick={() => setRightTab(tab)}>{tab}</button>)}
            </div>
            {rightTab === 'scene' && selectedScene && (
              <div className="movie-field-stack">
                <label><span>Scene title</span><input value={selectedScene.title} onChange={event => patchScene(selectedScene.id, { title: event.target.value })} /></label>
                <label><span>Duration</span><input type="number" min="1" max="120" step="0.5" value={selectedScene.duration_seconds} onChange={event => patchScene(selectedScene.id, { duration_seconds: Number(event.target.value) })} /></label>
                <label><span>Fit</span><select value={selectedScene.fit_mode} onChange={event => patchScene(selectedScene.id, { fit_mode: event.target.value as MovieScene['fit_mode'] })}>{FIT_MODES.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
                <label><span>Motion</span><select value={selectedScene.motion_preset} onChange={event => patchScene(selectedScene.id, { motion_preset: event.target.value as MovieScene['motion_preset'] })}>{MOTIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
                <label><span>Transition</span><select value={selectedScene.transition_type} onChange={event => patchScene(selectedScene.id, { transition_type: event.target.value as MovieScene['transition_type'] })}>{TRANSITIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
                <label><span>Trim start</span><input type="number" min="0" step="0.1" value={selectedScene.trim_in_seconds ?? ''} onChange={event => patchScene(selectedScene.id, { trim_in_seconds: event.target.value === '' ? undefined : Number(event.target.value) })} /></label>
                <label><span>Trim end</span><input type="number" min="0" step="0.1" value={selectedScene.trim_out_seconds ?? ''} onChange={event => patchScene(selectedScene.id, { trim_out_seconds: event.target.value === '' ? undefined : Number(event.target.value) })} /></label>
              </div>
            )}
            {rightTab === 'captions' && (
              <div className="movie-field-stack">
                <label className="movie-check"><input type="checkbox" checked={edit.caption_settings.enabled !== false} onChange={event => patchEdit({ caption_settings: { ...edit.caption_settings, enabled: event.target.checked } })} /> Captions enabled</label>
                {selectedScene && <label><span>Caption text</span><textarea value={selectedScene.caption_text || ''} onChange={event => patchScene(selectedScene.id, { caption_text: event.target.value })} /></label>}
              </div>
            )}
            {rightTab === 'audio' && (
              <div className="movie-field-stack">
                <p>Narration defaults to the project voiceover when available. Music is mixed under narration in the final render.</p>
                <button className="generate-btn secondary" type="button" onClick={() => onNavigate('voiceStudio', projectID)} disabled={!projectID}>Open Voice Studio</button>
              </div>
            )}
            {rightTab === 'export' && (
              <div className="movie-field-stack">
                <label><span>Movie name</span><input value={edit.name} onChange={event => patchEdit({ name: event.target.value })} /></label>
                <label><span>Quality</span><select value={edit.quality_preset} onChange={event => patchEdit({ quality_preset: event.target.value as MovieEdit['quality_preset'] })}><option value="draft">Draft</option><option value="standard">Standard</option><option value="high">High</option></select></label>
                {renderJob && <div className={`movie-render-status ${renderJob.status}`}><strong>{renderJob.current_stage}</strong><span>{renderJob.completed_scene_count}/{renderJob.total_scene_count} scenes</span>{renderJob.failure_message && <small>{renderJob.failure_message}</small>}</div>}
                {completedRender?.output_asset_id && <a className="generate-btn secondary movie-download-link" href={apiUrl(`/api/movie-studio/renders/${completedRender.id}/download`)}>Download MP4</a>}
                <button className="generate-btn secondary" type="button" onClick={() => void markActive()} disabled={!completedRender?.output_asset_id}>Mark active/final</button>
                <button className="generate-btn secondary" type="button" onClick={() => void duplicateCurrentEdit()}>Duplicate draft</button>
              </div>
            )}
          </aside>
        </section>
      )}
    </div>
  );
}

function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
  const rounded = Math.round(seconds);
  const mins = Math.floor(rounded / 60);
  const secs = rounded % 60;
  return `${mins}:${String(secs).padStart(2, '0')}`;
}

function friendlyMovieError(error: unknown, fallback: string): string {
  const raw = error instanceof Error ? error.message : '';
  if (!raw || /^HTTP\s+\d+/i.test(raw) || raw.includes('Cannot reach')) return fallback;
  return raw.replace(/\bffmpeg\b/gi, 'video processing');
}
