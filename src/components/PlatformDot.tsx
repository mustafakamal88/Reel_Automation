import { PLATFORMS } from '../data/platforms';
import type { Platform } from '../types';

interface Props {
  platform: Platform;
  size?: number;
}

export function PlatformDot({ platform, size = 18 }: Props) {
  const meta = PLATFORMS[platform];
  return (
    <span
      className="plat-dot"
      title={meta.name}
      style={{
        width: size,
        height: size,
        background: meta.bg,
        color: meta.color,
        fontSize: size * 0.48,
      }}
    >
      {meta.short}
    </span>
  );
}

export function SrcBadge({ platform }: { platform: Platform }) {
  const meta = PLATFORMS[platform];
  return (
    <span
      className="src-badge"
      style={{ color: meta.color, background: meta.bg }}
    >
      {meta.short}
    </span>
  );
}
