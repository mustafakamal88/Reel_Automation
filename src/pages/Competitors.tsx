import { COMPETITORS } from '../data/competitors';
import { PLATFORMS } from '../data/platforms';

export function CompetitorsPage() {
  return (
    <section className="page-section">
      <div className="competitors-list">
        {COMPETITORS.map(comp => {
          const meta = PLATFORMS[comp.platform];
          return (
            <article key={comp.id} className="competitor-row">
              <div
                className="comp-src-icon"
                style={{ background: meta.bg, color: meta.color }}
                aria-label={meta.name}
              >
                {meta.short}
              </div>

              <div className="comp-handle-block">
                <div className="comp-handle">{comp.handle}</div>
                <div className="comp-niche">{comp.niche}</div>
              </div>

              <div className="comp-stat">
                <div className="comp-stat-value">{comp.followers}</div>
                <div className="comp-stat-label">followers</div>
              </div>

              <div className="comp-stat">
                <div className="comp-stat-value">{comp.viewsPerHour}</div>
                <div className="comp-stat-label">views/hr</div>
              </div>

              <div className="comp-latest">
                <div className="comp-latest-label">latest break-out</div>
                <div className="comp-latest-title" title={comp.latestBreakout}>
                  {comp.latestBreakout}
                </div>
              </div>

              <div className="comp-velocity">
                <div
                  className="comp-vel-value"
                  style={{ color: comp.velocityUp ? '#5fd39a' : '#e8736b' }}
                >
                  {comp.velocity}
                </div>
                <div className="comp-vel-label">velocity</div>
              </div>
            </article>
          );
        })}
      </div>

      <div style={{ marginTop: 16 }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Demo data · public signals only
        </span>
      </div>
    </section>
  );
}
