import type { Provider } from './index';

export const youtubeProvider: Provider = {
  id: 'yt',
  name: 'YouTube Shorts',
  async fetchSignals() {
    return [];
  },
};
