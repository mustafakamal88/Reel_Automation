import { sparkPoints } from '../lib/scoring';

interface Props {
  data: number[];
  color: string;
  width?: number;
  height?: number;
}

export function Sparkline({ data, color, width = 86, height = 30 }: Props) {
  const pts = sparkPoints(data);
  return (
    <svg
      width={width}
      height={height}
      viewBox="0 0 100 30"
      preserveAspectRatio="none"
      style={{ flexShrink: 0 }}
      aria-hidden="true"
    >
      <polyline
        points={pts}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
