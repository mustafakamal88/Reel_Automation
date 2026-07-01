const COL_HEADERS = [
  { key: 'kw', label: 'Keyword', align: 'left' as const },
  { key: 'tg', label: 'Trend', title: 'Trend growth', align: 'center' as const },
  { key: 'xp', label: 'X-plat', title: 'Cross-platform', align: 'center' as const },
  { key: 'vph', label: 'V/hr', title: 'Views per hour', align: 'center' as const },
  { key: 'eng', label: 'Eng', title: 'Engagement ratio', align: 'center' as const },
  { key: 'si', label: 'Srch', title: 'Search interest', align: 'center' as const },
  { key: 'nm', label: 'Niche', title: 'Niche match', align: 'center' as const },
  { key: 'lc', label: 'LowC', title: 'Low competition bonus', align: 'center' as const },
  { key: 'sat', label: 'Sat', title: 'Saturation penalty', align: 'center' as const },
  { key: 'risk', label: 'Risk', align: 'center' as const },
  { key: 'score', label: 'Score', align: 'right' as const },
];

export function ScoringPage() {
  return (
    <section className="page-section">
      <div className="scoring-table">
        <div className="scoring-header" role="row">
          {COL_HEADERS.map(h => (
            <div key={h.key} role="columnheader" title={h.title} style={{ textAlign: h.align }}>
              {h.label}
            </div>
          ))}
        </div>
        <div className="empty-state" style={{ margin: 0, borderRadius: 0 }}>
          <div className="empty-icon">SC</div>
          <div className="empty-title">No live trend data connected yet.</div>
          <div className="empty-desc">Connect trend sources/API keys to start collecting real signals.</div>
        </div>
      </div>

      <div className="scoring-formula">
        score = trend_growth + cross_platform + views_per_hour + engagement + search_interest + niche_match + low_competition_bonus - saturation_penalty - risk_penalty
      </div>
    </section>
  );
}
