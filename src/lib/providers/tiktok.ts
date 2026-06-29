// TikTok mock provider.
// TODO: Replace with TikTok Research API (research.tiktok.com) — requires application approval.
//       Endpoint: POST /research/video/query/ with hashtag filters.
//       Alternative: TikTok Display API for public user content (requires approval).

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const tiktokProvider: Provider = {
  id: 'tt',
  name: 'TikTok',
  async fetchSignals() {
    // TODO: call TikTok Research API and normalise to Signal shape
    return SIGNALS.filter(s => s.src === 'tt');
  },
};
