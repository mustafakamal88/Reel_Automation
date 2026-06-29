import type { IntegrationServiceResult } from './types';

function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export const mockIntegrationService = {
  async testConnection(id: string): Promise<IntegrationServiceResult> {
    await delay(1200);
    // Production: GET /api/integrations/test/:id
    // Backend makes a lightweight API call to verify credentials are still valid.
    // Returns 200 if live, 401 if token expired, 403 if revoked.
    return {
      success: true,
      status: 'demo',
      message: `Mock test passed (demo mode). Production: GET /api/integrations/test/${id} validates live credentials server-side.`,
    };
  },
};
