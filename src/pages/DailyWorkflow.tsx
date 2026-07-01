export function DailyWorkflowPage() {
  return (
    <section className="page-section">
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 20 }}>
        {['0 approved', '0 scheduled', '0 needs review', '0 draft', '0 in pipeline'].map(label => (
          <span
            key={label}
            style={{
              padding: '5px 11px',
              borderRadius: 20,
              fontSize: 10,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-muted)',
              background: 'var(--bg-raised)',
              border: '1px solid var(--border-card)',
              fontWeight: 600,
            }}
          >
            {label}
          </span>
        ))}
      </div>

      <div className="empty-state">
        <div className="empty-icon">WF</div>
        <div className="empty-title">No batch runs yet.</div>
        <div className="empty-desc">No uploads have been attempted. Connect accounts and configure provider keys first.</div>
      </div>
    </section>
  );
}
