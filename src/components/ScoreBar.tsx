import { trendColor } from '../lib/scoring';

interface Props {
  score: number;
  maxScore?: number;
  width?: number;
}

export function ScoreBar({ score, maxScore = 100, width = 34 }: Props) {
  const color = trendColor(score);
  const pct = `${Math.round((score / maxScore) * 100)}%`;
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <div
        className="score-bar-track"
        style={{ width }}
        role="progressbar"
        aria-valuenow={score}
        aria-valuemin={0}
        aria-valuemax={maxScore}
      >
        <div className="score-bar-fill" style={{ width: pct, background: color }} />
      </div>
      <span
        style={{
          fontFamily: 'var(--font-mono)',
          fontWeight: 700,
          fontSize: 15,
          color,
          width: 26,
          textAlign: 'right',
        }}
      >
        {score}
      </span>
    </div>
  );
}
