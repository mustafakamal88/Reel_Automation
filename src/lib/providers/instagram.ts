// Instagram mock provider.
// TODO: Replace with Meta Graph API — GET /{hashtag-id}/recent_media or top_media.
//       Requires Facebook Developer App with instagram_basic or pages_read_engagement permissions.
//       Hashtag ID lookup: GET /ig_hashtag_search?q={hashtag}

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const instagramProvider: Provider = {
  id: 'ig',
  name: 'Instagram Reels',
  async fetchSignals() {
    // TODO: call Meta Graph API and normalise to Signal shape
    return SIGNALS.filter(s => s.src === 'ig');
  },
};
