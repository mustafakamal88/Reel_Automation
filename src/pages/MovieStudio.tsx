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
  const [playhead, setPlayhead] = useState(0);
  const [volume, setVolume] = useState(0.8);
  const [narrationMuted, setNarrationMuted] = useState(false);
  const [musicMuted, setMusicMuted] = useState(false);
  const [safeGuides, setSafeGuides] = useState(true);
  const [viewerMode, setViewerMode] = useState<'preview' | 'final'>('preview');
  const [saveState, setSaveState] = useState<'saved' | 'saving' | 'unsaved' | 'failed'>('saved');
  const [undoStack, setUndoStack] = useState<MovieEdit[]>([]);
  const [redoStack, setRedoStack] = useState<MovieEdit[]>([]);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const finalVideoRef = useRef<HTMLVideoElement | null>(null);

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
  const playbackScene = useMemo(() => sceneAtTime(edit?.scenes || [], playhead), [edit, playhead]);
  const selectedScene = edit?.scenes.find(scene => scene.id === selectedSceneID) || playbackScene || edit?.scenes[0];
  const selectedVisual = assets.find(asset => asset.id === (viewerMode === 'preview' ? (playbackScene || selectedScene)?.visual_asset_id : selectedScene?.visual_asset_id));
  const completedRender = edit?.renders?.find(render => render.status === 'completed' && render.output_asset_id);
  const totalDuration = useMemo(() => edit?.scenes.reduce((sum, scene) => sum + scene.duration_seconds, 0) || 0, [edit]);
  const missingVisuals = edit?.scenes.filter(scene => !scene.visual_asset_id).length || 0;
  const visualAssets = assets.filter(asset => asset.asset_type.includes('video') || asset.mime_type?.startsWith('image/') || asset.asset_type === 'thumbnail');
  const audioAssets = assets.filter(asset => asset.asset_type === 'audio' || asset.asset_type === 'voiceover');
  const finalAsset = assets.find(asset => asset.id === completedRender?.output_asset_id);
  const preflight = useMemo(() => buildPreflight(edit, assets), [edit, assets]);

  useEffect(() => {
    if (!previewPlaying || viewerMode !== 'preview') return undefined;
    const id = window.setInterval(() => {
      setPlayhead(current => {
        const next = current + 0.1;
        if (next >= totalDuration) {
          setPreviewPlaying(false);
          return totalDuration;
        }
        return next;
      });
    }, 100);
    return () => window.clearInterval(id);
  }, [previewPlaying, totalDuration, viewerMode]);

  useEffect(() => {
    if (!edit || saveState !== 'unsaved') return undefined;
    const id = window.setTimeout(() => {
      void saveEdit(edit, true);
    }, 900);
    return () => window.clearTimeout(id);
  }, [edit, saveState]);

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

  async function saveEdit(nextEdit = edit, quiet = false) {
    if (!nextEdit) return;
    setSaving(true);
    setError(null);
    try {
      setSaveState('saving');
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
      setSaveState('saved');
      if (!quiet) setMessage('Saved.');
    } catch (err) {
      setSaveState('failed');
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
    remember(edit);
    setEdit({ ...edit, ...patch });
    setSaveState('unsaved');
  }

  function patchScene(sceneID: string | undefined, patch: Partial<MovieScene>) {
    if (!edit || !sceneID) return;
    remember(edit);
    setEdit({
      ...edit,
      scenes: edit.scenes.map(scene => scene.id === sceneID ? { ...scene, ...patch } : scene),
    });
    setSaveState('unsaved');
  }

  function remember(snapshot: MovieEdit) {
    setUndoStack(stack => [...stack.slice(-24), snapshot]);
    setRedoStack([]);
  }

  function restore(next: MovieEdit, redoFrom?: MovieEdit) {
    if (redoFrom) setRedoStack(stack => [...stack.slice(-24), redoFrom]);
    setEdit(next);
    setSelectedSceneID(next.scenes[0]?.id || '');
    setSaveState('unsaved');
  }

  function undo() {
    if (!edit || undoStack.length === 0) return;
    const previous = undoStack[undoStack.length - 1];
    setUndoStack(stack => stack.slice(0, -1));
    restore(previous, edit);
  }

  function redo() {
    if (!edit || redoStack.length === 0) return;
    const next = redoStack[redoStack.length - 1];
    setRedoStack(stack => stack.slice(0, -1));
    remember(edit);
    setEdit(next);
    setSaveState('unsaved');
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

  function moveScene(sceneID: string | undefined, direction: -1 | 1) {
    if (!edit || !sceneID) return;
    const idx = edit.scenes.findIndex(scene => scene.id === sceneID);
    const target = idx + direction;
    if (idx < 0 || target < 0 || target >= edit.scenes.length) return;
    remember(edit);
    const scenes = [...edit.scenes];
    const [scene] = scenes.splice(idx, 1);
    scenes.splice(target, 0, scene);
    setEdit({ ...edit, scenes: recalculateScenes(scenes) });
    setSelectedSceneID(sceneID);
    setSaveState('unsaved');
  }

  function duplicateScene(sceneID: string | undefined) {
    if (!edit || !sceneID) return;
    const idx = edit.scenes.findIndex(scene => scene.id === sceneID);
    if (idx < 0) return;
    remember(edit);
    const source = edit.scenes[idx];
    const copy = { ...source, id: undefined, title: `${source.title || `Scene ${idx + 1}`} copy`, voiceover_asset_id: undefined };
    const scenes = [...edit.scenes.slice(0, idx + 1), copy, ...edit.scenes.slice(idx + 1)];
    setEdit({ ...edit, scenes: recalculateScenes(scenes) });
    setSaveState('unsaved');
  }

  function deleteScene(sceneID: string | undefined) {
    if (!edit || !sceneID || edit.scenes.length <= 1) return;
    remember(edit);
    const scenes = recalculateScenes(edit.scenes.filter(scene => scene.id !== sceneID));
    setEdit({ ...edit, scenes });
    setSelectedSceneID(scenes[0]?.id || '');
    setSaveState('unsaved');
  }

  function splitScene() {
    if (!edit || !selectedScene) return;
    const sceneStart = selectedScene.start_seconds || 0;
    const offset = playhead - sceneStart;
    if (offset <= 0.5 || offset >= selectedScene.duration_seconds - 0.5) {
      setError('Move the playhead inside the selected scene before splitting.');
      return;
    }
    remember(edit);
    const idx = edit.scenes.findIndex(scene => scene === selectedScene || scene.id === selectedScene.id);
    const first = { ...selectedScene, duration_seconds: offset };
    const second = { ...selectedScene, id: undefined, title: `${selectedScene.title || 'Scene'} continued`, duration_seconds: selectedScene.duration_seconds - offset, voiceover_asset_id: undefined };
    const scenes = recalculateScenes([...edit.scenes.slice(0, idx), first, second, ...edit.scenes.slice(idx + 1)]);
    setEdit({ ...edit, scenes });
    setSaveState('unsaved');
  }

  async function markActive() {
    if (!edit?.content_project_id) return;
    await setActiveMovieEdit(edit.content_project_id, edit.id);
    if (completedRender?.output_asset_id) await setActiveProjectVideo(edit.content_project_id, completedRender.output_asset_id);
    setMessage('Marked as active project video.');
  }

  function playPreview() {
    const video = videoRef.current;
    if (viewerMode === 'final') {
      const finalVideo = finalVideoRef.current;
      if (!finalVideo) return;
      if (previewPlaying) finalVideo.pause();
      else void finalVideo.play();
      setPreviewPlaying(!previewPlaying);
      return;
    }
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
          <button className="generate-btn secondary" type="button" onClick={undo} disabled={!undoStack.length}>Undo</button>
          <button className="generate-btn secondary" type="button" onClick={redo} disabled={!redoStack.length}>Redo</button>
          <span className={`movie-save-state ${saveState}`}>{saveState}</span>
          <button className="generate-btn secondary" type="button" onClick={() => void saveEdit()} disabled={!edit || saving}>{saving ? 'Saving...' : 'Save'}</button>
          <button className="generate-btn" type="button" onClick={() => void startRender()} disabled={!edit || rendering || preflight.errors.length > 0}>{rendering ? 'Rendering...' : 'Render Video'}</button>
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
            <div className="movie-view-mode" role="tablist" aria-label="Player mode">
              <button className={viewerMode === 'preview' ? 'active' : ''} type="button" onClick={() => setViewerMode('preview')}>Edit Preview</button>
              <button className={viewerMode === 'final' ? 'active' : ''} type="button" onClick={() => setViewerMode('final')} disabled={!completedRender?.output_asset_id}>Final Render</button>
            </div>
            <div className="movie-preview-shell">
              <div className="movie-preview-canvas">
                {viewerMode === 'final' && completedRender?.output_asset_id ? (
                  <video ref={finalVideoRef} src={apiUrl(`/api/movie-studio/renders/${completedRender.id}/download`)} controls playsInline />
                ) : selectedVisual?.mime_type?.startsWith('video/') && selectedVisual.preview_url ? (
                  <video ref={videoRef} src={apiUrl(selectedVisual.preview_url)} muted playsInline />
                ) : selectedVisual?.preview_url ? (
                  <img className={`motion-${(playbackScene || selectedScene)?.motion_preset || 'none'}`} style={{ objectFit: selectedScene?.fit_mode === 'original' ? 'contain' : 'cover', transformOrigin: `${(selectedScene?.focal_x ?? 0.5) * 100}% ${(selectedScene?.focal_y ?? 0.5) * 100}%` }} src={apiUrl(selectedVisual.preview_url)} alt="" />
                ) : (
                  <div className="movie-preview-empty">Scene needs a visual</div>
                )}
                {safeGuides && <div className="movie-safe-area" aria-hidden="true" />}
                {edit.caption_settings.enabled !== false && <div className="movie-caption-preview">{String(selectedScene?.caption_text || selectedScene?.script_text || selectedProject?.caption || '')}</div>}
                {Boolean(edit.branding_settings.text) && <div className="movie-brand-preview">{String(edit.branding_settings.text)}</div>}
              </div>
              <div className="movie-preview-controls">
                <button type="button" onClick={() => setPlayhead(0)}>Restart</button>
                <button type="button" onClick={playPreview}>{previewPlaying ? 'Pause' : 'Play'}</button>
                <button type="button" onClick={() => seekScene(edit.scenes, playhead, -1, setPlayhead, setSelectedSceneID)}>Prev</button>
                <button type="button" onClick={() => seekScene(edit.scenes, playhead, 1, setPlayhead, setSelectedSceneID)}>Next</button>
                <input aria-label="Timeline scrubber" type="range" min="0" max={Math.max(totalDuration, 0.1)} step="0.1" value={playhead} onChange={event => { const next = Number(event.target.value); setPlayhead(next); setSelectedSceneID(sceneAtTime(edit.scenes, next)?.id || ''); }} />
                <span>{formatDuration(playhead)} / {formatDuration(totalDuration)} · 1080x1920 · {edit.frame_rate} fps</span>
                <label className="movie-inline-control"><span>Vol</span><input type="range" min="0" max="1" step="0.05" value={volume} onChange={event => setVolume(Number(event.target.value))} /></label>
                <label className="movie-check"><input type="checkbox" checked={narrationMuted} onChange={event => setNarrationMuted(event.target.checked)} /> Narration mute</label>
                <label className="movie-check"><input type="checkbox" checked={musicMuted} onChange={event => setMusicMuted(event.target.checked)} /> Music mute</label>
                <label className="movie-check"><input type="checkbox" checked={safeGuides} onChange={event => setSafeGuides(event.target.checked)} /> Safe area</label>
              </div>
            </div>
            <div className="movie-timeline" aria-label="Scene timeline">
              {edit.scenes.map(scene => (
                <button key={scene.id || scene.position} type="button" style={{ flexBasis: `${Math.max(86, scene.duration_seconds * 18)}px` }} className={selectedScene?.id === scene.id ? 'active' : ''} onClick={() => { setSelectedSceneID(scene.id || ''); setPlayhead(scene.start_seconds || 0); }}>
                  <span>{scene.position}</span>
                  <small>{scene.transition_type}</small>
                  <strong>{formatDuration(scene.duration_seconds)}</strong>
                </button>
              ))}
              <div className="movie-playhead" style={{ left: `${totalDuration ? (playhead / totalDuration) * 100 : 0}%` }} />
            </div>
            <div className={`movie-preflight ${preflight.errors.length ? 'blocked' : 'ready'}`}>
              <strong>{preflight.errors.length ? 'Preflight needs attention' : 'Ready to render'}</strong>
              {(preflight.errors.length ? preflight.errors : preflight.warnings).slice(0, 4).map(item => <span key={item}>{item}</span>)}
            </div>
          </main>

          <aside className="movie-panel movie-right-panel">
            <div className="movie-tabs" role="tablist" aria-label="Scene controls">
              {(['scene', 'captions', 'audio', 'export'] as const).map(tab => <button key={tab} className={rightTab === tab ? 'active' : ''} type="button" onClick={() => setRightTab(tab)}>{tab}</button>)}
            </div>
            {rightTab === 'scene' && selectedScene && (
              <div className="movie-field-stack">
                <label><span>Scene title</span><input value={selectedScene.title} onChange={event => patchScene(selectedScene.id, { title: event.target.value })} /></label>
                <div className="movie-button-grid">
                  <button type="button" onClick={() => moveScene(selectedScene.id, -1)}>Move left</button>
                  <button type="button" onClick={() => moveScene(selectedScene.id, 1)}>Move right</button>
                  <button type="button" onClick={splitScene}>Split</button>
                  <button type="button" onClick={() => duplicateScene(selectedScene.id)}>Duplicate</button>
                  <button type="button" onClick={() => deleteScene(selectedScene.id)}>Delete</button>
                </div>
                <label><span>Duration</span><input type="number" min="1" max="120" step="0.5" value={selectedScene.duration_seconds} onChange={event => patchScene(selectedScene.id, { duration_seconds: Number(event.target.value) })} /></label>
                <label><span>Fit</span><select value={selectedScene.fit_mode} onChange={event => patchScene(selectedScene.id, { fit_mode: event.target.value as MovieScene['fit_mode'] })}>{FIT_MODES.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
                <label><span>Focal X</span><input type="range" min="0" max="1" step="0.01" value={selectedScene.focal_x ?? 0.5} onChange={event => patchScene(selectedScene.id, { focal_x: Number(event.target.value) })} /></label>
                <label><span>Focal Y</span><input type="range" min="0" max="1" step="0.01" value={selectedScene.focal_y ?? 0.5} onChange={event => patchScene(selectedScene.id, { focal_y: Number(event.target.value) })} /></label>
                <label><span>Zoom</span><input type="range" min="0.5" max="3" step="0.05" value={selectedScene.zoom ?? 1} onChange={event => patchScene(selectedScene.id, { zoom: Number(event.target.value) })} /></label>
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
                {finalAsset && <div className="movie-final-meta"><strong>{finalAsset.display_name}</strong><span>{finalAsset.width || edit.output_width}x{finalAsset.height || edit.output_height}</span><span>{finalAsset.size_bytes ? `${Math.round(finalAsset.size_bytes / 1024)} KB` : 'Stored movie asset'}</span></div>}
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

function sceneAtTime(scenes: MovieScene[], time: number): MovieScene | undefined {
  if (!scenes.length) return undefined;
  return scenes.find(scene => {
    const start = scene.start_seconds || 0;
    return time >= start && time < start + scene.duration_seconds;
  }) || scenes[scenes.length - 1];
}

function recalculateScenes(scenes: MovieScene[]): MovieScene[] {
  let start = 0;
  return scenes.map((scene, index) => {
    const next = { ...scene, position: index + 1, start_seconds: start };
    start += Math.max(0.5, Number(scene.duration_seconds) || 4);
    next.duration_seconds = Math.max(0.5, Number(scene.duration_seconds) || 4);
    return next;
  });
}

function seekScene(scenes: MovieScene[], time: number, direction: -1 | 1, setTime: (time: number) => void, setScene: (id: string) => void) {
  if (!scenes.length) return;
  const current = sceneAtTime(scenes, time);
  const idx = Math.max(0, scenes.findIndex(scene => scene === current || scene.id === current?.id));
  const next = scenes[Math.min(scenes.length - 1, Math.max(0, idx + direction))];
  setTime(next.start_seconds || 0);
  setScene(next.id || '');
}

function buildPreflight(edit: MovieEdit | null, assets: MediaAsset[]): { errors: string[]; warnings: string[] } {
  const errors: string[] = [];
  const warnings: string[] = [];
  if (!edit) return { errors: ['Open or create a movie edit.'], warnings };
  if (!edit.scenes.length) errors.push('Add at least one scene.');
  const assetIDs = new Set(assets.map(asset => asset.id));
  edit.scenes.forEach((scene, index) => {
    if (!scene.visual_asset_id) errors.push(`Scene ${index + 1} needs a visual.`);
    if (scene.visual_asset_id && !assetIDs.has(scene.visual_asset_id)) errors.push(`Scene ${index + 1} visual is missing from the Asset Library.`);
    if (scene.duration_seconds <= 0) errors.push(`Scene ${index + 1} needs a positive duration.`);
    if (scene.trim_in_seconds !== undefined && scene.trim_in_seconds < 0) errors.push(`Scene ${index + 1} trim start is invalid.`);
    if (scene.trim_out_seconds !== undefined && scene.trim_in_seconds !== undefined && scene.trim_out_seconds <= scene.trim_in_seconds) errors.push(`Scene ${index + 1} trim end must be after trim start.`);
  });
  if (edit.voiceover_asset_id && !assetIDs.has(edit.voiceover_asset_id)) errors.push('Selected narration is missing.');
  if (!edit.voiceover_asset_id) warnings.push('No narration selected.');
  if (edit.music_asset_id && !assetIDs.has(edit.music_asset_id)) errors.push('Selected music is missing.');
  if ((edit.scenes.reduce((sum, scene) => sum + scene.duration_seconds, 0)) > 180) warnings.push('Long edits may take more time to render.');
  return { errors, warnings: warnings.length ? warnings : ['All required media is available.'] };
}

function friendlyMovieError(error: unknown, fallback: string): string {
  const raw = error instanceof Error ? error.message : '';
  if (!raw || /^HTTP\s+\d+/i.test(raw) || raw.includes('Cannot reach')) return fallback;
  return raw.replace(/\bffmpeg\b/gi, 'video processing');
}
