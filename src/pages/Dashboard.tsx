import type { StoredScriptPackage } from '../lib/storage';

interface Props {
  latestScript: StoredScriptPackage | null;
  onNavigate: (view: 'trendFinder' | 'scriptStudio' | 'clipStudio' | 'publish') => void;
}

export function DashboardPage({ latestScript, onNavigate }: Props) {
  const cards = [
    { title: 'Trends found', body: 'Real trend candidates appear after Trend Finder loads connected sources.', action: 'Open Trend Finder', view: 'trendFinder' as const },
    { title: 'Scripts generated', body: latestScript ? latestScript.package.title : 'Generate a script from Trend Finder to begin.', action: 'Open Script Studio', view: 'scriptStudio' as const },
    { title: 'Clips generated', body: 'Upload a source file or use a direct video URL. ZIP downloads stay inside Clip Generator.', action: 'Open Clip Generator', view: 'clipStudio' as const },
    { title: 'Publishing status', body: 'Social upload is disabled until real OAuth and platform API setup exists.', action: 'Open Publish', view: 'publish' as const },
  ];

  return (
    <section className="page-section">
      <div style={{ maxWidth: 880, marginBottom: 18 }}>
        <div style={{ fontSize: 24, fontWeight: 800, color: 'var(--text-primary)' }}>Catch trends. Create clips. Publish everywhere.</div>
        <div style={{ fontSize: 13, color: 'var(--text-muted)', lineHeight: 1.6, marginTop: 7 }}>
          TrendCortex is focused on real trend discovery, script generation, clip creation, and future social publishing.
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12 }}>
        {cards.map(card => (
          <div key={card.title} className="settings-card" style={{ display: 'flex', flexDirection: 'column', gap: 12, minHeight: 160 }}>
            <div>
              <div style={{ fontSize: 15, fontWeight: 800, color: 'var(--text-primary)' }}>{card.title}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.55, marginTop: 7 }}>{card.body}</div>
            </div>
            <button className="generate-btn idle" type="button" onClick={() => onNavigate(card.view)} style={{ marginTop: 'auto', justifyContent: 'center' }}>
              {card.action}
            </button>
          </div>
        ))}
      </div>

      <div className="settings-card" style={{ marginTop: 14, display: 'flex', gap: 10, alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' }}>
        <div>
          <div style={{ fontSize: 14, fontWeight: 800, color: 'var(--text-primary)' }}>Publishing</div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>
            Social upload is disabled until real OAuth and platform API setup exists. Manual ZIP download is available.
          </div>
        </div>
        <button className="generate-btn idle" type="button" onClick={() => onNavigate('publish')}>
          Open Publish
        </button>
      </div>
    </section>
  );
}
