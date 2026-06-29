import type { VideoTopic, StoryboardScene, AssetPlan } from '../../types';

// MOCK ONLY — selects copyright-safe assets from static lists, no external calls.

const MUSIC_TRACKS = [
  'Upbeat EDM — Pixabay (CC0)',
  'Lo-fi ambient focus — Pixabay (CC0)',
  'Motivational cinematic — Pixabay (CC0)',
  'Tech minimal electronic pulse — Pixabay (CC0)',
  'Energetic hip-hop instrumental — Pixabay (CC0)',
];

const VOICE_STYLES = [
  'Conversational direct',
  'High-energy hype',
  'Calm authoritative',
  'Storytelling casual',
];

const CAPTION_STYLES = [
  'Bold kinetic (white + black stroke)',
  'Clean minimal sans-serif',
  'TikTok-style pop captions',
  'Lower-third subtitle bar',
];

const COLOR_GRADES = [
  'Dark moody — crushed blacks + cool highlights',
  'Bright punchy — lifted shadows + warm skin',
  'Neon cyberpunk — teal/magenta split',
  'Warm natural — balanced, no heavy grade',
];

function pickByScore(list: string[], score: number): string {
  return list[Math.abs(Math.floor(score)) % list.length];
}

export async function planAssets(
  topic: VideoTopic,
  scenes: StoryboardScene[],
): Promise<AssetPlan> {
  await new Promise<void>(r => setTimeout(r, 420));

  const hasCopyrightRisk = topic.riskScore > 50;

  return {
    topicId: topic.id,
    musicTrack: pickByScore(MUSIC_TRACKS, topic.trendScore),
    voiceStyle: pickByScore(VOICE_STYLES, topic.riskScore),
    captionStyle: pickByScore(CAPTION_STYLES, topic.trendScore + 1),
    colorGrade: pickByScore(COLOR_GRADES, topic.riskScore + 2),
    requiredBRoll: scenes
      .filter(s => s.type !== 'hook')
      .map(s => s.bRoll),
    copyrightSafe: !hasCopyrightRisk,
    copyrightWarning: hasCopyrightRisk
      ? 'Topic risk score > 50 — review claims for copyright-sensitive content before rendering'
      : undefined,
  };
}
