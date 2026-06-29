// YouTube mock provider.
// TODO: Replace with YouTube Data API v3 — GET /youtube/v3/videos?chart=mostPopular&videoCategoryId=&regionCode=US
//       Requires OAuth 2.0 or an API key scoped to YouTube Data API v3.
//       Quota unit cost: ~1 unit per call.

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const youtubeProvider: Provider = {
  id: 'yt',
  name: 'YouTube Shorts',
  async fetchSignals() {
    // TODO: call YouTube Data API and map to Signal shape
    return SIGNALS.filter(s => s.src === 'yt');
  },
};
