import type { ScriptDraft, StoryboardScene } from '../../types';

export async function generateStoryboard(script: ScriptDraft): Promise<StoryboardScene[]> {
  return script.beats.map((beat, i) => {
    const isFirst = i === 0;
    const isLast = i === script.beats.length - 1;
    const type: StoryboardScene['type'] = isFirst ? 'hook' : isLast ? 'cta' : 'body';

    return {
      index: i,
      timeRange: beat.timeRange,
      visual: 'Provider not connected',
      caption:
        beat.text.length > 64
          ? beat.text.slice(0, 64) + '…'
          : beat.text,
      type,
      bRoll: 'Provider not connected',
    };
  });
}
