import type { Provider } from './index';

export const instagramProvider: Provider = {
  id: 'ig',
  name: 'Instagram Reels',
  async fetchSignals() {
    return [];
  },
};
