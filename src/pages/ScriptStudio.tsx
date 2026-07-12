import { useRef } from 'react';
import type { StoredScriptPackage } from '../lib/storage';

interface Props {
  latestScript: StoredScriptPackage | null;
  onUseInClipGenerator?: () => void;
  onGoToTrendFinder?: () => void;
}

type ScriptSectionID = 'hook' | 'script' | 'caption' | 'hashtags';

type EvidenceRecord = Record<string, unknown>;

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

async function copyText(value: string) {
  if (!value) return;
  try {
    await navigator.clipboard?.writeText(value);
  } catch {
    // Clipboard access can be blocked by browser permissions; keep the UI stable.
  }
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

export function ScriptStudioPage({ latestScript, onUseInClipGenerator, onGoToTrendFinder }: Props) {
  const highlightTimer = useRef<number | null>(null);

  if (!latestScript) {
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

  const pkg = latestScript.package;
  const sourceType = pkg.source_type || latestScript.candidate.source;
  const sourceURL = pkg.source_url || pkg.provider_metadata.source_url || latestScript.candidate.source_url;
  const createdAt = pkg.created_at || pkg.provider_metadata.generated_at || latestScript.savedAt;
  const title = latestScript.candidate.title || latestScript.candidate.keyword || pkg.title;
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

  return (
    <section className="page-section script-studio-page">
      <div className="script-shell">
        <header className="script-hero">
          <div className="script-source-line">
            <span className="script-source-badge">{sourceLabel(sourceType)}</span>
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
          <button className="generate-btn secondary" type="button" onClick={() => void copyText(exportText)}>Copy all</button>
          {(Object.keys(sectionLabels) as ScriptSectionID[]).map(section => (
            <a className="generate-btn secondary" href={`#script-section-${section}`} role="button" onClick={() => scrollToSection(section)} key={section}>
              {sectionLabels[section]}
            </a>
          ))}
          <a className="generate-btn secondary" href="#script-section-platform-text" role="button" onClick={() => document.getElementById('script-section-platform-text')?.scrollIntoView({ behavior: 'smooth', block: 'start' })}>Platform text</a>
          {onUseInClipGenerator && (
            <button className="generate-btn idle" type="button" onClick={onUseInClipGenerator}>Use in Clip Generator</button>
          )}
        </nav>

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
