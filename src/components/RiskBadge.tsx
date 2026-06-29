import { riskMeta } from '../lib/scoring';

interface Props {
  score: number;
}

export function RiskBadge({ score }: Props) {
  const { color, bg, label } = riskMeta(score);
  return (
    <span
      className="risk-badge"
      style={{ color, background: bg }}
      title={`Risk score: ${score}/100`}
    >
      {label}
    </span>
  );
}
