import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  apiUrl,
  createVoiceGeneration,
  createVoicePreview,
  listContentProjectScenes,
  listContentProjects,
  listVoices,
  setActiveProjectVoiceover,
  uploadVoiceover,
  type ContentProject,
  type ContentProjectScene,
  type VoiceGeneration,
  type VoiceOption,
} from '../lib/api/client';
import type { View } from '../types';

interface VoiceStudioPageProps {
  onNavigate: (view: View, projectID?: string) => void;
}

const PRESETS = ['natural', 'energetic', 'calm', 'documentary', 'conversational', 'promotional', 'dramatic'];

export function VoiceStudioPage({ onNavigate }: VoiceStudioPageProps) {
  const [voices, setVoices] = useState<VoiceOption[]>([]);
  const [projects, setProjects] = useState<ContentProject[]>([]);
  const [scenes, setScenes] = useState<ContentProjectScene[]>([]);
  const [projectID, setProjectID] = useState(() => new URLSearchParams(window.location.search).get('project_id') || '');
  const [source, setSource] = useState<'blank' | 'project_script' | 'all_scenes' | 'scene'>('blank');
  const [sceneID, setSceneID] = useState('');
  const [text, setText] = useState('');
  const [voiceID, setVoiceID] = useState('marin');
  const [speed, setSpeed] = useState(1);
  const [preset, setPreset] = useState('natural');
  const [format, setFormat] = useState('mp3');
  const [customInstructions, setCustomInstructions] = useState('');
  const [mode, setMode] = useState<'full' | 'scene'>('full');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const [result, setResult] = useState<VoiceGeneration | null>(null);
  const [sceneResults, setSceneResults] = useState<VoiceGeneration[]>([]);
  const [previewing, setPreviewing] = useState<string | null>(null);
  const [previewCache, setPreviewCache] = useState<Record<string, VoiceGeneration>>({});
  const [uploading, setUploading] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([listVoices(), listContentProjects()])
      .then(([voiceResp, projectResp]) => {
        if (cancelled) return;
        setVoices(voiceResp.voices);
        setProjects(projectResp.projects);
        if (!voiceResp.voices.some(v => v.id === voiceID) && voiceResp.voices[0]) setVoiceID(voiceResp.voices[0].id);
      })
      .catch(err => { if (!cancelled) setError(err instanceof Error ? err.message : 'Voice Studio is unavailable.'); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (!projectID) {
      setScenes([]);
      return;
    }
    let cancelled = false;
    listContentProjectScenes(projectID)
      .then(resp => { if (!cancelled) setScenes(resp.scenes); })
      .catch(() => { if (!cancelled) setScenes([]); });
    return () => { cancelled = true; };
  }, [projectID]);

  const selectedProject = projects.find(project => project.id === projectID);
  const selectedScene = scenes.find(scene => scene.id === sceneID);

  useEffect(() => {
    if (source === 'project_script') setText(selectedProject?.main_script || '');
    if (source === 'all_scenes') setText(scenes.map(scene => scene.spoken_text).filter(Boolean).join('\n\n'));
    if (source === 'scene') setText(selectedScene?.spoken_text || '');
  }, [source, selectedProject?.main_script, scenes, selectedScene?.spoken_text]);

  const charCount = text.trim().length;
  const estimatedSeconds = useMemo(() => estimateDuration(text, speed), [speed, text]);
  const selectedVoice = voices.find(voice => voice.id === voiceID);
  const canGenerate = charCount > 0 && charCount <= 12000 && Boolean(voiceID) && !generating;

  const payload = useCallback((overrideVoice?: string) => ({
    project_id: projectID || undefined,
    scene_id: source === 'scene' ? sceneID || undefined : undefined,
    source_type: source,
    text,
    mode,
    voice_id: overrideVoice || voiceID,
    speed,
    delivery_preset: preset,
    custom_instructions: customInstructions,
    output_format: format,
    idempotency_key: `${projectID || 'workspace'}-${source}-${mode}-${voiceID}-${Date.now()}`,
  }), [customInstructions, format, mode, preset, projectID, sceneID, source, speed, text, voiceID]);

  async function playPreview(voice: VoiceOption) {
    setError(null);
    audioRef.current?.pause();
    const excerpt = text.trim().slice(0, 220);
    const key = `${voice.id}:${speed}:${preset}:${format}:${excerpt || 'default'}`;
    try {
      setPreviewing(voice.id);
      const cached = previewCache[key] || (await createVoicePreview({ ...payload(voice.id), text: excerpt || 'Here is a short preview of this narration voice for your next video.', mode: 'full' })).generation;
      setPreviewCache(current => ({ ...current, [key]: cached }));
      const url = cached.asset?.preview_url || cached.asset?.download_url;
      if (!url) throw new Error('Preview audio was not saved.');
      const audio = new Audio(apiUrl(url));
      audioRef.current = audio;
      audio.onended = () => setPreviewing(null);
      audio.onerror = () => { setPreviewing(null); setError('This preview could not be played in the browser.'); };
      await audio.play();
    } catch (err) {
      setPreviewing(null);
      setError(err instanceof Error ? err.message : 'Preview failed.');
    }
  }

  async function generate() {
    if (!canGenerate) return;
    setError(null);
    setGenerating(true);
    setSceneResults([]);
    try {
      const resp = await createVoiceGeneration(payload());
      setResult(resp.generation);
      setSceneResults(resp.scene_generations ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Generation failed.');
    } finally {
      setGenerating(false);
    }
  }

  async function upload(file?: File | null) {
    if (!file) return;
    setError(null);
    setUploading(true);
    try {
      const resp = await uploadVoiceover(file, projectID || undefined);
      setResult(resp.generation);
      setSceneResults([]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed.');
    } finally {
      setUploading(false);
    }
  }

  async function makeActive(gen: VoiceGeneration) {
    if (!projectID || !gen.asset_id) return;
    await setActiveProjectVoiceover(projectID, gen.asset_id);
    setResult({ ...gen });
  }

  return (
    <div className="voice-studio-page">
      <section className="voice-studio-hero">
        <div>
          <div className="page-eyebrow">CONTENT</div>
          <h1>Voice Studio</h1>
          <p>Create narration from scripts or scenes, compare voices, save audio to the Asset Library, and attach the selected take to a project.</p>
        </div>
        <button className="generate-btn secondary" type="button" onClick={() => onNavigate('assets')}>Open Asset Library</button>
      </section>

      {loading && <div className="voice-state">Loading Voice Studio...</div>}
      {error && <div className="confirmation-error" role="alert">{error}</div>}

      {!loading && (
        <section className="voice-workspace">
          <div className="voice-editor-panel">
            <div className="voice-section-title">
              <h2>Source</h2>
              <span>{charCount.toLocaleString()} characters · {formatDuration(estimatedSeconds)}</span>
            </div>
            <div className="voice-source-grid">
              <label>
                <span>Project</span>
                <select value={projectID} onChange={event => { setProjectID(event.target.value); setSceneID(''); }}>
                  <option value="">No project</option>
                  {projects.map(project => <option key={project.id} value={project.id}>{project.title}</option>)}
                </select>
              </label>
              <label>
                <span>Start from</span>
                <select value={source} onChange={event => setSource(event.target.value as typeof source)}>
                  <option value="blank">Blank text</option>
                  <option value="project_script" disabled={!selectedProject?.main_script}>Project script</option>
                  <option value="all_scenes" disabled={scenes.length === 0}>All scenes</option>
                  <option value="scene" disabled={scenes.length === 0}>Specific scene</option>
                </select>
              </label>
              {source === 'scene' && (
                <label>
                  <span>Scene</span>
                  <select value={sceneID} onChange={event => setSceneID(event.target.value)}>
                    <option value="">Choose scene</option>
                    {scenes.map(scene => <option key={scene.id} value={scene.id}>{scene.position}. {scene.title || 'Untitled scene'}</option>)}
                  </select>
                </label>
              )}
            </div>
            <label className="voice-textarea-label">
              <span>Narration text</span>
              <textarea value={text} onChange={event => setText(event.target.value)} placeholder="Paste or write the narration you want to generate." maxLength={12000} />
            </label>
            <div className="voice-mode-row" role="radiogroup" aria-label="Generation mode">
              <button className={mode === 'full' ? 'active' : ''} type="button" onClick={() => setMode('full')}>Full narration</button>
              <button className={mode === 'scene' ? 'active' : ''} type="button" onClick={() => setMode('scene')} disabled={!projectID || scenes.length === 0}>Scene by scene</button>
            </div>
          </div>

          <aside className="voice-control-panel">
            <div className="voice-section-title"><h2>Voice</h2><span>{selectedVoice?.display_name}</span></div>
            <div className="voice-browser">
              {voices.map(voice => (
                <article key={voice.id} className={`voice-card${voiceID === voice.id ? ' active' : ''}`}>
                  <button className="voice-card-main" type="button" onClick={() => setVoiceID(voice.id)}>
                  <strong>{voice.display_name}</strong>
                  <span>{voice.character}</span>
                  <small>{voice.language_note}</small>
                  <em>{voice.best_for.slice(0, 3).join(' · ')}</em>
                  </button>
                  <button className="voice-preview-btn" type="button" onClick={() => playPreview(voice)}>{previewing === voice.id ? 'Playing' : 'Preview'}</button>
                </article>
              ))}
            </div>
            <label className="voice-field">
              <span>Speaking speed</span>
              <input type="range" min="0.75" max="1.5" step="0.05" value={speed} onChange={event => setSpeed(Number(event.target.value))} />
              <b>{speed.toFixed(2)}x</b>
            </label>
            <label className="voice-field">
              <span>Delivery</span>
              <select value={preset} onChange={event => setPreset(event.target.value)}>
                {PRESETS.map(item => <option key={item} value={item}>{titleCase(item)}</option>)}
              </select>
            </label>
            <details className="voice-advanced" open={advancedOpen} onToggle={event => setAdvancedOpen(event.currentTarget.open)}>
              <summary>Advanced</summary>
              <label className="voice-field">
                <span>Format</span>
                <select value={format} onChange={event => setFormat(event.target.value)}>
                  <option value="mp3">MP3</option>
                  <option value="wav">WAV</option>
                  <option value="aac">AAC</option>
                  <option value="opus">Opus</option>
                  <option value="flac">FLAC</option>
                </select>
              </label>
              <label className="voice-field">
                <span>Pronunciation or delivery notes</span>
                <textarea value={customInstructions} onChange={event => setCustomInstructions(event.target.value)} maxLength={600} placeholder="Optional notes for names, pacing, or tone." />
              </label>
            </details>
            <button className="generate-btn idle" type="button" disabled={!canGenerate} onClick={generate}>{generating ? 'Generating...' : 'Generate voiceover'}</button>
            <label className="voice-upload">
              <span>{uploading ? 'Uploading...' : 'Upload existing narration'}</span>
              <input type="file" accept="audio/*,.mp3,.wav,.m4a,.aac,.flac,.ogg,.opus" disabled={uploading} onChange={event => upload(event.target.files?.[0])} />
            </label>
          </aside>
        </section>
      )}

      {(result || sceneResults.length > 0) && (
        <section className="voice-results">
          <div className="voice-section-title"><h2>Saved voiceovers</h2><span>{sceneResults.length ? `${sceneResults.length} scenes` : result?.status}</span></div>
          {result && !sceneResults.length && <VoiceResult generation={result} projectID={projectID} onMakeActive={() => makeActive(result)} onNavigate={onNavigate} />}
          {sceneResults.map(gen => <VoiceResult key={gen.id || gen.scene_id} generation={gen} projectID={projectID} onMakeActive={() => makeActive(gen)} onNavigate={onNavigate} />)}
        </section>
      )}
    </div>
  );
}

function VoiceResult({ generation, projectID, onMakeActive, onNavigate }: { generation: VoiceGeneration; projectID: string; onMakeActive: () => void; onNavigate: (view: View, projectID?: string) => void }) {
  const asset = generation.asset;
  return (
    <article className="voice-result-card">
      <div>
        <strong>{asset?.display_name || generation.voice_display_name || 'Voiceover'}</strong>
        <span>{generation.status} · {generation.output_format?.toUpperCase()} · {formatDuration(generation.generated_duration_seconds || generation.estimated_duration_seconds || 0)}</span>
      </div>
      {asset?.preview_url && <audio controls preload="metadata" src={apiUrl(asset.preview_url)} />}
      <div className="voice-result-actions">
        {asset?.download_url && <a className="generate-btn secondary" href={apiUrl(asset.download_url)}>Download</a>}
        {projectID && asset?.id && <button className="generate-btn secondary" type="button" onClick={onMakeActive}>Set active</button>}
        {projectID && <button className="generate-btn secondary" type="button" onClick={() => onNavigate('scriptStudio', projectID)}>Return to project</button>}
        <button className="generate-btn secondary" type="button" onClick={() => onNavigate('assets')}>Asset Library</button>
      </div>
    </article>
  );
}

function estimateDuration(value: string, speed: number): number {
  const words = value.trim().split(/\s+/).filter(Boolean).length;
  return words ? (words / 155) * 60 / Math.max(speed, 0.1) : 0;
}

function formatDuration(seconds: number): string {
  if (!seconds) return '0s';
  const min = Math.floor(seconds / 60);
  const sec = Math.round(seconds % 60).toString().padStart(2, '0');
  return min ? `${min}:${sec}` : `${Math.round(seconds)}s`;
}

function titleCase(value: string): string {
  return value.slice(0, 1).toUpperCase() + value.slice(1);
}
