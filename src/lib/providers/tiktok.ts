import type { Provider } from './index';

export const tiktokProvider: Provider = {
  id: 'tt',
  name: 'TikTok',
  async fetchSignals() {
    return [];
  },
};
