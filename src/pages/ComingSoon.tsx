interface Props {
  eyebrow: string;
  title: string;
  description: string;
}

export function ComingSoonPage({ eyebrow, title, description }: Props) {
  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">{eyebrow}</div>
          <h1>{title}</h1>
          <p>{description}</p>
        </div>
      </div>

      <div className="settings-card coming-soon-card">
        <div className="coming-soon-badge">Coming soon</div>
        <div>
          <div className="settings-card-title">{title}</div>
          <p className="settings-section-desc">
            This area is reserved for the planned workflow. It will stay empty until real product support is available.
          </p>
        </div>
      </div>
    </section>
  );
}
