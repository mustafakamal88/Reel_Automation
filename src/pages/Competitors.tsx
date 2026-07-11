export function CompetitorsPage() {
  return (
    <section className="page-section">
      <div className="empty-state system-state is-empty" role="status">
        <div className="empty-icon" aria-hidden="true">i</div>
        <div className="empty-title">No competitor tracking configured.</div>
        <div className="empty-desc">Add real competitor accounts before this page shows public trend tracking.</div>
      </div>
    </section>
  );
}
