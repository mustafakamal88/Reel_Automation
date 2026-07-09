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

  return (
    <section className="page-section">
      <div className="settings-card" style={{ display: 'grid', gap: 18, maxWidth: 980 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', alignItems: 'flex-start' }}>
          <div>
            <div style={{ fontSize: 12, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>
              Selected trend
            </div>
            <div style={{ fontSize: 20, fontWeight: 800, color: 'var(--text-primary)', marginTop: 5, overflowWrap: 'anywhere' }}>
              {latestScript.candidate.title || latestScript.candidate.keyword}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 6 }}>
              {latestScript.candidate.source.replaceAll('_', ' ')} · saved {new Date(latestScript.savedAt).toLocaleString()}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(exportText)}>Copy all</button>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(pkg.script)}>Copy script</button>
            <button className="generate-btn idle" type="button" onClick={() => void copyText(pkg.caption)}>Copy caption</button>
            {onUseInClipGenerator && <button className="generate-btn idle" type="button" onClick={onUseInClipGenerator}>Use in Clip Generator</button>}
          </div>
        </div>

        <TextBlock label="Hook" value={pkg.hook} />
        <TextBlock label="Script" value={pkg.script} />
        <TextBlock label="Caption" value={pkg.caption} />
        <TextBlock label="Description" value={pkg.youtube_description} />
        {pkg.hashtags.length > 0 && <TextBlock label="Hashtags" value={pkg.hashtags.join(' ')} />}
        <TextBlock label="Thumbnail brief" value={pkg.thumbnail_brief} />

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
