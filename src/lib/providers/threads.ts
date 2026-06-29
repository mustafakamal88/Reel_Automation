// Threads mock provider.
// TODO: Replace with Meta Threads API (developers.facebook.com/docs/threads) —
//       GET /threads/search (keyword search) once it becomes available in v2+.
//       Currently limited; monitor Meta developer changelog for hashtag/trend endpoints.

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const threadsProvider: Provider = {
  id: 'th',
  name: 'Threads',
  async fetchSignals() {
    // TODO: call Threads API and normalise to Signal shape
    return SIGNALS.filter(s => s.src === 'th');
  },
};
