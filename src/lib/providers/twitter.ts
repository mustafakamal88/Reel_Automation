import type { Provider } from './index';

export const twitterProvider: Provider = {
  id: 'x',
  name: 'X / Twitter',
  async fetchSignals() {
    return [];
  },
};
