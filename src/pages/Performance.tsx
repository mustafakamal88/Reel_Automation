export function PerformancePage() {
  return (
    <section className="page-section">
      <div className="perf-stats-grid">
        {['Views', 'Watch time', 'Revenue', 'Growth'].map(label => (
          <div key={label} className="stat-card">
            <div className="stat-label">{label}</div>
            <div className="stat-value">0</div>
            <div className="stat-delta" style={{ color: 'var(--text-muted)' }}>No real publishing history yet</div>
          </div>
        ))}
      </div>

      <div className="empty-state">
        <div className="empty-icon">PF</div>
        <div className="empty-title">No real publishing history yet.</div>
        <div className="empty-desc">Performance analytics will appear after real uploads are connected.</div>
      </div>
    </section>
  );
}
