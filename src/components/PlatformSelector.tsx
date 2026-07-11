import type { CSSProperties } from 'react';
import type { Platform } from '../types';
import { PLATFORMS } from '../data/platforms';

const DEFAULT_PLATFORMS: Platform[] = ['yt', 'tt', 'ig', 'fb', 'x'];

interface Props {
  value: Platform[];
  onChange: (platforms: Platform[]) => void;
  platforms?: Platform[];
  label?: string;
  description?: string;
  compact?: boolean;
}

export function PlatformSelector({
  value,
  onChange,
  platforms = DEFAULT_PLATFORMS,
  label = 'Target platforms',
  description,
  compact = false,
}: Props) {
  function toggle(platform: Platform) {
    onChange(value.includes(platform) ? value.filter(item => item !== platform) : [...value, platform]);
  }

  return (
    <div className={`platform-selector${compact ? ' compact' : ''}`}>
      <div className="platform-selector-heading">
        <div>
          <div className="settings-card-title">{label}</div>
          {description && <div className="muted-note">{description}</div>}
        </div>
      </div>
      <div className="platform-selector-options" role="group" aria-label={label}>
        {platforms.map(platform => {
          const meta = PLATFORMS[platform];
          const selected = value.includes(platform);
          return (
            <button
              key={platform}
              type="button"
              className={`platform-selector-option${selected ? ' selected' : ''}`}
              aria-pressed={selected}
              onClick={() => toggle(platform)}
              style={{
                '--platform-color': meta.color,
                '--platform-bg': meta.bg,
              } as CSSProperties}
            >
              <span className="platform-selector-dot" aria-hidden="true" />
              <span>{meta.name}</span>
              <span className="platform-selector-check" aria-hidden="true">{selected ? '✓' : ''}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
