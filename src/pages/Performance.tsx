import { PERF_STATS, PERF_POSTS, PLATFORM_BARS } from '../data/performance';
import { PLATFORMS } from '../data/platforms';

export function PerformancePage() {
  const maxBar = Math.max(...PLATFORM_BARS.map(b => b.rawValue));

  return (
    <section className="page-section">
      {/* Stat cards */}
      <div className="perf-stats-grid">
        {PERF_STATS.map((stat, i) => (
          <div key={i} className="stat-card">
            <div className="stat-label">{stat.label}</div>
            <div className="stat-value">{stat.value}</div>
            <div
              className="stat-delta"
              style={{ color: stat.positive ? 'var(--green)' : 'var(--text-muted)' }}
            >
              {stat.delta}
            </div>
          </div>
        ))}
      </div>

      <div className="perf-bottom">
        {/* Posts table */}
        <div className="perf-posts-table">
          <div className="perf-table-title">
            Published — last 7 days{' '}
            <span className="muted-text">· own accounts, real watch-time</span>
          </div>
          <div className="perf-table-header" role="row">
            <div>Post</div>
            <div style={{ textAlign: 'right' }}>Views</div>
            <div style={{ textAlign: 'right' }}>Avg view</div>
            <div style={{ textAlign: 'right' }}>Retention</div>
          </div>
          {PERF_POSTS.map(post => {
            const meta = PLATFORMS[post.platform];
            const retColor =
              post.retention >= 65 ? '#5fd39a' : post.retention >= 50 ? '#9fd16a' : '#e3b54e';
            return (
              <div key={post.id} className="perf-table-row" role="row">
                <div className="perf-post-title-cell">
                  <span
                    className="src-badge"
                    style={{ color: meta.color, background: meta.bg }}
                  >
                    {meta.short}
                  </span>
                  <span className="perf-post-title" title={post.title}>{post.title}</span>
                </div>
                <div className="perf-views">{post.views}</div>
                <div className="perf-avd">{post.avgDuration}</div>
                <div className="perf-ret-cell">
                  <div
                    style={{
                      width: 46,
                      height: 5,
                      background: 'var(--border-strong)',
                      borderRadius: 3,
                      overflow: 'hidden',
                    }}
                  >
                    <div
                      style={{
                        height: '100%',
                        width: `${post.retention}%`,
                        background: retColor,
                        borderRadius: 3,
                      }}
                    />
                  </div>
                  <span className="perf-ret-value" style={{ color: retColor }}>
                    {post.retention}%
                  </span>
                </div>
              </div>
            );
          })}
        </div>

        {/* Platform bars */}
        <div className="platform-bars-card">
          <div className="plat-bars-title">
            Views by platform{' '}
            <span className="muted-text">· 7d</span>
          </div>
          <div className="plat-bars-list">
            {PLATFORM_BARS.map(bar => {
              const meta = PLATFORMS[bar.platform];
              const pct = Math.round((bar.rawValue / maxBar) * 100);
              return (
                <div key={bar.platform} className="plat-bar-item">
                  <div className="plat-bar-labels">
                    <span className="plat-bar-name">{bar.name}</span>
                    <span className="plat-bar-value">{bar.value}</span>
                  </div>
                  <div className="plat-bar-track">
                    <div
                      className="plat-bar-fill"
                      style={{ width: `${pct}%`, background: meta.color }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      <div style={{ marginTop: 16 }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Demo data
        </span>
      </div>
    </section>
  );
}
