/**
 * server/integrations/providerCallService.ts — BACKEND ONLY.
 *
 * Secure provider API call service. This file is reference architecture for
 * the production server. It must never be imported by frontend (src/) code;
 * in production it is called only by backend route handlers.
 *
 * Pattern per provider:
 *   1. Retrieve the decrypted access token via tokenVault._getAccessToken().
 *   2. Set it as `Authorization: Bearer <token>` (or the provider's required header format).
 *   3. Handle 401 → trigger oauthService.refreshConnection() and retry once.
 *   4. Handle 429 → exponential backoff with jitter.
 *   5. Handle 5xx → retry with backoff; alert on-call after N failures.
 *   6. Return normalized data; never propagate raw token values in any response.
 *
 * Raw credential values (access_token, refresh_token, client_secret) must never
 * appear in any HTTP response or be passed to the frontend in any form.
 */

import { tokenVault } from './tokenVault';

export interface ProviderCallResult<T = unknown> {
  providerId: string;
  success: boolean;
  data?: T;
  error?: string;
  rateLimitRemaining?: number;
  retryAfterMs?: number;
}

// Mock trend data returned in demo mode.
// Production: replace each entry with a real API response normalizer.
const _MOCK_TRENDS: Record<string, unknown> = {
  'google-trends': {
    trending: ['AI tools', 'short-form video', 'passive income'],
    region: 'US',
    fetchedAt: new Date().toISOString(),
  },
  'youtube': {
    trending: ['YouTube Shorts automation', 'AI video tools', 'creator workflow'],
    fetchedAt: new Date().toISOString(),
  },
  'tiktok': {
    trending: ['AI tools', 'day in my life', 'coding tips'],
    fetchedAt: new Date().toISOString(),
  },
  'instagram': {
    trending: ['#reels', '#contentcreator', '#trending'],
    fetchedAt: new Date().toISOString(),
  },
  'facebook': {
    trending: ['AI videos', 'tutorial reels'],
    fetchedAt: new Date().toISOString(),
  },
  'twitter': {
    trending: ['#AItools', '#buildinpublic', '#solodev'],
    fetchedAt: new Date().toISOString(),
  },
  'threads': {
    trending: ['creator economy', 'AI side projects'],
    fetchedAt: new Date().toISOString(),
  },
};

// Lightweight verification endpoint per provider (used by verifyCredentials).
// Production: these make a minimal API call to confirm token validity.
const _VERIFY_ENDPOINTS: Record<string, string> = {
  'tiktok':    'GET /v2/user/info/',
  'instagram': 'GET /me?fields=id',
  'facebook':  'GET /me?fields=id',
  'youtube':   'GET /youtube/v3/channels?part=id&mine=true',
  'twitter':   'GET /2/users/me',
  'threads':   'GET /me?fields=id',
};

function _delay(ms: number): Promise<void> {
  return new Promise(r => setTimeout(r, ms));
}

export const providerCallService = {
  /**
   * Fetch trend/signal data from an external provider API.
   *
   * Production:
   *   const token = tokenVault._getAccessToken(providerId);
   *   const response = await fetch(PROVIDER_ENDPOINTS[providerId], {
   *     headers: { Authorization: `Bearer ${token}` },
   *   });
   *   if (response.status === 401) {
   *     await oauthService.refreshConnection(providerId);
   *     return this.fetchTrends(providerId); // retry once
   *   }
   *   if (response.status === 429) {
   *     return { providerId, success: false, error: 'Rate limited', retryAfterMs: ... };
   *   }
   *   const data = await response.json();
   *   return { providerId, success: true, data: normalize(data) };
   */
  async fetchTrends<T = unknown>(providerId: string): Promise<ProviderCallResult<T>> {
    await _delay(500);

    // Google Trends uses a server adapter — no OAuth token required
    if (providerId !== 'google-trends' && !tokenVault.hasCredentials(providerId)) {
      return { providerId, success: false, error: 'Provider not connected — no credentials in vault' };
    }

    if (tokenVault.isExpired(providerId)) {
      return { providerId, success: false, error: 'Access token expired — call refreshConnection() before retrying' };
    }

    // The access token is used server-side only; it is never returned to the caller.
    // PRODUCTION: const token = tokenVault._getAccessToken(providerId);
    const data = _MOCK_TRENDS[providerId] ?? { note: `No mock data for provider: ${providerId}` };
    return { providerId, success: true, data: data as T, rateLimitRemaining: 98 };
  },

  /**
   * Verify that stored credentials are still accepted by the provider.
   * Uses the cheapest available API call (typically a profile/me endpoint).
   *
   * Production: make a real HTTP GET to _VERIFY_ENDPOINTS[providerId] with the stored token.
   * Return success=false + error on 401/403 so the caller can trigger re-auth.
   */
  async verifyCredentials(providerId: string): Promise<ProviderCallResult<{ verified: boolean; endpoint: string }>> {
    await _delay(450);

    if (!tokenVault.hasCredentials(providerId)) {
      return { providerId, success: false, data: { verified: false, endpoint: '' }, error: 'No credentials stored' };
    }
    if (tokenVault.isExpired(providerId)) {
      return { providerId, success: false, data: { verified: false, endpoint: '' }, error: 'Token expired' };
    }

    // PRODUCTION: make real HTTP request using the stored token
    const endpoint = _VERIFY_ENDPOINTS[providerId] ?? 'N/A (server adapter)';
    return { providerId, success: true, data: { verified: true, endpoint }, rateLimitRemaining: 99 };
  },

  /**
   * Upload and publish (or schedule) a video to a connected platform.
   *
   * Production:
   *   1. Retrieve publish account token: tokenVault._getAccessToken(`pub-${platformId}`)
   *   2. Upload video binary to the platform's resumable upload endpoint.
   *   3. POST publish/schedule request with the returned upload ID.
   *   4. Return the platform's post ID for storage in the approvals database.
   *   5. Never return the access token in the response.
   *
   * Provider-specific notes:
   *   YouTube:   POST /upload/youtube/v3/videos (resumable) → POST /youtube/v3/videos
   *   TikTok:    POST /v2/post/publish/video/init → poll /v2/post/publish/status/fetch
   *   Instagram: POST /me/media → POST /me/media_publish
   *   Facebook:  POST /{page-id}/videos → returns video_id
   *   Twitter:   POST /2/tweets with media_ids
   *   Threads:   POST /me/threads → POST /me/threads_publish
   */
  async publishContent(
    platformId: string,
    _payload: {
      videoUrl: string;
      caption: string;
      hashtags: string[];
      scheduledAt?: string;
      privacy?: 'public' | 'private' | 'unlisted';
    },
  ): Promise<ProviderCallResult<{ postId: string; platformUrl?: string }>> {
    await _delay(1400);

    const vaultKey = `pub-${platformId}`;
    if (!tokenVault.hasCredentials(vaultKey)) {
      return { providerId: platformId, success: false, error: `Publishing account not connected: ${platformId}` };
    }
    if (tokenVault.isExpired(vaultKey)) {
      return { providerId: platformId, success: false, error: 'Publish token expired — reconnect the account' };
    }

    // PRODUCTION: upload video, then publish
    const mockPostId = `post_${platformId}_${Date.now()}`;
    return {
      providerId: platformId,
      success: true,
      data: {
        postId: mockPostId,
        platformUrl: `https://${platformId}.com/watch/${mockPostId}`,
      },
    };
  },

  /**
   * Run a lightweight test upload to verify publish permissions.
   * Uploads a 1-second silent black video and immediately deletes it.
   *
   * Production: generate a minimal valid video binary server-side,
   * upload it, confirm success, then delete via the platform's delete API.
   */
  async testPublishPermissions(
    platformId: string,
  ): Promise<ProviderCallResult<{ uploadOk: boolean; publishOk: boolean }>> {
    await _delay(1800);

    const vaultKey = `pub-${platformId}`;
    if (!tokenVault.hasCredentials(vaultKey)) {
      return { providerId: platformId, success: false, error: 'Account not connected' };
    }

    // PRODUCTION: upload 1s silent test video, verify, delete
    return { providerId: platformId, success: true, data: { uploadOk: true, publishOk: true } };
  },
};
