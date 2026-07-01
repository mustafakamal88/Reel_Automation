import type { Provider } from './index';

export const threadsProvider: Provider = {
  id: 'th',
  name: 'Threads',
  async fetchSignals() {
    return [];
  },
};
