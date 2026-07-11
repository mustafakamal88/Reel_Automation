import { useRef, type CSSProperties, type KeyboardEvent } from 'react';
import type { Platform } from '../types';
import { PLATFORMS } from '../data/platforms';

export interface TargetPlatformOption {
  id: Platform;
  label?: string;
  icon?: string;
  disabled?: boolean;
  unavailable?: boolean;
}

interface Props {
  platforms: TargetPlatformOption[];
  selectedPlatform: Platform;
  onSelect: (platform: Platform) => void;
  compact?: boolean;
  ariaLabel?: string;
}

export function TargetPlatformSelector({ platforms, selectedPlatform, onSelect, compact = false, ariaLabel = 'Target platforms' }: Props) {
  const buttonRefs = useRef<Array<HTMLButtonElement | null>>([]);

  function moveFocus(currentIndex: number, direction: 1 | -1) {
    const available = platforms
      .map((platform, index) => ({ platform, index }))
      .filter(item => !item.platform.disabled);
    const activeIndex = available.findIndex(item => item.index === currentIndex);
    const next = available[(activeIndex + direction + available.length) % available.length];
    if (next) buttonRefs.current[next.index]?.focus();
  }

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
    event.preventDefault();
    moveFocus(index, event.key === 'ArrowRight' ? 1 : -1);
  }

  return (
    <div className={`target-platform-selector${compact ? ' compact' : ''}`} role="radiogroup" aria-label={ariaLabel}>
      {platforms.map((platform, index) => {
        const meta = PLATFORMS[platform.id];
        const active = platform.id === selectedPlatform;
        const label = platform.label ?? meta.name;
        const icon = platform.icon ?? meta.short;
        return (
          <button
            key={platform.id}
            ref={element => {
              buttonRefs.current[index] = element;
            }}
            type="button"
            className={`target-platform-card${active ? ' is-active' : ''}${platform.unavailable ? ' is-unavailable' : ''}`}
            role="radio"
            aria-checked={active}
            disabled={platform.disabled}
            onClick={() => onSelect(platform.id)}
            onKeyDown={event => handleKeyDown(event, index)}
            style={{ '--platform-color': meta.color, '--platform-bg': meta.bg } as CSSProperties & Record<string, string>}
          >
            <span className="target-platform-icon" aria-hidden="true">{icon}</span>
            <span className="target-platform-label">{label}</span>
            {platform.unavailable && <span className="target-platform-state">Unavailable</span>}
          </button>
        );
      })}
    </div>
  );
}
