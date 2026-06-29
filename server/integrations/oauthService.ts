/**
 * server/integrations/oauthService.ts — BACKEND ONLY.
 *
 * OAuth 2.0 flow manager and connection lifecycle service. This file is
 * reference architecture for the production server. It must never be imported
 * by frontend (src/) code. In production this module runs on a Node/Express/
 * Fastify server and is called only by HTTP route handlers.
 *
 * HTTP boundary (frontend calls these endpoints — never this module directly):
 *   POST /api/integrations/:provider/oauth/start   → startOAuth()
 *   GET  /api/integrations/callback                → handleOAuthCallback()
 *   POST /api/integrations/:provider/disconnect    → disconnectProvider()
 *   GET  /api/integrations/status                  → getConnectionStatus()
 *   POST /api/integrations/:provider/refresh       → refreshConnection()
 *
 * Security properties:
 *   - OAuth client_secret is stored only in server environment variables (never VITE_).
 *   - PKCE (S256) is used for all OAuth 2.0 flows to prevent code interception attacks.
 *   - State parameter is stored server-side (Redis with TTL) for CSRF protection.
 *   - Token values are encrypted before storage and never returned to the frontend.
 *   - Refresh tokens are rotated on every use (RFC 6749 §10.4).
 *
 * Raw credential values (access_token, refresh_token, client_secret) must never
 * appear in any HTTP response or be passed to the frontend in any form.
 */

import { tokenVault } from './tokenVault';
import type { OAuthInitResult, ConnectionStatusResponse } from './types';

// In production: store in Redis with a 10-minute TTL to prevent replay attacks.
interface PendingOAuthState {
  providerId: string;
  codeVerifier: string; // PKCE verifier — kept server-side; never sent to the client
  startedAt: number;
}
const _pendingStates = new Map<string, PendingOAuthState>();
const OAUTH_STATE_TTL_MS = 10 * 60 * 1000; // 10 minutes

function _generateState(): string {
  // PRODUCTION: crypto.randomBytes(32).toString('hex')
  return `state_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function _generateCodeVerifier(): string {
  // PRODUCTION: crypto.randomBytes(43).toString('base64url') — must be 43-128 chars
  return `verifier_${Math.random().toString(36).slice(2)}${Math.random().toString(36).slice(2)}`;
}

// Authorization URLs by provider. Client IDs come from server env (never VITE_ vars).
const PROVIDER_AUTH_URLS: Record<string, string> = {
  'google-trends': '',  // server adapter — no user OAuth consent required
  'youtube':       'https://accounts.google.com/o/oauth2/v2/auth',
  'tiktok':        'https://www.tiktok.com/v2/auth/authorize',
  'instagram':     'https://api.instagram.com/oauth/authorize',
  'facebook':      'https://www.facebook.com/v19.0/dialog/oauth',
  'twitter':       'https://twitter.com/i/oauth2/authorize',
  'threads':       'https://www.threads.net/oauth/authorize',
};

// Scopes per provider (read-only / analytics scopes for data ingestion)
const PROVIDER_SCOPES: Record<string, string> = {
  'google-trends': 'trends:read',
  'youtube':       'https://www.googleapis.com/auth/youtube.readonly',
  'tiktok':        'research.data.basic research.adlib.basic',
  'instagram':     'instagram_basic pages_read_engagement',
  'facebook':      'public_content',
  'twitter':       'tweet.read users.read',
  'threads':       'threads_basic',
  // Publishing accounts
  'pub-youtube':   'https://www.googleapis.com/auth/youtube.upload',
  'pub-tiktok':    'video.upload',
  'pub-instagram': 'instagram_content_publish',
  'pub-facebook':  'pages_manage_posts pages_read_engagement',
  'pub-twitter':   'tweet.write users.read',
  'pub-threads':   'threads_content_publish',
};

function _delay(ms: number): Promise<void> {
  return new Promise(r => setTimeout(r, ms));
}

export const oauthService = {
  /**
   * Step 1 of the OAuth flow: build the authorization URL and store the PKCE state.
   *
   * The frontend redirects (or opens a popup) to authorizeUrl. After user consent the
   * provider redirects to the backend callback URL with ?code=...&state=...
   *
   * Production additions:
   *   - Compute code_challenge = BASE64URL(SHA256(codeVerifier)) for PKCE S256
   *   - Append code_challenge + code_challenge_method=S256 to authorizeUrl
   *   - Store { providerId, codeVerifier } in Redis with key=state, TTL=10min
   *   - Use the real CLIENT_ID from process.env (never VITE_CLIENT_ID)
   */
  async startOAuth(providerId: string): Promise<OAuthInitResult> {
    await _delay(300);
    const state = _generateState();
    const codeVerifier = _generateCodeVerifier();
    _pendingStates.set(state, { providerId, codeVerifier, startedAt: Date.now() });

    const baseUrl = PROVIDER_AUTH_URLS[providerId] ?? '';
    const scope   = PROVIDER_SCOPES[providerId] ?? '';

    // PRODUCTION: append &code_challenge=<S256(codeVerifier)>&code_challenge_method=S256
    const authorizeUrl = baseUrl
      ? `${baseUrl}?client_id=BACKEND_CLIENT_ID_FROM_ENV&redirect_uri=https://your-backend.com/api/integrations/callback&response_type=code&scope=${encodeURIComponent(scope)}&state=${state}`
      : '';

    return { authorizeUrl, state };
  },

  /**
   * Step 2: Handle the provider callback — exchange code for tokens, encrypt, and store.
   *
   * Production:
   *   1. Validate state (CSRF) — reject if not found or expired in Redis.
   *   2. Retrieve and delete the pending state (one-time use).
   *   3. POST to the provider's token endpoint with:
   *        client_id, client_secret (from env), code, code_verifier, redirect_uri, grant_type=authorization_code
   *   4. Validate the token response (check error field, check id_token if OIDC).
   *   5. Encrypt and store tokens via tokenVault.store().
   *   6. Return only connection status — never the token values.
   */
  async handleOAuthCallback(
    providerId: string,
    _code: string,
    state: string,
  ): Promise<ConnectionStatusResponse> {
    await _delay(700);

    // Validate CSRF state
    const pending = _pendingStates.get(state);
    if (!pending) {
      return { success: false, providerId, status: 'error', error: 'OAuth state not found or expired (possible CSRF)' };
    }
    if (pending.providerId !== providerId) {
      return { success: false, providerId, status: 'error', error: 'OAuth state provider mismatch' };
    }
    if (Date.now() - pending.startedAt > OAUTH_STATE_TTL_MS) {
      _pendingStates.delete(state);
      return { success: false, providerId, status: 'error', error: 'OAuth state expired — please reconnect' };
    }
    _pendingStates.delete(state); // one-time use

    // PRODUCTION: POST to provider token endpoint here.
    // const tokens = await fetch(PROVIDER_TOKEN_URL, { method: 'POST', body: new URLSearchParams({
    //   client_id: process.env.PROVIDER_CLIENT_ID,
    //   client_secret: process.env.PROVIDER_CLIENT_SECRET, // never VITE_
    //   code: _code,
    //   code_verifier: pending.codeVerifier,
    //   redirect_uri: REDIRECT_URI,
    //   grant_type: 'authorization_code',
    // })}).then(r => r.json());

    // Mock token values — backend-only; never returned to the frontend.
    const mockAccessToken  = `access_${providerId}_${Date.now()}`;
    const mockRefreshToken = `refresh_${providerId}_${Date.now()}`;
    const mockExpiresIn    = 3600; // 1 hour

    tokenVault.store(providerId, mockAccessToken, mockRefreshToken, mockExpiresIn, PROVIDER_SCOPES[providerId] ?? '');

    const meta = tokenVault.getMetadata(providerId);
    return {
      success: true,
      providerId,
      status: 'connected',
      connectedAt: new Date(meta.issuedAt!).toISOString(),
      expiresAt: meta.expiresAt ? new Date(meta.expiresAt).toISOString() : null,
      scopes: (meta.scope ?? '').split(/[\s,]+/).filter(Boolean),
    };
  },

  /**
   * Revoke OAuth tokens and remove credentials from storage.
   *
   * Production:
   *   1. Retrieve the encrypted refresh token.
   *   2. Call the provider's token revocation endpoint (RFC 7009).
   *   3. Delete the record from the vault.
   *   4. Write an audit log event.
   */
  async disconnectProvider(providerId: string): Promise<ConnectionStatusResponse> {
    await _delay(400);
    // PRODUCTION: call provider revocation endpoint before vault.revoke()
    tokenVault.revoke(providerId);
    return { success: true, providerId, status: 'not_connected' };
  },

  /**
   * Return current connection status for one or more providers.
   * Safe to call from the frontend — only metadata is returned, never token values.
   *
   * Production: read from the vault (or a separate lightweight status cache)
   * to avoid decrypting tokens on every status poll.
   */
  async getConnectionStatus(providerIds?: string[]): Promise<ConnectionStatusResponse[]> {
    await _delay(150);
    const ids = providerIds ?? Object.keys(PROVIDER_AUTH_URLS);
    return ids.map(id => {
      if (!tokenVault.hasCredentials(id)) {
        return { success: true, providerId: id, status: 'not_connected' as const };
      }
      const meta    = tokenVault.getMetadata(id);
      const expired  = tokenVault.isExpired(id);
      const expiring = tokenVault.isExpiringSoon(id, 600); // warn 10 min before expiry

      let status: ConnectionStatusResponse['status'] = 'connected';
      if (expired) status = 'error';
      else if (expiring) status = 'needs_attention';

      return {
        success: true,
        providerId: id,
        status,
        connectedAt: meta.issuedAt  ? new Date(meta.issuedAt).toISOString()  : null,
        expiresAt:   meta.expiresAt ? new Date(meta.expiresAt).toISOString() : null,
        scopes: (meta.scope ?? '').split(/[\s,]+/).filter(Boolean),
      };
    });
  },

  /**
   * Use the stored refresh token to obtain a new access token.
   *
   * Production:
   *   1. Retrieve the encrypted refresh token from the vault.
   *   2. POST to the provider's token endpoint with grant_type=refresh_token.
   *   3. Rotate credentials via tokenVault.rotate() (replace both tokens atomically).
   *   4. Return only the updated status — never the new token values.
   *
   * RFC 6749 §10.4: providers may issue a new refresh token on each refresh grant.
   * Always store the new refresh token even if the provider returns the same value.
   */
  async refreshConnection(providerId: string): Promise<ConnectionStatusResponse> {
    await _delay(800);

    if (!tokenVault.hasCredentials(providerId)) {
      return { success: false, providerId, status: 'not_connected', error: 'No credentials — provider not connected' };
    }

    // PRODUCTION: const oldRefresh = tokenVault._getAccessToken(providerId + '_refresh');
    // const response = await fetch(PROVIDER_TOKEN_URL, { method: 'POST', body: new URLSearchParams({
    //   client_id: process.env.PROVIDER_CLIENT_ID,
    //   client_secret: process.env.PROVIDER_CLIENT_SECRET,
    //   refresh_token: oldRefresh,
    //   grant_type: 'refresh_token',
    // })}).then(r => r.json());

    // Mock rotated tokens — backend-only; never returned to the frontend.
    const newAccess  = `access_refreshed_${providerId}_${Date.now()}`;
    const newRefresh = `refresh_rotated_${providerId}_${Date.now()}`;
    tokenVault.rotate(providerId, newAccess, newRefresh, 3600);

    const meta = tokenVault.getMetadata(providerId);
    return {
      success: true,
      providerId,
      status: 'connected',
      expiresAt: meta.expiresAt ? new Date(meta.expiresAt).toISOString() : null,
      scopes: (meta.scope ?? '').split(/[\s,]+/).filter(Boolean),
    };
  },
};
