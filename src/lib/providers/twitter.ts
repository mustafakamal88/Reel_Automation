// X / Twitter mock provider.
// TODO: Replace with X API v2 — GET /2/trends/by/woeid/{id} (Basic or Pro tier required).
//       Alternatively: GET /2/tweets/search/recent with query=#topic for keyword volume.
//       Bearer token required; Basic plan: 10k tweets/month.

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const twitterProvider: Provider = {
  id: 'x',
  name: 'X / Twitter',
  async fetchSignals() {
    // TODO: call X API v2 and normalise to Signal shape
    return SIGNALS.filter(s => s.src === 'x');
  },
};
