import { KEYWORDS } from '../data/keywords';
import { RiskBadge } from '../components/RiskBadge';
import { ScoreBar } from '../components/ScoreBar';

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
        {/* Table header */}
        <div className="scoring-header" role="row">
          {COL_HEADERS.map(h => (
            <div
              key={h.key}
              role="columnheader"
              title={h.title}
              style={{ textAlign: h.align }}
            >
              {h.label}
            </div>
          ))}
        </div>

        {/* Rows */}
        {KEYWORDS.map(kw => {
          return (
            <div key={kw.id} className="scoring-row" role="row">
              {/* Keyword */}
              <div style={{ paddingRight: 10 }}>
                <div className="scoring-kw-name">{kw.kw}</div>
                <div className="scoring-kw-niche">{kw.niche}</div>
              </div>

              {/* Factors — green for positive */}
              <div className="scoring-cell" style={{ color: '#5fd39a' }}>+{kw.trendGrowth}</div>
              <div className="scoring-cell" style={{ color: 'var(--text-muted)' }}>+{kw.crossPlatform}</div>
              <div className="scoring-cell" style={{ color: 'var(--text-muted)' }}>+{kw.viewsPerHour}</div>
              <div className="scoring-cell" style={{ color: 'var(--text-muted)' }}>+{kw.engagement}</div>
              <div className="scoring-cell" style={{ color: 'var(--text-muted)' }}>+{kw.searchInterest}</div>
              <div className="scoring-cell" style={{ color: 'var(--text-muted)' }}>+{kw.nicheMatch}</div>

              {/* Low competition bonus — blue */}
              <div className="scoring-cell" style={{ color: '#6aa3f0' }}>+{kw.lowCompBonus}</div>

              {/* Saturation penalty — red */}
              <div className="scoring-cell" style={{ color: '#e8736b' }}>−{kw.saturationPenalty}</div>

              {/* Risk badge */}
              <div className="scoring-cell">
                <RiskBadge score={kw.riskScore} />
              </div>

              {/* Final score */}
              <div style={{ textAlign: 'right', display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 8 }}>
                <ScoreBar score={kw.finalScore} width={34} />
              </div>
            </div>
          );
        })}
      </div>

      <div className="scoring-formula">
        score = trend_growth + cross_platform + views_per_hour + engagement + search_interest + niche_match + low_competition_bonus − saturation_penalty − risk_penalty
      </div>

      <div style={{ marginTop: 12, display: 'flex', gap: 8 }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Demo data
        </span>
      </div>
    </section>
  );
}
