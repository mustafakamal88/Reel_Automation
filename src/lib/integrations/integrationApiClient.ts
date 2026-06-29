/**
 * src/lib/integrations/integrationApiClient.ts
 *
 * Frontend API client for data-source integration management (trend data
 * providers, AI providers). All credential handling is server-side only.
 *
 * Token boundary: this file NEVER stores or handles OAuth tokens, refresh
 * tokens, client secrets, or API keys. Only status metadata is returned.
 *
 * Endpoints (Go backend, proxied by Vite in dev):
 *   GET  /platforms/connections
 *   GET  /oauth/{platform}/start             → returns authorize_url; caller redirects
 *   POST /platforms/{platform}/disconnect    → 501 until backend worker implemented
 *   POST /platforms/{platform}/refresh       → 501 until backend worker implemented
 *   POST /platforms/{platform}/test          → 501 until backend worker implemented
 */

import { ApiError, getPlatformConnections, getOAuthStartURL } from '../api/client';

export type IntegrationStatus =
  | 'not_connected'
  | 'connected'
  | 'configuring'
  | 'needs_attention'
  | 'error';

export interface ConnectionStatusResponse {
  success: boolean;
  providerId: string;
  status: IntegrationStatus;
  connectedAt?: string | null;
  expiresAt?: string | null;
  scopes?: string[];
}

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

export const integrationApiClient = {
  /**
   * GET /platforms/connections
   *
   * Returns token-free connection status for all publishable platforms.
   * Only status, timestamps, and scope names are included — never token values.
   */
  async getConnectionStatus(providerIds?: string[]): Promise<ConnectionStatusResponse[]> {
    try {
      const res = await getPlatformConnections();
      const all = res.platforms.map(p => ({
        success: true,
        providerId: p.platform,
        status: (p.status === 'credentials_missing' ? 'error' : p.status) as IntegrationStatus,
        scopes: p.scopes,
      }));
      if (providerIds) {
        return all.filter(p => providerIds.includes(p.providerId));
      }
      return all;
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Backend unreachable';
      return (providerIds ?? []).map(id => ({
        success: false,
        providerId: id,
        status: 'error' as IntegrationStatus,
        error: msg,
      }));
    }
  },

  /**
   * GET /oauth/{platform}/start
   *
   * Backend returns an authorization URL. The browser is redirected there so
   * the user can log in on the platform's own site. Backend handles the
   * callback, encrypts tokens, and stores them server-side only.
   *
   * If credentials are not configured, throws ApiError with isCredentialsMissing=true.
   */
  async startOAuth(providerId: string): Promise<ConnectResult> {
    try {
      const res = await getOAuthStartURL(providerId);
      window.location.href = res.authorize_url;
      return {
        success: true,
        status: 'configuring',
        message: `Redirecting to ${providerId} OAuth…`,
        connectedAt: null,
        expiresAt: null,
        scopes: [],
      };
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.isCredentialsMissing) {
          return {
            success: false,
            status: 'error',
            message: `Platform app credentials are missing. Add backend environment variables first.`,
            connectedAt: null,
            expiresAt: null,
            scopes: [],
          };
        }
        if (err.isBackendOffline) {
          return {
            success: false,
            status: 'error',
            message: `Backend offline — run: cd backend && go run ./cmd/api`,
            connectedAt: null,
            expiresAt: null,
            scopes: [],
          };
        }
        if (err.isNotImplemented) {
          return {
            success: false,
            status: 'error',
            message: `OAuth not yet wired on backend — PKCE + state storage required.`,
            connectedAt: null,
            expiresAt: null,
            scopes: [],
          };
        }
      }
      return {
        success: false,
        status: 'error',
        message: err instanceof Error ? err.message : 'Unknown error',
        connectedAt: null,
        expiresAt: null,
        scopes: [],
      };
    }
  },

  /**
   * POST /platforms/{platform}/disconnect
   *
   * Backend revokes the OAuth token and deletes the encrypted credential record.
   * Returns 501 until the backend worker is implemented.
   */
  async disconnectProvider(providerId: string): Promise<ActionResult> {
    try {
      await fetch(`/platforms/${providerId}/disconnect`, {
        method: 'POST',
        credentials: 'include',
      });
      return { success: true, status: 'not_connected', message: `Disconnected ${providerId}.` };
    } catch (err) {
      return {
        success: false,
        status: 'error',
        message: err instanceof Error ? err.message : 'Disconnect failed',
      };
    }
  },

  /**
   * POST /platforms/{platform}/refresh
   *
   * Backend exchanges the stored refresh token for a new access token.
   * Only an updated expiry timestamp is returned — never the new token itself.
   */
  async refreshConnection(providerId: string): Promise<ActionResult> {
    try {
      const res = await fetch(`/platforms/${providerId}/refresh`, {
        method: 'POST',
        credentials: 'include',
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` })) as { error?: string };
        return { success: false, status: 'error', message: body.error ?? `HTTP ${res.status}` };
      }
      return { success: true, status: 'connected', message: `Token refreshed for ${providerId}.` };
    } catch (err) {
      return {
        success: false,
        status: 'error',
        message: err instanceof Error ? err.message : 'Refresh failed',
      };
    }
  },

  /**
   * POST /platforms/{platform}/test
   *
   * Backend makes a lightweight API call to the provider to confirm the stored
   * credentials are still valid. The token is never returned.
   */
  async testConnection(providerId: string): Promise<ActionResult> {
    try {
      const res = await fetch(`/platforms/${providerId}/test`, {
        method: 'POST',
        credentials: 'include',
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` })) as { error?: string };
        return { success: false, status: 'error', message: body.error ?? `HTTP ${res.status}` };
      }
      return { success: true, status: 'connected', message: `Credential test passed for ${providerId}.` };
    } catch (err) {
      return {
        success: false,
        status: 'error',
        message: err instanceof Error ? err.message : 'Test failed',
      };
    }
  },
};
