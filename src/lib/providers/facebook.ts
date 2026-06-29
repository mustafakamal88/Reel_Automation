// Facebook Reels mock provider.
// TODO: Replace with Meta Graph API — GET /{page-id}/videos?fields=title,description,views,created_time
//       for Facebook Reels velocity data. Requires page_read_engagement permission.
//       Public hashtag search not directly available; combine with CrowdTangle API if licensed.

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const facebookProvider: Provider = {
  id: 'fb',
  name: 'Facebook Reels',
  async fetchSignals() {
    // TODO: call Meta Graph API for Reels data and normalise to Signal shape
    return SIGNALS.filter(s => s.src === 'fb');
  },
};
