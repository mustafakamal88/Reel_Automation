import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import {
  ApiError,
  createContentProjectScene,
  deleteContentProjectScene,
  generateContentProjectScenes,
  getContentProject,
  importLegacyContentProject,
  listContentProjectScenes,
  reorderContentProjectScenes,
  updateContentProjectScene,
  updateContentProject,
  type ContentProject,
  type ContentProjectScene,
  type ContentProjectScenePayload,
  type ContentProjectPayload,
  type ReelContentPackage,
  type TrendCandidate,
} from '../lib/api/client';
import type { ScenePlanDraft, StoredScriptPackage } from '../lib/storage';
import { storage } from '../lib/storage';
import { ConfirmationDialog } from '../components/ConfirmationDialog';

interface Props {
  latestScript: StoredScriptPackage | null;
  onUseInClipGenerator?: () => void;
  onGoToTrendFinder?: () => void;
}

type ScriptSectionID = 'hook' | 'script' | 'caption' | 'hashtags';
type SaveState = 'saved' | 'dirty' | 'saving' | 'failed';
type WorkspaceTab = 'script' | 'scenes';
type SceneConfirmation =
  | { type: 'replace-scene-plan'; sceneCount: number }
  | { type: 'delete-scene'; sceneID: string; label: string };

interface SceneDraft extends ContentProjectScene {
  localOnly?: boolean;
}

type EvidenceRecord = Record<string, unknown>;

interface ScriptDraft {
  hook: string;
  mainScript: string;
  caption: string;
  hashtags: string;
  platformText: Record<string, string>;
}

interface EvidenceViewModel {
  sourceSummary: string[];
  sources: string[];
  performanceSignals: Array<{ label: string; value: string }>;
  relatedVideos: Array<{ title: string; channel?: string; views?: string; published?: string; url?: string }>;
  relatedChannels: string[];
  keywords: string[];
  limitations: string[];
  advanced: Array<{ label: string; value: string }>;
}

const sectionLabels: Record<ScriptSectionID, string> = {
  hook: 'Hook',
  script: 'Script',
  caption: 'Caption',
  hashtags: 'Hashtags',
};

const platformEditors = [
  { key: 'instagram', label: 'Instagram', rows: 4 },
  { key: 'tiktok', label: 'TikTok', rows: 4 },
  { key: 'youtube', label: 'YouTube', rows: 6 },
  { key: 'facebook', label: 'Facebook', rows: 4 },
  { key: 'x', label: 'X', rows: 3 },
  { key: 'threads', label: 'Threads', rows: 4 },
];

async function copyText(value: string) {
  if (!value) return;
  try {
    await navigator.clipboard?.writeText(value);
  } catch {
    // Clipboard access can be blocked by browser permissions; keep the UI stable.
  }
}

function errMsg(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return fallback;
}

function sourceLabel(source?: string): string {
  switch (source) {
    case 'youtube_video_analysis':
      return 'YouTube Video Analysis';
    case 'youtube_channel_analysis':
      return 'YouTube Channel Analysis';
    case 'google_trend':
    case 'google_trends_rss':
      return 'Keyword Search';
    case 'niche_idea':
      return 'Niche Idea';
    default:
      return source?.replaceAll('_', ' ') || 'Research';
  }
}

function formatDate(value?: string): string {
  if (!value) return 'just now';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function toRecord(value: unknown): EvidenceRecord | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as EvidenceRecord : null;
}

function parseJSONRecord(value?: string): EvidenceRecord | null {
  if (!value) return null;
  try {
    return toRecord(JSON.parse(value));
  } catch {
    return null;
  }
}

function textList(value: unknown): string[] {
  if (!value) return [];
  if (Array.isArray(value)) {
    return value.flatMap(item => {
      if (typeof item === 'string' || typeof item === 'number') return [String(item)];
      const record = toRecord(item);
      if (record?.title && typeof record.title === 'string') return [record.title];
      if (record?.name && typeof record.name === 'string') return [record.name];
      return [];
    }).map(item => item.trim()).filter(Boolean);
  }
  if (typeof value === 'string' || typeof value === 'number') return [String(value)].filter(Boolean);
  return [];
}

function firstString(...values: unknown[]): string | undefined {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) return value.trim();
    if (typeof value === 'number') return String(value);
  }
  return undefined;
}

function formatValue(value: unknown): string {
  if (value == null || value === '') return 'Not available';
  if (typeof value === 'number') return Number.isInteger(value) ? value.toLocaleString() : value.toFixed(2);
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return textList(value).join(', ') || `${value.length} items`;
  const record = toRecord(value);
  if (!record) return String(value);
  return Object.entries(record)
    .map(([key, item]) => `${humanizeKey(key)}: ${formatValue(item)}`)
    .join(' · ');
}

function humanizeKey(key: string): string {
  return key
    .replace(/[_-]+/g, ' ')
    .replace(/\b\w/g, char => char.toUpperCase());
}

function formatViews(value: unknown): string | undefined {
  if (typeof value === 'number') return value.toLocaleString();
  if (typeof value === 'string' && value.trim()) return value.trim();
  return undefined;
}

function scoreValue(value: unknown): string {
  if (typeof value !== 'number') return formatValue(value);
  const rounded = Math.round(value);
  if (value >= 0 && value <= 1) return `${Math.round(value * 100)}%`;
  if (rounded >= 0 && rounded <= 100) return `${rounded}/100`;
  return value.toLocaleString();
}

function collectRelatedVideos(evidence: EvidenceRecord): EvidenceViewModel['relatedVideos'] {
  const nested = toRecord(evidence.evidence);
  const videos = [
    evidence.related_videos,
    nested?.related_videos,
    nested?.top_videos_summary,
  ].find(Array.isArray);

  if (!Array.isArray(videos)) return [];
  return videos.map(item => {
    if (typeof item === 'string') return { title: item };
    const record = toRecord(item);
    if (!record) return { title: '' };
    return {
      title: firstString(record.title, record.video_title, record.name) || 'Related video',
      channel: firstString(record.channel, record.channel_title, record.channel_name),
      views: formatViews(record.views ?? record.view_count),
      published: firstString(record.published_at, record.published, record.date),
      url: firstString(record.url, record.video_url, record.source_url),
    };
  }).filter(video => video.title);
}

function buildEvidenceViewModel(rawGrounding?: string, safetyNotes: string[] = []): EvidenceViewModel | null {
  const parsed = parseJSONRecord(rawGrounding);
  const sourceSummary = textList(parsed?.summary ?? rawGrounding).slice(0, 3);
  const nestedEvidence = toRecord(parsed?.evidence);
  const metadata = toRecord(parsed?.metadata);
  const performance = toRecord(parsed?.performance_signals);
  const performanceMap: Array<[string, string]> = [
    ['demand_score', 'Demand score'],
    ['monetization_score', 'Monetization score'],
    ['competition_level', 'Competition level'],
    ['competition_score', 'Competition level'],
    ['opportunity_score', 'Opportunity score'],
    ['success_probability', 'Success probability'],
    ['success_probability_score', 'Success probability'],
  ];
  const performanceSignals = performanceMap
    .filter(([key]) => performance?.[key] != null)
    .map(([key, label]) => ({ label, value: key.includes('score') ? scoreValue(performance?.[key]) : formatValue(performance?.[key]) }));

  const sources = [
    ...textList(nestedEvidence?.evidence_sources),
    ...textList(parsed?.source_url),
    ...textList(metadata?.source_url),
  ];
  const keywords = [
    ...textList(parsed?.keywords),
    ...textList(nestedEvidence?.primary_keywords),
    ...textList(nestedEvidence?.supporting_keywords),
  ];
  const limitations = [
    ...textList(parsed?.limitations),
    ...safetyNotes,
  ];
  const relatedChannels = [
    ...textList(nestedEvidence?.related_channels),
    ...textList(nestedEvidence?.channel_title),
  ];

  const knownKeys = new Set([
    'source_type',
    'source_id',
    'source_url',
    'topic',
    'title',
    'summary',
    'keywords',
    'inferred_niche',
    'inferred_angle',
    'suggested_angle',
    'performance_signals',
    'evidence',
    'metadata',
    'limitations',
  ]);
  const advanced = parsed
    ? Object.entries(parsed)
      .filter(([key, value]) => !knownKeys.has(key) && value != null && value !== '')
      .map(([key, value]) => ({ label: humanizeKey(key), value: formatValue(value) }))
    : [];

  if (!parsed && !rawGrounding && limitations.length === 0) return null;
  return {
    sourceSummary: sourceSummary.length ? sourceSummary : ['Generated from the selected research context.'],
    sources: Array.from(new Set(sources)).filter(Boolean),
    performanceSignals,
    relatedVideos: parsed ? collectRelatedVideos(parsed) : [],
    relatedChannels: Array.from(new Set(relatedChannels)).filter(Boolean),
    keywords: Array.from(new Set(keywords)).filter(Boolean).slice(0, 14),
    limitations: Array.from(new Set([
      ...limitations,
      'Public metadata only. Exact RPM/search ranking requires authorized analytics.',
    ])).filter(Boolean),
    advanced,
  };
}

function ScriptCard({ id, title, value, onCopy, highlighted }: {
  id: string;
  title: string;
  value?: string;
  onCopy: () => void;
  highlighted?: boolean;
}) {
  if (!value) return null;
  return (
    <article id={id} className={`script-card${highlighted ? ' script-card-highlight' : ''}`}>
      <div className="script-card-header">
        <h2>{title}</h2>
        <button className="mini-copy-btn" type="button" onClick={onCopy}>Copy</button>
      </div>
      <div className="script-card-body">{value}</div>
    </article>
  );
}

function PlatformCard({ label, value, subValue }: { label: string; value?: string; subValue?: string }) {
  if (!value && !subValue) return null;
  const copyValue = [value, subValue].filter(Boolean).join('\n\n');
  return (
    <article className="platform-copy-card">
      <div className="script-card-header">
        <h3>{label}</h3>
        <button className="mini-copy-btn" type="button" onClick={() => void copyText(copyValue)}>Copy</button>
      </div>
      {value && <p>{value}</p>}
      {subValue && <p>{subValue}</p>}
    </article>
  );
}

function EditorField({ id, label, value, rows, maxLength, hint, onChange, onCopy }: {
  id: string;
  label: string;
  value: string;
  rows: number;
  maxLength: number;
  hint?: string;
  onChange: (value: string) => void;
  onCopy: () => void;
}) {
  const inputID = `${id}-input`;
  return (
    <div id={id} className="script-editor-field">
      <span className="script-editor-label-row">
        <label htmlFor={inputID}>{label}</label>
        <button className="mini-copy-btn" type="button" onClick={onCopy}>Copy</button>
      </span>
      <textarea
        id={inputID}
        className="form-textarea script-editor-textarea"
        value={value}
        rows={rows}
        maxLength={maxLength}
        onChange={event => onChange(event.target.value)}
      />
      <span className="script-editor-hint">{hint || `${value.length.toLocaleString()}/${maxLength.toLocaleString()} characters`}</span>
    </div>
  );
}

function StatusRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="script-context-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function EvidencePanel({ evidence }: { evidence: EvidenceViewModel | null }) {
  if (!evidence) return null;
  return (
    <section className="evidence-panel" aria-label="Evidence and grounding">
      <div className="script-section-heading">
        <span>Evidence</span>
        <strong>Grounded from public research context</strong>
      </div>

      <div className="evidence-grid">
        <EvidenceSection title="Source summary" items={evidence.sourceSummary} />
        <EvidenceSection title="Evidence sources" items={evidence.sources} linkItems />
      </div>

      {evidence.performanceSignals.length > 0 && (
        <div className="evidence-block">
          <h3>Performance indicators</h3>
          <div className="evidence-pill-grid">
            {evidence.performanceSignals.map(indicator => (
              <div className="evidence-pill" key={indicator.label}>
                <span>{indicator.label}</span>
                <strong>{indicator.value}</strong>
              </div>
            ))}
          </div>
        </div>
      )}

      {evidence.relatedVideos.length > 0 && (
        <div className="evidence-block">
          <h3>Related videos</h3>
          <div className="related-video-list">
            {evidence.relatedVideos.map((video, index) => (
              <div className="related-video-row" key={`${video.title}-${index}`}>
                <div>
                  <strong>{video.title}</strong>
                  <span>{[video.channel, video.views ? `${video.views} views` : '', video.published ? formatDate(video.published) : ''].filter(Boolean).join(' · ')}</span>
                </div>
                {video.url && <a href={video.url} target="_blank" rel="noreferrer">Open</a>}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="evidence-grid">
        <EvidenceSection title="Related channels" items={evidence.relatedChannels} />
        <EvidenceSection title="Keywords" items={evidence.keywords} chips />
      </div>

      <EvidenceSection title="Limitations" items={evidence.limitations} soft />

      {evidence.advanced.length > 0 && (
        <details className="advanced-evidence">
          <summary>Advanced evidence data</summary>
          <div className="advanced-evidence-list">
            {evidence.advanced.map(item => (
              <div key={item.label}>
                <span>{item.label}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
          </div>
        </details>
      )}
    </section>
  );
}

function EvidenceSection({ title, items, chips, linkItems, soft }: {
  title: string;
  items: string[];
  chips?: boolean;
  linkItems?: boolean;
  soft?: boolean;
}) {
  if (items.length === 0) return null;
  return (
    <div className={`evidence-block${soft ? ' evidence-block-soft' : ''}`}>
      <h3>{title}</h3>
      {chips ? (
        <div className="script-badge-row">
          {items.map(item => <span className="script-badge" key={item}>{item}</span>)}
        </div>
      ) : (
        <ul>
          {items.map(item => (
            <li key={item}>
              {linkItems && item.startsWith('http') ? <a href={item} target="_blank" rel="noreferrer">{item}</a> : item}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function currentProjectID(): string {
  return new URLSearchParams(window.location.search).get('project_id')?.trim() || '';
}

function projectToStoredScript(project: ContentProject): StoredScriptPackage {
  const platformText = project.platform_text || {};
  const pkg: ReelContentPackage = {
    title: project.title,
    hook: project.hook || '',
    script: project.main_script || '',
    caption: project.caption || '',
    description: platformText.youtube || '',
    hashtags: project.hashtags || [],
    platform_posts: platformText,
    thumbnail_brief: '',
    instagram_caption: platformText.instagram || '',
    tiktok_caption: platformText.tiktok || '',
    youtube_title: project.title,
    youtube_description: platformText.youtube || '',
    facebook_caption: platformText.facebook || '',
    x_caption: platformText.x || '',
    safety_grounding_notes: [],
    grounding: '',
    source_type: project.source_type,
    source_url: project.source_reference,
    created_at: project.created_at,
    inferred_keywords: [],
    inferred_niche: project.topic,
    inferred_angle: '',
    provider_metadata: {
      provider: 'content_project',
      model: 'project',
      source_candidate_id: project.id,
      source: project.source_type,
      source_url: project.source_reference,
      platform_targets: project.target_platforms,
      duration_target: `${project.target_duration_seconds}s`,
      generated_at: project.updated_at,
    },
  };
  const candidate: TrendCandidate = {
    id: project.id,
    source: project.source_type,
    region: '',
    language: project.language,
    keyword: project.topic,
    title: project.title,
    score: 0,
    discovered_at: project.created_at,
    source_url: project.source_reference,
    evidence: '',
    status: project.status,
  };
  return { candidate, package: pkg, savedAt: project.updated_at };
}

function draftFromProject(project: ContentProject): ScriptDraft {
  return {
    hook: project.hook || '',
    mainScript: project.main_script || '',
    caption: project.caption || '',
    hashtags: (project.hashtags || []).map(tag => tag.startsWith('#') ? tag : `#${tag}`).join(' '),
    platformText: { ...(project.platform_text || {}) },
  };
}

function normalizeHashtags(value: string): string[] {
  return Array.from(new Set(
    value
      .split(/[\s,]+/)
      .map(tag => tag.trim().replace(/^#+/, '').toLowerCase())
      .filter(Boolean),
  ));
}

function draftSignature(draft: ScriptDraft): string {
  return JSON.stringify({
    hook: draft.hook,
    mainScript: draft.mainScript,
    caption: draft.caption,
    hashtags: normalizeHashtags(draft.hashtags),
    platformText: draft.platformText,
  });
}

function payloadFromDraft(project: ContentProject, draft: ScriptDraft): ContentProjectPayload {
  return {
    title: project.title,
    topic: project.topic,
    source_type: project.source_type,
    source_reference: project.source_reference || '',
    source_label: project.source_label || '',
    status: project.status === 'idea' ? 'draft' : project.status,
    current_stage: 'script',
    target_platforms: project.target_platforms.length ? project.target_platforms : ['youtube', 'tiktok', 'instagram'],
    content_format: project.content_format || 'short_video',
    target_duration_seconds: project.target_duration_seconds || 30,
    language: project.language || 'en-US',
    creative_brief: project.creative_brief || {},
    hook: draft.hook,
    main_script: draft.mainScript,
    caption: draft.caption,
    hashtags: normalizeHashtags(draft.hashtags),
    platform_text: draft.platformText,
  };
}

function countWords(value: string): number {
  const matches = value.trim().match(/\b[\w'-]+\b/g);
  return matches ? matches.length : 0;
}

function formatDuration(seconds: number): string {
  const rounded = Math.max(0, Math.round(seconds));
  const minutes = Math.floor(rounded / 60);
  const remainder = rounded % 60;
  if (minutes <= 0) return `${remainder}s`;
  return `${minutes}m ${String(remainder).padStart(2, '0')}s`;
}

function durationTone(estimatedSeconds: number, targetSeconds: number): string {
  if (!targetSeconds) return 'neutral';
  const delta = Math.abs(estimatedSeconds - targetSeconds);
  if (delta <= Math.max(4, targetSeconds * 0.12)) return 'ready';
  if (estimatedSeconds > targetSeconds) return 'warning';
  return 'neutral';
}

function sceneSignature(scenes: SceneDraft[]): string {
  return JSON.stringify(scenes.map((scene, index) => ({
    id: scene.localOnly ? `local-${index}` : scene.id,
    title: scene.title,
    spoken_text: scene.spoken_text,
    on_screen_text: scene.on_screen_text,
    visual_direction: scene.visual_direction,
    broll_direction: scene.broll_direction,
    camera_direction: scene.camera_direction,
    transition_direction: scene.transition_direction,
    planned_duration_seconds: scene.planned_duration_seconds,
    production_notes: scene.production_notes,
  })));
}

function scenePayload(scene: ContentProjectScene, position: number): ContentProjectScenePayload {
  return {
    position,
    title: scene.title,
    spoken_text: scene.spoken_text,
    on_screen_text: scene.on_screen_text,
    visual_direction: scene.visual_direction,
    broll_direction: scene.broll_direction,
    camera_direction: scene.camera_direction,
    transition_direction: scene.transition_direction,
    planned_duration_seconds: scene.planned_duration_seconds,
    production_notes: scene.production_notes,
  };
}

function emptyLocalScene(projectID: string, position: number): SceneDraft {
  const now = new Date().toISOString();
  return {
    id: `local-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    project_id: projectID,
    position,
    title: '',
    spoken_text: '',
    on_screen_text: '',
    visual_direction: '',
    broll_direction: '',
    camera_direction: '',
    transition_direction: '',
    planned_duration_seconds: 5,
    production_notes: '',
    created_at: now,
    updated_at: now,
    localOnly: true,
  };
}

function cloneSceneForDraft(scene: ContentProjectScene, projectID: string, position: number): SceneDraft {
  const now = new Date().toISOString();
  return {
    ...scene,
    id: `local-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    project_id: projectID,
    position,
    title: scene.title ? `${scene.title} copy` : '',
    created_at: now,
    updated_at: now,
    localOnly: true,
  };
}

function renumberScenes(scenes: SceneDraft[]): SceneDraft[] {
  return scenes.map((scene, index) => ({ ...scene, position: index + 1 }));
}

function sceneDurationSummary(scenes: ContentProjectScene[], targetSeconds: number) {
  const total = scenes.reduce((sum, scene) => sum + Math.max(0, Number(scene.planned_duration_seconds) || 0), 0);
  const delta = Math.round(total - targetSeconds);
  let label = 'No target set';
  let tone = 'neutral';
  if (targetSeconds > 0) {
    const abs = Math.abs(delta);
    const threshold = Math.max(4, targetSeconds * 0.12);
    if (abs <= threshold) {
      label = 'Within target';
      tone = 'ready';
    } else if (delta > 0) {
      label = `Approximately ${formatDuration(abs)} over target`;
      tone = 'warning';
    } else {
      label = `Approximately ${formatDuration(abs)} under target`;
    }
  }
  const warnings = scenes.flatMap((scene, index) => {
    const sceneLabel = scene.title || `Scene ${index + 1}`;
    if (!scene.planned_duration_seconds) return [`${sceneLabel} has no planned duration.`];
    if (targetSeconds > 0 && scene.planned_duration_seconds > targetSeconds * 0.55 && scenes.length > 1) {
      return [`${sceneLabel} uses most of the target duration.`];
    }
    return [];
  });
  return { total, delta, label, tone, warnings };
}

function buildProductionCopy(title: string, draft: ScriptDraft): string {
  const hashtagText = normalizeHashtags(draft.hashtags).map(tag => `#${tag}`).join(' ');
  const platformText = Object.entries(draft.platformText)
    .filter(([, value]) => value.trim())
    .map(([key, value]) => `${humanizeKey(key)}:\n${value.trim()}`)
    .join('\n\n');
  return [
    `Title: ${title}`,
    draft.hook.trim() ? `Hook:\n${draft.hook.trim()}` : '',
    draft.mainScript.trim() ? `Script:\n${draft.mainScript.trim()}` : '',
    draft.caption.trim() ? `Caption:\n${draft.caption.trim()}` : '',
    hashtagText ? `Hashtags:\n${hashtagText}` : '',
    platformText ? `Platform copy:\n${platformText}` : '',
  ].filter(Boolean).join('\n\n');
}

function legacyImportPayload(latestScript: StoredScriptPackage) {
  const pkg = latestScript.package;
  const sourceType = pkg.source_type || latestScript.candidate.source || 'legacy_script_studio';
  return {
    import_key: [latestScript.candidate.id, latestScript.savedAt, pkg.title, pkg.hook].filter(Boolean).join(':'),
    title: latestScript.candidate.title || latestScript.candidate.keyword || pkg.title || 'Imported script',
    topic: pkg.inferred_niche || latestScript.candidate.keyword || pkg.title,
    source_type: sourceType,
    source_reference: pkg.source_url || pkg.provider_metadata.source_url || latestScript.candidate.source_url || '',
    source_label: sourceLabel(sourceType),
    target_platforms: normalizePlatforms(pkg.provider_metadata.platform_targets),
    content_format: 'short_video',
    target_duration_seconds: durationSeconds(pkg.provider_metadata.duration_target),
    language: latestScript.candidate.language || 'en-US',
    hook: pkg.hook,
    main_script: pkg.script,
    caption: pkg.caption,
    hashtags: pkg.hashtags,
    platform_text: {
      instagram: pkg.instagram_caption,
      tiktok: pkg.tiktok_caption,
      youtube: [pkg.youtube_title, pkg.youtube_description].filter(Boolean).join('\n\n'),
      facebook: pkg.facebook_caption,
      x: pkg.x_caption,
      ...(pkg.platform_posts || {}),
    },
    saved_at: latestScript.savedAt,
  };
}

function normalizePlatforms(values: string[]): string[] {
  const mapped = values.map(value => {
    const lower = value.toLowerCase();
    if (lower === 'yt') return 'youtube';
    if (lower === 'tt') return 'tiktok';
    if (lower === 'ig') return 'instagram';
    if (lower === 'fb') return 'facebook';
    return lower;
  }).filter(value => ['youtube', 'tiktok', 'instagram', 'facebook', 'x', 'threads'].includes(value));
  return mapped.length ? Array.from(new Set(mapped)) : ['youtube', 'tiktok', 'instagram'];
}

function durationSeconds(value?: string): number {
  const match = String(value || '').match(/\d+/);
  if (!match) return 30;
  return Math.min(7200, Math.max(5, Number(match[0])));
}

export function ScriptStudioPage({ latestScript, onUseInClipGenerator, onGoToTrendFinder }: Props) {
  const highlightTimer = useRef<number | null>(null);
  const savedSignature = useRef('');
  const projectID = currentProjectID();
  const [project, setProject] = useState<ContentProject | null>(null);
  const [draft, setDraft] = useState<ScriptDraft | null>(null);
  const [projectLoading, setProjectLoading] = useState(Boolean(projectID));
  const [projectError, setProjectError] = useState<string | null>(null);
  const [saveState, setSaveState] = useState<SaveState>('saved');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [activeWorkspace, setActiveWorkspace] = useState<WorkspaceTab>('script');
  const [scenes, setScenes] = useState<SceneDraft[]>([]);
  const [deletedSceneIDs, setDeletedSceneIDs] = useState<string[]>([]);
  const [scenesLoading, setScenesLoading] = useState(false);
  const [sceneSaveState, setSceneSaveState] = useState<SaveState>('saved');
  const [sceneSaveError, setSceneSaveError] = useState<string | null>(null);
  const [sceneGenerateBusy, setSceneGenerateBusy] = useState(false);
  const [sceneGenerateError, setSceneGenerateError] = useState<string | null>(null);
  const [sceneConfirmation, setSceneConfirmation] = useState<SceneConfirmation | null>(null);
  const [sceneConfirmationError, setSceneConfirmationError] = useState<string | null>(null);
  const [sceneRecoveryPrompt, setSceneRecoveryPrompt] = useState<ScenePlanDraft | null>(null);
  const [importBusy, setImportBusy] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const savedSceneSignature = useRef('');

  useEffect(() => {
    if (!projectID) return;
    let cancelled = false;
    setProjectLoading(true);
    setProjectError(null);
    getContentProject(projectID)
      .then(result => {
        if (!cancelled) {
          const nextDraft = draftFromProject(result);
          savedSignature.current = draftSignature(nextDraft);
          setProject(result);
          setDraft(nextDraft);
          setSaveState('saved');
          setSaveError(null);
        }
      })
      .catch(err => {
        if (!cancelled) setProjectError(errMsg(err, 'Content project could not be loaded.'));
      })
      .finally(() => {
        if (!cancelled) setProjectLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [projectID]);

  useEffect(() => {
    if (!projectID) return;
    let cancelled = false;
    setScenesLoading(true);
    listContentProjectScenes(projectID)
      .then(result => {
        if (cancelled) return;
        const ordered = renumberScenes(result.scenes.map(scene => ({ ...scene, localOnly: false })));
        const signature = sceneSignature(ordered);
        savedSceneSignature.current = signature;
        setScenes(ordered);
        setDeletedSceneIDs([]);
        setSceneSaveState('saved');
        setSceneSaveError(null);
        const localDraft = storage.getScenePlanDraft(projectID);
        if (localDraft && localDraft.savedSceneSignature === signature && localDraft.scenes.length > 0 && sceneSignature(localDraft.scenes) !== signature) {
          setSceneRecoveryPrompt(localDraft);
        } else {
          setSceneRecoveryPrompt(null);
        }
      })
      .catch(err => {
        if (!cancelled) {
          setSceneSaveError(errMsg(err, 'Scene plan could not be loaded.'));
          setSceneSaveState('failed');
        }
      })
      .finally(() => {
        if (!cancelled) setScenesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [projectID]);

  const draftStats = useMemo(() => {
    const body = draft ? [draft.hook, draft.mainScript, draft.caption].filter(Boolean).join('\n\n') : '';
    const words = countWords(body);
    const estimatedSeconds = words / 2.45;
    const targetSeconds = project?.target_duration_seconds || 0;
    return {
      words,
      estimatedSeconds,
      targetSeconds,
      tone: durationTone(estimatedSeconds, targetSeconds),
    };
  }, [draft, project?.target_duration_seconds]);

  const productionCopy = useMemo(() => {
    if (!project || !draft) return '';
    return buildProductionCopy(project.title, draft);
  }, [draft, project]);

  function updateDraft(updater: (current: ScriptDraft) => ScriptDraft) {
    setDraft(current => {
      if (!current) return current;
      const next = updater(current);
      setSaveError(null);
      setSaveState(draftSignature(next) === savedSignature.current ? 'saved' : 'dirty');
      return next;
    });
  }

  function updateScenes(updater: (current: SceneDraft[]) => SceneDraft[]) {
    setScenes(current => {
      const next = renumberScenes(updater(current));
      setSceneSaveError(null);
      setSceneGenerateError(null);
      const nextSignature = sceneSignature(next);
      setSceneSaveState(nextSignature === savedSceneSignature.current && deletedSceneIDs.length === 0 ? 'saved' : 'dirty');
      if (projectID) {
        storage.setScenePlanDraft(projectID, {
          projectId: projectID,
          scenes: next,
          savedSceneSignature: savedSceneSignature.current,
          updatedAt: new Date().toISOString(),
        });
      }
      return next;
    });
  }

  async function saveProjectDraft(nextDraft = draft): Promise<ContentProject | null> {
    if (!project || !nextDraft) return null;
    setSaveState('saving');
    setSaveError(null);
    try {
      const saved = await updateContentProject(project.id, payloadFromDraft(project, nextDraft));
      const cleanDraft = draftFromProject(saved);
      savedSignature.current = draftSignature(cleanDraft);
      setProject(saved);
      setDraft(cleanDraft);
      setSaveState('saved');
      return saved;
    } catch (err) {
      setSaveError(errMsg(err, 'Save failed. Your unsaved edits are still visible.'));
      setSaveState('failed');
      return null;
    }
  }

  async function saveScenePlan(nextScenes = scenes): Promise<ContentProjectScene[] | null> {
    if (!project) return null;
    setSceneSaveState('saving');
    setSceneSaveError(null);
    try {
      for (const sceneID of deletedSceneIDs) {
        await deleteContentProjectScene(project.id, sceneID);
      }
      const savedScenes: ContentProjectScene[] = [];
      for (let index = 0; index < nextScenes.length; index += 1) {
        const scene = nextScenes[index];
        const payload = scenePayload(scene, index + 1);
        const saved = scene.localOnly
          ? await createContentProjectScene(project.id, payload)
          : await updateContentProjectScene(project.id, scene.id, payload);
        savedScenes.push(saved);
      }
      const reordered = savedScenes.length > 0
        ? (await reorderContentProjectScenes(project.id, savedScenes.map(scene => scene.id))).scenes
        : [];
      const ordered = renumberScenes(reordered.map(scene => ({ ...scene, localOnly: false })));
      savedSceneSignature.current = sceneSignature(ordered);
      setScenes(ordered);
      setDeletedSceneIDs([]);
      setSceneSaveState('saved');
      storage.clearScenePlanDraft(project.id);
      const refreshedProject = await getContentProject(project.id);
      setProject(refreshedProject);
      return ordered;
    } catch (err) {
      setSceneSaveError(errMsg(err, 'Save failed. Your unsaved scene edits are still visible.'));
      setSceneSaveState('failed');
      return null;
    }
  }

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
        event.preventDefault();
        if (activeWorkspace === 'scenes') void saveScenePlan();
        else void saveProjectDraft();
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  });

  async function handleImportLegacy() {
    if (!latestScript) return;
    setImportBusy(true);
    setImportError(null);
    try {
      const imported = await importLegacyContentProject(legacyImportPayload(latestScript));
      window.history.pushState(null, '', `/script-studio?project_id=${encodeURIComponent(imported.id)}`);
      const importedDraft = draftFromProject(imported);
      savedSignature.current = draftSignature(importedDraft);
      setProject(imported);
      setDraft(importedDraft);
      setSaveState('saved');
      setSaveError(null);
    } catch (err) {
      setImportError(errMsg(err, 'Legacy script import failed. The original saved script is still available here.'));
    } finally {
      setImportBusy(false);
    }
  }

  if (projectLoading) {
    return (
      <section className="page-section">
        <div className="script-empty-state system-state is-empty" role="status">
          <div className="empty-title">Loading project...</div>
          <div className="empty-desc">Fetching persistent Script Lab content.</div>
        </div>
      </section>
    );
  }

  if (projectError) {
    return (
      <section className="page-section">
        <div className="script-empty-state system-state is-error" role="alert">
          <div className="empty-title">Project could not be opened.</div>
          <div className="empty-desc">{projectError}</div>
        </div>
      </section>
    );
  }

  const activeScript = project ? projectToStoredScript(project) : latestScript;

  if (!activeScript) {
    return (
      <section className="page-section">
        <div className="script-empty-state system-state is-empty" role="status">
          <div className="empty-icon" aria-hidden="true">i</div>
          <h1 className="empty-title">No script selected yet.</h1>
          <div className="empty-desc">Generate a script from Research to start.</div>
          {onGoToTrendFinder && (
            <button className="generate-btn idle" type="button" onClick={onGoToTrendFinder}>Go to Research</button>
          )}
        </div>
      </section>
    );
  }

  if (project && draft) {
    const activeProject = project;
    const activeDraft = draft;
    const sourceURL = project.source_reference;
    const updatedLabel = formatDate(project.updated_at);
    const targetLabel = project.target_duration_seconds ? formatDuration(project.target_duration_seconds) : 'No target';
    const estimateLabel = formatDuration(draftStats.estimatedSeconds);
    const sceneStats = sceneDurationSummary(scenes, project.target_duration_seconds || 0);
    const sceneStatusLabel: Record<SaveState, string> = {
      saved: 'Scene plan saved',
      dirty: 'Scene plan has unsaved changes',
      saving: 'Saving scene plan...',
      failed: 'Scene plan save failed',
    };
    const statusLabel: Record<SaveState, string> = {
      saved: 'Saved',
      dirty: 'Unsaved changes',
      saving: 'Saving...',
      failed: 'Save failed',
    };
    const evidence = buildEvidenceViewModel(JSON.stringify(project.creative_brief || {}));

    async function submitSave(event?: FormEvent) {
      event?.preventDefault();
      await saveProjectDraft();
    }

    async function sendSavedProjectToClipGenerator() {
      const latest = saveState === 'dirty' || saveState === 'failed'
        ? await saveProjectDraft()
        : project;
      if (!latest) return;
      const confirmedScenes = activeWorkspace === 'scenes' && (sceneSaveState === 'dirty' || sceneSaveState === 'failed')
        ? await saveScenePlan()
        : scenes.filter(scene => !scene.localOnly);
      if (activeWorkspace === 'scenes' && !confirmedScenes) return;
      storage.setClipGeneratorHandoff({
        projectId: latest.id,
        title: latest.title,
        hook: latest.hook || '',
        script: latest.main_script || '',
        caption: latest.caption || '',
        platformText: latest.platform_text,
        scenes: confirmedScenes || [],
        totalPlannedDurationSeconds: (confirmedScenes || []).reduce((sum, scene) => sum + scene.planned_duration_seconds, 0),
        targetDurationSeconds: latest.target_duration_seconds,
        savedAt: latest.updated_at,
      });
      onUseInClipGenerator?.();
    }

    function addScene() {
      updateScenes(current => [...current, emptyLocalScene(activeProject.id, current.length + 1)]);
    }

    function duplicateScene(index: number) {
      updateScenes(current => {
        const copy = cloneSceneForDraft(current[index], activeProject.id, index + 2);
        return [...current.slice(0, index + 1), copy, ...current.slice(index + 1)];
      });
    }

    function requestDeleteScene(index: number) {
      const scene = scenes[index];
      if (!scene) return;
      setSceneConfirmationError(null);
      setSceneConfirmation({ type: 'delete-scene', sceneID: scene.id, label: `Scene ${index + 1}` });
    }

    function deleteScene(sceneID: string) {
      const scene = scenes.find(item => item.id === sceneID);
      if (!scene) return;
      if (!scene.localOnly) {
        setDeletedSceneIDs(current => Array.from(new Set([...current, scene.id])));
      }
      updateScenes(current => current.filter(item => item.id !== sceneID));
      setSceneConfirmation(null);
      setSceneConfirmationError(null);
    }

    function moveScene(index: number, direction: -1 | 1) {
      updateScenes(current => {
        const next = [...current];
        const target = index + direction;
        if (target < 0 || target >= next.length) return current;
        [next[index], next[target]] = [next[target], next[index]];
        return next;
      });
    }

    function requestGenerateScenePlan() {
      if (scenes.length > 0) {
        setSceneConfirmationError(null);
        setSceneGenerateError(null);
        setSceneConfirmation({ type: 'replace-scene-plan', sceneCount: scenes.length });
        return;
      }
      void generateScenePlan();
    }

    async function generateScenePlan(): Promise<boolean> {
      if (!activeDraft.mainScript.trim() && !activeDraft.hook.trim()) {
        setSceneGenerateError('Save a script before generating a scene plan.');
        return false;
      }
      const latest = saveState === 'dirty' || saveState === 'failed' ? await saveProjectDraft() : project;
      if (!latest) return false;
      setSceneGenerateBusy(true);
      setSceneGenerateError(null);
      setSceneConfirmationError(null);
      try {
        const result = await generateContentProjectScenes(latest.id, 'replace');
        const ordered = renumberScenes(result.scenes.map(scene => ({ ...scene, localOnly: false })));
        savedSceneSignature.current = sceneSignature(ordered);
        setScenes(ordered);
        setDeletedSceneIDs([]);
        setSceneSaveState('saved');
        storage.clearScenePlanDraft(latest.id);
        const refreshedProject = await getContentProject(latest.id);
        setProject(refreshedProject);
        setSceneConfirmation(null);
        return true;
      } catch (err) {
        const message = errMsg(err, 'Scene plan generation is unavailable. Existing scenes were not changed.');
        setSceneGenerateError(message);
        setSceneConfirmationError(message);
        return false;
      } finally {
        setSceneGenerateBusy(false);
      }
    }

    function closeSceneConfirmation() {
      if (sceneGenerateBusy) return;
      setSceneConfirmation(null);
      setSceneConfirmationError(null);
    }

    function confirmSceneAction() {
      if (!sceneConfirmation || sceneGenerateBusy) return;
      if (sceneConfirmation.type === 'delete-scene') {
        deleteScene(sceneConfirmation.sceneID);
        return;
      }
      void generateScenePlan();
    }

    function restoreSceneDraft() {
      if (!sceneRecoveryPrompt) return;
      setScenes(renumberScenes(sceneRecoveryPrompt.scenes.map(scene => ({ ...scene, localOnly: scene.id.startsWith('local-') }))));
      setSceneSaveState('dirty');
      setSceneRecoveryPrompt(null);
    }

    function discardSceneDraft() {
      storage.clearScenePlanDraft(activeProject.id);
      setSceneRecoveryPrompt(null);
    }

    return (
      <section className="page-section script-studio-page">
        <form className="script-shell script-lab-shell" onSubmit={event => void submitSave(event)}>
          <header className="script-hero script-lab-hero">
            <div className="script-source-line">
              <span className="script-source-badge">{sourceLabel(project.source_type)}</span>
              <span className="script-source-badge">Project-backed</span>
              <span>{project.content_format.replaceAll('_', ' ')}</span>
              <span>updated {updatedLabel}</span>
              {sourceURL && <a href={sourceURL} target="_blank" rel="noreferrer">Source evidence</a>}
            </div>
            <div className="script-lab-title-row">
              <div>
                <h1>{project.title}</h1>
                <p>{project.topic}</p>
              </div>
              <div className={`script-save-status ${saveState}`} role="status" aria-live="polite">
                <span>{statusLabel[saveState]}</span>
              </div>
            </div>
            <div className="script-badge-row">
              {project.target_platforms.map(platform => <span className="script-badge" key={platform}>{humanizeKey(platform)}</span>)}
              <span className="script-badge">{project.language}</span>
              <span className="script-badge">{humanizeKey(project.status)}</span>
            </div>
          </header>

          <nav className="script-action-bar script-lab-actions" aria-label="Script actions">
            <div className="script-workspace-tabs" role="tablist" aria-label="Script Lab workspace">
              <button className={activeWorkspace === 'script' ? 'active' : ''} type="button" role="tab" aria-selected={activeWorkspace === 'script'} onClick={() => setActiveWorkspace('script')}>Script</button>
              <button className={activeWorkspace === 'scenes' ? 'active' : ''} type="button" role="tab" aria-selected={activeWorkspace === 'scenes'} onClick={() => setActiveWorkspace('scenes')}>Scene Plan</button>
            </div>
            <button
              className="generate-btn idle"
              type="button"
              disabled={activeWorkspace === 'script' ? saveState === 'saving' || saveState === 'saved' : sceneSaveState === 'saving' || sceneSaveState === 'saved'}
              onClick={() => activeWorkspace === 'script' ? void saveProjectDraft() : void saveScenePlan()}
            >
              {activeWorkspace === 'script'
                ? (saveState === 'saving' ? 'Saving...' : 'Save')
                : (sceneSaveState === 'saving' ? 'Saving...' : 'Save Scene Plan')}
            </button>
            {activeWorkspace === 'script' && (
              <>
                <button className="generate-btn secondary" type="button" onClick={() => void copyText(productionCopy)}>Copy all</button>
                <button className="generate-btn secondary" type="button" onClick={() => void copyText(draft.hook)}>Copy hook</button>
                <button className="generate-btn secondary" type="button" onClick={() => void copyText(draft.mainScript)}>Copy script</button>
                <button className="generate-btn secondary" type="button" onClick={() => void copyText(draft.caption)}>Copy caption</button>
              </>
            )}
            {activeWorkspace === 'scenes' && (
              <>
                <button className="generate-btn secondary" type="button" onClick={requestGenerateScenePlan} disabled={sceneGenerateBusy || saveState === 'saving' || (!draft.hook.trim() && !draft.mainScript.trim())}>
                  {sceneGenerateBusy ? 'Generating...' : 'Generate Scene Plan'}
                </button>
                <button className="generate-btn secondary" type="button" onClick={addScene}>Add Scene</button>
              </>
            )}
            {onUseInClipGenerator && (
              <button className="generate-btn secondary" type="button" onClick={() => void sendSavedProjectToClipGenerator()} disabled={saveState === 'saving'}>
                Use in Clip Generator
              </button>
            )}
          </nav>

          {saveError && <div className="clip-error" role="alert">{saveError}</div>}
          {sceneSaveError && activeWorkspace === 'scenes' && <div className="clip-error" role="alert">{sceneSaveError}</div>}
          {sceneGenerateError && activeWorkspace === 'scenes' && <div className="clip-error" role="alert">{sceneGenerateError}</div>}
          {importError && <div className="clip-error" role="alert">{importError}</div>}
          {sceneRecoveryPrompt && activeWorkspace === 'scenes' && (
            <div className="scene-recovery-banner" role="alert">
              <div>
                <strong>Unsaved local scene edits found</strong>
                <span>Restore them or keep the latest saved scene plan from the backend.</span>
              </div>
              <button className="generate-btn secondary" type="button" onClick={restoreSceneDraft}>Restore</button>
              <button className="generate-btn secondary" type="button" onClick={discardSceneDraft}>Discard</button>
            </div>
          )}

          {activeWorkspace === 'script' ? (
          <div className="script-lab-grid">
            <main className="script-editor-panel">
              <EditorField
                id="script-section-hook"
                label="Hook"
                value={draft.hook}
                rows={3}
                maxLength={2000}
                onChange={value => updateDraft(current => ({ ...current, hook: value }))}
                onCopy={() => void copyText(draft.hook)}
              />
              <EditorField
                id="script-section-script"
                label="Main script"
                value={draft.mainScript}
                rows={14}
                maxLength={20000}
                onChange={value => updateDraft(current => ({ ...current, mainScript: value }))}
                onCopy={() => void copyText(draft.mainScript)}
              />
              <EditorField
                id="script-section-caption"
                label="Caption"
                value={draft.caption}
                rows={5}
                maxLength={4000}
                onChange={value => updateDraft(current => ({ ...current, caption: value }))}
                onCopy={() => void copyText(draft.caption)}
              />
              <EditorField
                id="script-section-hashtags"
                label="Hashtags"
                value={draft.hashtags}
                rows={3}
                maxLength={4800}
                hint={`${normalizeHashtags(draft.hashtags).length} hashtag(s)`}
                onChange={value => updateDraft(current => ({ ...current, hashtags: value }))}
                onCopy={() => void copyText(normalizeHashtags(draft.hashtags).map(tag => `#${tag}`).join(' '))}
              />

              <section id="script-section-platform-text" className="platform-text-section script-platform-editor">
                <div className="script-section-heading">
                  <span>Platform copy</span>
                  <strong>Readable channel-specific publishing text</strong>
                </div>
                <div className="platform-editor-grid">
                  {platformEditors.map(platform => (
                    <EditorField
                      key={platform.key}
                      id={`platform-copy-${platform.key}`}
                      label={platform.label}
                      value={draft.platformText[platform.key] || ''}
                      rows={platform.rows}
                      maxLength={6000}
                      onChange={value => updateDraft(current => ({
                        ...current,
                        platformText: { ...current.platformText, [platform.key]: value },
                      }))}
                      onCopy={() => void copyText(draft.platformText[platform.key] || '')}
                    />
                  ))}
                </div>
              </section>
            </main>

            <aside className="script-lab-side" aria-label="Script project context">
              <section className="script-lab-card">
                <div className="script-section-heading">
                  <span>Production fit</span>
                  <strong>Estimate</strong>
                </div>
                <div className="script-metric-grid">
                  <div>
                    <span>Words</span>
                    <strong>{draftStats.words.toLocaleString()}</strong>
                  </div>
                  <div>
                    <span>Speaking time</span>
                    <strong>{estimateLabel}</strong>
                  </div>
                </div>
                <div className={`duration-meter ${draftStats.tone}`}>
                  <span>Target {targetLabel}</span>
                  <strong>{draftStats.targetSeconds ? `${Math.round(draftStats.estimatedSeconds - draftStats.targetSeconds)}s vs target` : 'No target set'}</strong>
                </div>
              </section>

              <section className="script-lab-card">
                <div className="script-section-heading">
                  <span>Project</span>
                  <strong>Context</strong>
                </div>
                <div className="script-context-list">
                  <StatusRow label="Stage" value={humanizeKey(project.current_stage)} />
                  <StatusRow label="Status" value={humanizeKey(project.status)} />
                  <StatusRow label="Format" value={humanizeKey(project.content_format)} />
                  <StatusRow label="Created" value={formatDate(project.created_at)} />
                  <StatusRow label="Saved" value={updatedLabel} />
                </div>
              </section>

              <section className="script-lab-card">
                <div className="script-section-heading">
                  <span>Production copy</span>
                  <strong>Preview</strong>
                </div>
                <div className="script-copy-preview">{productionCopy || 'Start writing to build production copy.'}</div>
              </section>

              <EvidencePanel evidence={evidence} />
            </aside>
          </div>
          ) : (
          <div className="scene-plan-grid">
            <main className="scene-list-panel">
              {scenesLoading && (
                <div className="script-empty-state system-state is-empty" role="status">
                  <div className="empty-title">Loading scene plan...</div>
                  <div className="empty-desc">Fetching saved production scenes.</div>
                </div>
              )}
              {!scenesLoading && scenes.length === 0 && (
                <div className="script-empty-state system-state is-empty" role="status">
                  <div className="empty-title">No scene plan yet.</div>
                  <div className="empty-desc">Generate from the saved script or add scenes manually.</div>
                  <button className="generate-btn idle" type="button" onClick={addScene}>Add first scene</button>
                </div>
              )}
              {scenes.map((scene, index) => (
                <article className="scene-editor" key={scene.id}>
                  <div className="scene-editor-header">
                    <div>
                      <span>Scene {index + 1}</span>
                      <input
                        className="form-input"
                        value={scene.title}
                        maxLength={120}
                        placeholder="Optional title"
                        onChange={event => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, title: event.target.value } : item))}
                      />
                    </div>
                    <div className="scene-actions">
                      <button type="button" className="mini-copy-btn" onClick={() => moveScene(index, -1)} disabled={index === 0}>Move up</button>
                      <button type="button" className="mini-copy-btn" onClick={() => moveScene(index, 1)} disabled={index === scenes.length - 1}>Move down</button>
                      <button type="button" className="mini-copy-btn" onClick={() => duplicateScene(index)}>Duplicate</button>
                      <button type="button" className="mini-copy-btn danger" onClick={() => requestDeleteScene(index)}>Delete</button>
                    </div>
                  </div>

                  <div className="scene-primary-grid">
                    <EditorField
                      id={`scene-${scene.id}-spoken`}
                      label="Spoken text"
                      value={scene.spoken_text}
                      rows={5}
                      maxLength={5000}
                      onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, spoken_text: value } : item))}
                      onCopy={() => void copyText(scene.spoken_text)}
                    />
                    <div className="scene-side-fields">
                      <label className="script-editor-field">
                        <span className="script-editor-label-row"><span>Duration seconds</span></span>
                        <input
                          className="form-input"
                          type="number"
                          min={1}
                          max={600}
                          value={scene.planned_duration_seconds}
                          onChange={event => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, planned_duration_seconds: Math.max(1, Math.min(600, Number(event.target.value) || 1)) } : item))}
                        />
                      </label>
                      <EditorField
                        id={`scene-${scene.id}-onscreen`}
                        label="On-screen text"
                        value={scene.on_screen_text}
                        rows={3}
                        maxLength={500}
                        onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, on_screen_text: value } : item))}
                        onCopy={() => void copyText(scene.on_screen_text)}
                      />
                    </div>
                  </div>

                  <details className="scene-details">
                    <summary>Production details</summary>
                    <div className="scene-detail-grid">
                      <EditorField id={`scene-${scene.id}-visual`} label="Visual direction" value={scene.visual_direction} rows={3} maxLength={2000} onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, visual_direction: value } : item))} onCopy={() => void copyText(scene.visual_direction)} />
                      <EditorField id={`scene-${scene.id}-broll`} label="B-roll / supporting visual" value={scene.broll_direction} rows={3} maxLength={1200} onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, broll_direction: value } : item))} onCopy={() => void copyText(scene.broll_direction)} />
                      <EditorField id={`scene-${scene.id}-camera`} label="Camera / framing" value={scene.camera_direction} rows={2} maxLength={1000} onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, camera_direction: value } : item))} onCopy={() => void copyText(scene.camera_direction)} />
                      <EditorField id={`scene-${scene.id}-transition`} label="Transition" value={scene.transition_direction} rows={2} maxLength={700} onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, transition_direction: value } : item))} onCopy={() => void copyText(scene.transition_direction)} />
                      <EditorField id={`scene-${scene.id}-notes`} label="Production notes" value={scene.production_notes} rows={3} maxLength={1500} onChange={value => updateScenes(current => current.map(item => item.id === scene.id ? { ...item, production_notes: value } : item))} onCopy={() => void copyText(scene.production_notes)} />
                    </div>
                  </details>
                </article>
              ))}
            </main>

            <aside className="script-lab-side scene-summary-panel" aria-label="Scene plan summary">
              <section className="script-lab-card">
                <div className="script-section-heading">
                  <span>Scene Plan</span>
                  <strong>{sceneStatusLabel[sceneSaveState]}</strong>
                </div>
                <div className="script-metric-grid">
                  <div><span>Scenes</span><strong>{scenes.length}</strong></div>
                  <div><span>Planned</span><strong>{formatDuration(sceneStats.total)}</strong></div>
                </div>
                <div className={`duration-meter ${sceneStats.tone}`}>
                  <span>Target {targetLabel}</span>
                  <strong>{sceneStats.label}</strong>
                </div>
                {sceneStats.warnings.length > 0 && (
                  <ul className="scene-warning-list">
                    {sceneStats.warnings.map(warning => <li key={warning}>{warning}</li>)}
                  </ul>
                )}
              </section>
              <section className="script-lab-card">
                <div className="script-section-heading">
                  <span>Project</span>
                  <strong>Context</strong>
                </div>
                <div className="script-context-list">
                  <StatusRow label="Stage" value={humanizeKey(project.current_stage)} />
                  <StatusRow label="Status" value={humanizeKey(project.status)} />
                  <StatusRow label="Format" value={humanizeKey(project.content_format)} />
                  <StatusRow label="Saved" value={updatedLabel} />
                </div>
              </section>
            </aside>
          </div>
          )}
        </form>
        {sceneConfirmation && (
          <ConfirmationDialog
            open
            title={sceneConfirmation.type === 'replace-scene-plan' ? 'Replace scene plan?' : `Delete ${sceneConfirmation.label}?`}
            description={sceneConfirmation.type === 'replace-scene-plan'
              ? <>Generating a new plan will replace your {sceneConfirmation.sceneCount} existing {sceneConfirmation.sceneCount === 1 ? 'scene' : 'scenes'}. Your current plan will remain unchanged if generation fails.</>
              : 'This will permanently remove the scene from this project.'}
            confirmLabel={sceneConfirmation.type === 'replace-scene-plan' ? 'Replace plan' : 'Delete scene'}
            intent={sceneConfirmation.type === 'replace-scene-plan' ? 'standard' : 'destructive'}
            pending={sceneConfirmation.type === 'replace-scene-plan' && sceneGenerateBusy}
            pendingLabel={sceneConfirmation.type === 'replace-scene-plan' ? 'Generating plan...' : 'Deleting...'}
            errorMessage={sceneConfirmationError}
            onConfirm={confirmSceneAction}
            onCancel={closeSceneConfirmation}
          />
        )}
      </section>
    );
  }

  const pkg = activeScript.package;
  const sourceType = pkg.source_type || activeScript.candidate.source;
  const sourceURL = pkg.source_url || pkg.provider_metadata.source_url || activeScript.candidate.source_url;
  const createdAt = pkg.created_at || pkg.provider_metadata.generated_at || activeScript.savedAt;
  const title = activeScript.candidate.title || activeScript.candidate.keyword || pkg.title;
  const hashtagText = pkg.hashtags.join(' ');
  const evidence = buildEvidenceViewModel(pkg.grounding, pkg.safety_grounding_notes);
  const exportText = [
    `Source: ${sourceLabel(sourceType)}`,
    `Title: ${title}`,
    pkg.inferred_niche ? `Niche: ${pkg.inferred_niche}` : '',
    pkg.inferred_angle ? `Angle: ${pkg.inferred_angle}` : '',
    `Hook:\n${pkg.hook}`,
    `Script:\n${pkg.script}`,
    `Caption:\n${pkg.caption}`,
    `Description:\n${pkg.description || pkg.youtube_description}`,
    hashtagText ? `Hashtags:\n${hashtagText}` : '',
    pkg.thumbnail_brief ? `Thumbnail brief:\n${pkg.thumbnail_brief}` : '',
    pkg.youtube_title ? `YouTube title:\n${pkg.youtube_title}` : '',
    pkg.youtube_description ? `YouTube description:\n${pkg.youtube_description}` : '',
  ].filter(Boolean).join('\n\n');

  function scrollToSection(section: ScriptSectionID) {
    const element = document.getElementById(`script-section-${section}`);
    if (highlightTimer.current) window.clearTimeout(highlightTimer.current);
    document.querySelectorAll('.script-card-highlight').forEach(node => {
      node.classList.remove('script-card-highlight');
    });
    element?.classList.add('script-card-highlight');
    element?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    window.requestAnimationFrame(() => {
      element?.classList.add('script-card-highlight');
    });
    highlightTimer.current = window.setTimeout(() => {
      element?.classList.remove('script-card-highlight');
    }, 1300);
  }

  function useInClipGenerator() {
    storage.setClipGeneratorHandoff({
      projectId: project?.id,
      title,
      hook: pkg.hook,
      script: pkg.script,
      caption: pkg.caption,
      platformText: pkg.platform_posts,
      savedAt: new Date().toISOString(),
    });
    onUseInClipGenerator?.();
  }

  return (
    <section className="page-section script-studio-page">
      <div className="script-shell">
        <header className="script-hero">
          <div className="script-source-line">
            <span className="script-source-badge">{sourceLabel(sourceType)}</span>
            {project && <span className="script-source-badge">Project-backed</span>}
            <span>saved {formatDate(createdAt)}</span>
            {sourceURL && <a href={sourceURL} target="_blank" rel="noreferrer">Source evidence</a>}
          </div>
          <h1>{title}</h1>
          <div className="script-badge-row">
            {pkg.inferred_niche && <span className="script-badge">{pkg.inferred_niche}</span>}
            {pkg.inferred_angle && <span className="script-badge">{pkg.inferred_angle}</span>}
            {pkg.inferred_keywords?.slice(0, 5).map(keyword => <span className="script-badge" key={keyword}>{keyword}</span>)}
          </div>
        </header>

        <nav className="script-action-bar" aria-label="Script actions">
          {!project && latestScript && (
            <button className="generate-btn secondary" type="button" onClick={() => void handleImportLegacy()} disabled={importBusy}>
              {importBusy ? 'Importing...' : 'Import to Project'}
            </button>
          )}
          <button className="generate-btn secondary" type="button" onClick={() => void copyText(exportText)}>Copy all</button>
          {(Object.keys(sectionLabels) as ScriptSectionID[]).map(section => (
            <a className="generate-btn secondary" href={`#script-section-${section}`} role="button" onClick={() => scrollToSection(section)} key={section}>
              {sectionLabels[section]}
            </a>
          ))}
          <a className="generate-btn secondary" href="#script-section-platform-text" role="button" onClick={() => document.getElementById('script-section-platform-text')?.scrollIntoView({ behavior: 'smooth', block: 'start' })}>Platform text</a>
          {onUseInClipGenerator && (
            <button className="generate-btn idle" type="button" onClick={useInClipGenerator}>Use in Clip Generator</button>
          )}
        </nav>

        {importError && <div className="clip-error">{importError}</div>}

        <main className="script-content">
          <ScriptCard id="script-section-hook" title="Hook" value={pkg.hook} onCopy={() => void copyText(pkg.hook)} />
          <ScriptCard id="script-section-script" title="Script" value={pkg.script} onCopy={() => void copyText(pkg.script)} />
          <ScriptCard id="script-section-caption" title="Caption" value={pkg.caption} onCopy={() => void copyText(pkg.caption)} />
          <ScriptCard id="script-section-description" title="Description" value={pkg.description || pkg.youtube_description} onCopy={() => void copyText(pkg.description || pkg.youtube_description)} />
          <ScriptCard id="script-section-hashtags" title="Hashtags" value={hashtagText} onCopy={() => void copyText(hashtagText)} />
          <ScriptCard id="script-section-thumbnail" title="Thumbnail brief" value={pkg.thumbnail_brief} onCopy={() => void copyText(pkg.thumbnail_brief)} />

          <section id="script-section-platform-text" className="platform-text-section">
            <div className="script-section-heading">
              <span>Platform text</span>
              <strong>Ready-to-post copy by channel</strong>
            </div>
            <div className="platform-copy-grid">
              <PlatformCard label="Instagram" value={pkg.instagram_caption} />
              <PlatformCard label="TikTok" value={pkg.tiktok_caption} />
              <PlatformCard label="YouTube" value={pkg.youtube_title} subValue={pkg.youtube_description} />
              <PlatformCard label="Facebook" value={pkg.facebook_caption} />
              <PlatformCard label="X" value={pkg.x_caption} />
            </div>
          </section>

          <EvidencePanel evidence={evidence} />
        </main>
      </div>
    </section>
  );
}
