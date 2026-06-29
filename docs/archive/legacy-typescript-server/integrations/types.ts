/**
 * server/integrations/types.ts — backend-only reference architecture.
 *
 * Re-exports the frontend-safe shared types that cross the HTTP boundary
 * (status responses, OAuth init result) so server modules can import from
 * a single local path instead of reaching into src/.
 *
 * Server-internal types (TokenRecord, PendingOAuthState) are defined inline
 * in their respective modules and are never exported or surfaced to callers.
 *
 * NEVER export: accessToken, refreshToken, clientSecret, apiKey, or any
 * raw credential value. Only non-sensitive metadata crosses the HTTP boundary.
 */
export type {
  OAuthInitResult,
  ConnectionStatusResponse,
} from '../../src/lib/integrations/types';
