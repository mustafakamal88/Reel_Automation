/**
 * src/lib/integrations/integrationApiClient.ts — FRONTEND SAFE.
 *
 * Frontend API client for integration management. Written as if calling real
 * backend HTTP endpoints. No server modules are imported here; all credential
 * handling happens server-side only.
 *
 * Production endpoints (replace mock responses with real fetch calls):
 *   GET  /api/integrations/status
 *   POST /api/integrations/:providerId/oauth/start
 *   POST /api/integrations/:providerId/disconnect
 *   POST /api/integrations/:providerId/refresh
 *   POST /api/integrations/:providerId/test
 *
 * Token boundary: this file and all its callers must NEVER handle or store:
 *   accessToken, refreshToken, clientSecret, apiKey, or raw provider credentials.
 * Only non-sensitive metadata (status, timestamps, scope names) crosses the
 * HTTP boundary and is safe to keep in React state.
 *
 * Backend reference architecture lives in server/integrations/ — those files
 * are never imported by frontend code.
 */

import type { IntegrationStatus, ConnectionStatusResponse } from './types';

export interface ConnectResult {
  success: boolean;
  status: IntegrationStatus;
  message: string;
  connectedAt: string | null;
  expiresAt: string | null;
  scopes: string[];
}

export interface ActionResult {
  success: boolean;
  status: IntegrationStatus;
  message: string;
}

// In-memory connection state for demo mode — resets on page reload.
// Production: this state is derived from GET /api/integrations/status responses.
interface MockConnection {
  status: 'connected' | 'needs_attention' | 'error';
  connectedAt: string;
  expiresAt: string;
  scopes: string[];
}
const _mockState = new Map<string, MockConnection>();

function _delay(ms: number): Promise<void> {
  return new Promise(r => setTimeout(r, ms));
}

export const integrationApiClient = {
  /**
   * GET /api/integrations/status
   *
   * Returns token-free connection status for the given providers (or all
   * known providers when called with no argument). Only status, timestamps,
   * and scope names are included — never token values.
   */
  async getConnectionStatus(providerIds?: string[]): Promise<ConnectionStatusResponse[]> {
    // Production: return fetch('/api/integrations/status').then(r => r.json());
    await _delay(150);
    const ids = providerIds ?? [..._mockState.keys()];
    return ids.map(id => {
      const rec = _mockState.get(id);
      if (!rec) return { success: true, providerId: id, status: 'not_connected' as const };
      return {
        success: true,
        providerId: id,
        status: rec.status,
        connectedAt: rec.connectedAt,
        expiresAt: rec.expiresAt,
        scopes: rec.scopes,
      };
    });
  },

  /**
   * POST /api/integrations/:providerId/oauth/start
   *
   * Starts the OAuth 2.0 flow. In production the backend generates the PKCE
   * code verifier and state, returns an authorization URL, and the browser
   * redirects to the provider's consent screen. The backend handles the
   * callback, exchanges the code for tokens, encrypts them, and stores them
   * server-side only.
   *
   * In demo mode the full consent flow is simulated immediately; no real
   * token exchange occurs.
   */
  async startOAuth(providerId: string): Promise<ConnectResult> {
    // Production:
    // const { authorizeUrl } = await fetch(
    //   `/api/integrations/${providerId}/oauth/start`, { method: 'POST' }
    // ).then(r => r.json());
    // window.location.href = authorizeUrl; // redirect to provider consent
    // After redirect back, poll GET /api/integrations/status for updated state.
    await _delay(1000);
    const now = new Date();
    const expiresAt = new Date(now.getTime() + 3600 * 1000).toISOString();
    _mockState.set(providerId, {
      status: 'connected',
      connectedAt: now.toISOString(),
      expiresAt,
      scopes: [],
    });
    return {
      success: true,
      status: 'connected',
      message:
        `Connected (demo). Production: POST /api/integrations/${providerId}/oauth/start → ` +
        `browser redirects to provider consent page → backend exchanges authorization code → ` +
        `tokens encrypted and stored server-side only.`,
      connectedAt: now.toISOString(),
      expiresAt,
      scopes: [],
    };
  },

  /**
   * POST /api/integrations/:providerId/disconnect
   *
   * Asks the backend to revoke OAuth tokens via the provider's revocation
   * endpoint (RFC 7009) and delete the encrypted credential record.
   */
  async disconnectProvider(providerId: string): Promise<ActionResult> {
    // Production:
    // await fetch(`/api/integrations/${providerId}/disconnect`, { method: 'POST' });
    await _delay(400);
    _mockState.delete(providerId);
    return {
      success: true,
      status: 'not_connected',
      message:
        `Disconnected (demo). Production: POST /api/integrations/${providerId}/disconnect → ` +
        `backend revokes OAuth tokens and deletes encrypted credentials.`,
    };
  },

  /**
   * POST /api/integrations/:providerId/refresh
   *
   * Asks the backend to perform a refresh_token grant and atomically rotate
   * both tokens. The new tokens are stored server-side only; only the updated
   * status and new expiry timestamp are returned.
   */
  async refreshConnection(providerId: string): Promise<ActionResult> {
    // Production:
    // const updated = await fetch(
    //   `/api/integrations/${providerId}/refresh`, { method: 'POST' }
    // ).then(r => r.json());
    await _delay(800);
    const existing = _mockState.get(providerId);
    if (!existing) {
      return {
        success: false,
        status: 'not_connected',
        message: `Provider not connected — connect first before refreshing.`,
      };
    }
    const newExpiry = new Date(Date.now() + 3600 * 1000).toISOString();
    _mockState.set(providerId, { ...existing, expiresAt: newExpiry, status: 'connected' });
    return {
      success: true,
      status: 'connected',
      message:
        `Token refreshed (demo). Production: POST /api/integrations/${providerId}/refresh → ` +
        `backend exchanges refresh token, rotates credentials server-side. New expiry: ${newExpiry}.`,
    };
  },

  /**
   * POST /api/integrations/:providerId/test
   *
   * Asks the backend to make a lightweight API call to the provider (e.g.
   * GET /me) using the stored token to confirm credentials are still valid.
   * The token itself is never returned.
   */
  async testConnection(providerId: string): Promise<ActionResult> {
    // Production:
    // const result = await fetch(
    //   `/api/integrations/${providerId}/test`, { method: 'POST' }
    // ).then(r => r.json());
    await _delay(450);
    const existing = _mockState.get(providerId);
    if (!existing) {
      return {
        success: false,
        status: 'not_connected',
        message: `Provider not connected — connect first before testing.`,
      };
    }
    return {
      success: true,
      status: 'connected',
      message:
        `Test passed (demo). Production: POST /api/integrations/${providerId}/test → ` +
        `backend validates live credentials against provider API.`,
    };
  },
};
