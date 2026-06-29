// Deterministic mock content generator.
// TODO: Replace generateHook/generateScript/generateCaption with real LLM calls
//       (e.g. Anthropic claude-sonnet-4-6 via the Messages API) when API keys are configured.

export interface GeneratedContent {
  hook: string;
  scriptBeats: { timeRange: string; text: string }[];
  caption: string;
  visualDirection: string;
  cta: string;
  hashtags: string[];
}

const HOOK_TEMPLATES = [
  'Nobody talks about {topic} — until now.',
  'I tested {topic} for 30 days. Here is what happened.',
  'Everyone is doing {topic} wrong. Here is the right way.',
  'This {topic} trick changed how I work.',
  'You are missing out if you haven\'t tried {topic} yet.',
];

const CTA_TEMPLATES = [
  'Follow for more. Comment below if this helped.',
  'Save this. You\'ll thank me later.',
  'Comment YES if you want the full breakdown.',
  'Follow and drop a 🔥 if this was useful.',
];

function pick<T>(arr: T[], seed: number): T {
  return arr[seed % arr.length];
}

function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) - h + s.charCodeAt(i)) | 0;
  }
  return Math.abs(h);
}

export function generateContent(topicTitle: string, niche: string = 'creator tools'): GeneratedContent {
  const seed = hash(topicTitle);
  const hook = pick(HOOK_TEMPLATES, seed).replace('{topic}', niche);

  const scriptBeats = [
    { timeRange: '0–2s', text: hook },
    { timeRange: '2–8s', text: `The context around ${topicTitle.toLowerCase()} is shifting fast in 2026.` },
    { timeRange: '8–16s', text: `Here are the three things I changed after going deep on this topic.` },
    { timeRange: '16–20s', text: `Result: 40% more efficiency and way less guesswork.` },
    { timeRange: '20–24s', text: `Comment "${niche.split(' ')[0].toUpperCase()}" and I'll send you the full breakdown.` },
  ];

  const caption = `${hook} 👇\n\n${topicTitle}.\n\nDrop a comment if you want the full breakdown.`;

  const visualDirection = `Open on a bold title card, then cut to screen-share or b-roll showing the result. End with a strong CTA card.`;

  const cta = pick(CTA_TEMPLATES, seed + 1);

  const words = topicTitle.toLowerCase().split(' ').filter(w => w.length > 3);
  const hashtags = [
    ...words.slice(0, 3).map(w => `#${w.replace(/[^a-z0-9]/g, '')}`),
    '#aitools',
    '#creatortips',
    '#reels',
  ].filter(Boolean).slice(0, 6);

  return { hook, scriptBeats, caption, visualDirection, cta, hashtags };
}
