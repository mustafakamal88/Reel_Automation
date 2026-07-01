import type { Provider } from './index';

export const facebookProvider: Provider = {
  id: 'fb',
  name: 'Facebook Reels',
  async fetchSignals() {
    return [];
  },
};
