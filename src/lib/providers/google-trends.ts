import type { Provider } from './index';

export const googleTrendsProvider: Provider = {
  id: 'gt',
  name: 'Google Trends',
  async fetchSignals() {
    return [];
  },
};
