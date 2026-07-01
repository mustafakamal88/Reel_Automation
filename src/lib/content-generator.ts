export interface GeneratedContent {
  hook: string;
  scriptBeats: { timeRange: string; text: string }[];
  caption: string;
  visualDirection: string;
  cta: string;
  hashtags: string[];
}

export function generateContent(): GeneratedContent {
  return {
    hook: '',
    scriptBeats: [],
    caption: '',
    visualDirection: '',
    cta: '',
    hashtags: [],
  };
}
