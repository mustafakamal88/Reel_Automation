import type { Signal } from '../../types';
import { googleTrendsProvider } from './google-trends';
import { youtubeProvider } from './youtube';
import { tiktokProvider } from './tiktok';
import { instagramProvider } from './instagram';
import { twitterProvider } from './twitter';
import { threadsProvider } from './threads';
import { facebookProvider } from './facebook';

export interface Provider {
  id: string;
  name: string;
  fetchSignals(): Promise<Signal[]>;
}

export const ALL_PROVIDERS: Provider[] = [
  googleTrendsProvider,
  youtubeProvider,
  tiktokProvider,
  instagramProvider,
  twitterProvider,
  threadsProvider,
  facebookProvider,
];

export async function fetchAllSignals(): Promise<Signal[]> {
  const results = await Promise.allSettled(ALL_PROVIDERS.map(p => p.fetchSignals()));
  return results
    .filter((r): r is PromiseFulfilledResult<Signal[]> => r.status === 'fulfilled')
    .flatMap(r => r.value);
}
