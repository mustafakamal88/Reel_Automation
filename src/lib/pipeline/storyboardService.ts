import type { ScriptDraft, StoryboardScene } from '../../types';

// MOCK ONLY — generates storyboard scenes from script beats locally.

const VISUAL_TEMPLATES = [
  'Bold kinetic title card — text slams in on beat',
  'Screen recording with live cursor movement',
  'Split-screen comparison (before / after)',
  'Talking head — direct camera, no cuts',
  'B-roll with text overlay and animated caption',
  'Animated graph or metric filling in',
  'Phone mockup displaying app or tool',
  'Fast-cut montage with frame-locked timestamps',
];

const BROLL_OPTIONS = [
  'Hook overlay — pattern-interrupt graphic',
  'Supporting screen capture of tool in use',
  'Hands-on-keyboard footage',
  'Result reveal screen recording',
  'Side-by-side product comparison clip',
  'Reaction / talking head cutaway',
];

export async function generateStoryboard(script: ScriptDraft): Promise<StoryboardScene[]> {
  await new Promise<void>(r => setTimeout(r, 520));

  return script.beats.map((beat, i) => {
    const isFirst = i === 0;
    const isLast = i === script.beats.length - 1;
    const type: StoryboardScene['type'] = isFirst ? 'hook' : isLast ? 'cta' : 'body';

    return {
      index: i,
      timeRange: beat.timeRange,
      visual: VISUAL_TEMPLATES[i % VISUAL_TEMPLATES.length],
      caption:
        beat.text.length > 64
          ? beat.text.slice(0, 64) + '…'
          : beat.text,
      type,
      bRoll: BROLL_OPTIONS[i % BROLL_OPTIONS.length],
    };
  });
}
