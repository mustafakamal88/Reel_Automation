import type { StoredScriptPackage } from '../lib/storage';

interface Props {
  latestScript: StoredScriptPackage | null;
  onUseInClipGenerator?: () => void;
}

async function copyText(value: string) {
  if (!value) return;
  await navigator.clipboard?.writeText(value);
}

function TextBlock({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <div style={{ display: 'grid', gap: 5 }}>
      <div style={{ fontSize: 11, fontFamily: 'var(--font-mono)', color: 'var(--text-dim)', textTransform: 'uppercase' }}>{label}</div>
      <div style={{ fontSize: 13, color: 'var(--text-secondary)', lineHeight: 1.65, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{value}</div>
    </div>
  );
}

function sourceLabel(source?: string): string {
  switch (source) {
    case 'youtube_video_analysis':
      return 'YouTube Video Analysis';
    case 'youtube_channel_analysis':
      return 'YouTube Channel Analysis';
    case 'google_trend':
    case 'google_trends_rss':
      return 'Trend';
    case 'niche_idea':
      return 'Niche';
    default:
      return source?.replaceAll('_', ' ') || 'Research';
  }
}

export function ScriptStudioPage({ latestScript, onUseInClipGenerator }: Props) {
  if (!latestScript) {
    return (
      <section className="page-section">
        <div className="empty-state">
          <div className="empty-icon">SC</div>
          <div className="empty-title">Generate a script from Trend Finder to begin.</div>
          <div className="empty-desc">Script Studio only shows real generated script packages.</div>
        </div>
      </section>
    );
  }

  const pkg = latestScript.package;
  const sourceType = pkg.source_type || latestScript.candidate.source;
  const sourceURL = pkg.source_url || pkg.provider_metadata.source_url || latestScript.candidate.source_url;
  const createdAt = pkg.created_at || pkg.provider_metadata.generated_at || latestScript.savedAt;
  const platformText = [
    ['Instagram', pkg.instagram_caption],
    ['TikTok', pkg.tiktok_caption],
    ['YouTube title', pkg.youtube_title],
    ['YouTube description', pkg.youtube_description],
    ['Facebook', pkg.facebook_caption],
    ['X', pkg.x_caption],
  ].filter(([, value]) => Boolean(value));

  const exportText = [
    `Trend: ${latestScript.candidate.title || latestScript.candidate.keyword}`,
    `Hook: ${pkg.hook}`,
    `Script: ${pkg.script}`,
    `Caption: ${pkg.caption}`,
    `Description: ${pkg.youtube_description}`,
    `Hashtags: ${pkg.hashtags.join(' ')}`,
    `Thumbnail brief: ${pkg.thumbnail_brief}`,
  ].join('\n\n');
  const hashtagText = pkg.hashtags.join(' ');

  return (
    <section className="page-section">
      <div className="settings-card" style={{ display: 'grid', gap: 18, maxWidth: 980 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', alignItems: 'flex-start' }}>
          <div>
            <div style={{ fontSize: 12, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>
              {sourceLabel(sourceType)}
            </div>
            <div style={{ fontSize: 20, fontWeight: 800, color: 'var(--text-primary)', marginTop: 5, overflowWrap: 'anywhere' }}>
              {latestScript.candidate.title || latestScript.candidate.keyword}
            </div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', marginTop: 8 }}>
              <span className="platform-toggle">{sourceLabel(sourceType)}</span>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>saved {new Date(createdAt).toLocaleString()}</span>
              {sourceURL && <a href={sourceURL} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: 'var(--accent)' }}>Source evidence</a>}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(exportText)}>Copy all</button>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(pkg.hook)}>Copy hook</button>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(pkg.script)}>Copy script</button>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(pkg.caption)}>Copy caption</button>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(hashtagText)}>Copy hashtags</button>
            {onUseInClipGenerator && <button className="generate-btn idle" type="button" onClick={onUseInClipGenerator}>Use in Clip Generator</button>}
          </div>
        </div>

        {(pkg.inferred_keywords?.length || pkg.inferred_niche || pkg.inferred_angle) && (
          <div className="status-list">
            {pkg.inferred_keywords?.length ? <div className="status-row"><span>Inferred keywords</span><strong>{pkg.inferred_keywords.join(', ')}</strong></div> : null}
            {pkg.inferred_niche ? <div className="status-row"><span>Inferred niche</span><strong>{pkg.inferred_niche}</strong></div> : null}
            {pkg.inferred_angle ? <div className="status-row"><span>Inferred angle</span><strong>{pkg.inferred_angle}</strong></div> : null}
          </div>
        )}

        <TextBlock label="Hook" value={pkg.hook} />
        <TextBlock label="Script" value={pkg.script} />
        <TextBlock label="Caption" value={pkg.caption} />
        <TextBlock label="Description" value={pkg.description || pkg.youtube_description} />
        {pkg.hashtags.length > 0 && <TextBlock label="Hashtags" value={pkg.hashtags.join(' ')} />}
        <TextBlock label="Thumbnail brief" value={pkg.thumbnail_brief} />
        <TextBlock label="Grounding / evidence" value={pkg.grounding} />

        {platformText.length > 0 && (
          <div style={{ display: 'grid', gap: 10 }}>
            <div style={{ fontSize: 11, fontFamily: 'var(--font-mono)', color: 'var(--text-dim)', textTransform: 'uppercase' }}>Platform text</div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10 }}>
              {platformText.map(([label, value]) => (
                <div key={label} style={{ border: '1px solid var(--border-card)', borderRadius: 8, padding: 12, background: 'var(--bg-subtle)' }}>
                  <div style={{ fontSize: 12, fontWeight: 800, color: 'var(--text-primary)', marginBottom: 6 }}>{label}</div>
                  <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.55, overflowWrap: 'anywhere' }}>{value}</div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
