import type { PerfPost, PlatformBar, PerfStats } from '../types';

export const PERF_STATS: PerfStats[] = [
  { label: 'Views · 7d', value: '2.71M', delta: '+34% vs prev', positive: true },
  { label: 'Avg retention', value: '62%', delta: '+5 pts', positive: true },
  { label: 'Best platform', value: 'YT Shorts', delta: '71% retention', positive: false },
  { label: 'Published', value: '14', delta: '9 approved today', positive: false },
];

export const PERF_POSTS: PerfPost[] = [
  { id: 'p1', platform: 'yt', title: '3 AI agents I run every day', views: '1.2M', avgDuration: '21s', retention: 71 },
  { id: 'p2', platform: 'tt', title: 'This prompt writes a week of content', views: '884K', avgDuration: '18s', retention: 64 },
  { id: 'p3', platform: 'ig', title: '$0 home studio setup', views: '392K', avgDuration: '14s', retention: 58 },
  { id: 'p4', platform: 'yt', title: 'Notion AI vs new note apps', views: '241K', avgDuration: '24s', retention: 67 },
  { id: 'p5', platform: 'fb', title: 'I automated my content calendar', views: '118K', avgDuration: '12s', retention: 49 },
];

export const PLATFORM_BARS: PlatformBar[] = [
  { platform: 'yt', name: 'YouTube Shorts', value: '1.44M', rawValue: 1440 },
  { platform: 'tt', name: 'TikTok', value: '884K', rawValue: 884 },
  { platform: 'ig', name: 'Instagram Reels', value: '392K', rawValue: 392 },
  { platform: 'fb', name: 'Facebook Reels', value: '118K', rawValue: 118 },
  { platform: 'x', name: 'X / Twitter', value: '74K', rawValue: 74 },
  { platform: 'th', name: 'Threads', value: '31K', rawValue: 31 },
];
