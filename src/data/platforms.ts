import type { Platform, PlatformMeta } from '../types';

export const PLATFORMS: Record<Platform, PlatformMeta> = {
  yt: { name: 'YouTube Shorts', short: 'YT', color: '#ff6b66', bg: 'rgba(255,107,102,.14)' },
  ig: { name: 'Instagram Reels', short: 'IG', color: '#e87bc8', bg: 'rgba(232,123,200,.14)' },
  tt: { name: 'TikTok', short: 'TT', color: '#43e0d8', bg: 'rgba(67,224,216,.14)' },
  fb: { name: 'Facebook Reels', short: 'FB', color: '#6aa3f0', bg: 'rgba(106,163,240,.14)' },
  x: { name: 'X / Twitter', short: 'X', color: '#cdd0d6', bg: 'rgba(205,208,214,.12)' },
  th: { name: 'Threads', short: 'TH', color: '#9aa0a8', bg: 'rgba(154,160,168,.12)' },
  gt: { name: 'Google Trends', short: 'GT', color: '#a78bfa', bg: 'rgba(167,139,250,.14)' },
};
