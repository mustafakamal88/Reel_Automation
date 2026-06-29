// Google Trends mock provider.
// TODO: Replace with real implementation using the unofficial google-trends-api npm package
//       or a server-side proxy that calls trends.google.com/trends/api/dailytrends.
// NOTE: Google does not offer an official public API — use a rate-limited proxy in production.

import type { Provider } from './index';
import { SIGNALS } from '../../data/signals';

export const googleTrendsProvider: Provider = {
  id: 'gt',
  name: 'Google Trends',
  async fetchSignals() {
    // TODO: await fetch('/api/google-trends?geo=US&hl=en-US') then normalise response
    return SIGNALS.filter(s => s.src === 'gt');
  },
};
