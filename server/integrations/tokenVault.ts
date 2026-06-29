/**
 * server/integrations/tokenVault.ts — BACKEND ONLY.
 *
 * Mock encrypted token vault. This file is reference architecture for the
 * production server. It must never be imported by frontend (src/) code.
 *
 * Security architecture (production replacement points marked inline):
 *   - Tokens are never returned to frontend callers — only non-sensitive metadata is exported.
 *   - getMetadata() is the only safe-to-surface function; it returns timestamps and scope strings only.
 *   - Encryption: replace _mockEncrypt/_mockDecrypt with AES-256-GCM + KMS-managed data key.
 *   - Storage: replace the in-memory Map with a Postgres/DynamoDB table with encrypted columns,
 *     or delegate entirely to AWS Secrets Manager / HashiCorp Vault / GCP Secret Manager.
 *   - Audit: every store/rotate/revoke call must emit to an append-only audit log (no token values).
 *   - Rotation: schedule automatic re-encryption when the KMS key is rotated (separate cron job).
 *
 * Frontend must never import this module. Only oauthService and providerCallService may call it.
 */

// Internal token record shape. Never exported; never returned to any caller.
interface TokenRecord {
  providerId: string;
  // In production: AES-256-GCM ciphertext envelope — { iv, ciphertext, authTag, keyId }.
  // keyId references the KMS data key used to encrypt. The raw value is never stored in plaintext.
  encryptedAccessToken: string;
  encryptedRefreshToken: string | null;
  tokenType: string;
  scope: string;         // space/comma-separated scope names — not sensitive
  issuedAt: number;      // ms since epoch
  expiresAt: number | null; // ms since epoch; null = non-expiring (API keys)
}

// In production: replace with a persistent, encrypted database table.
const _vault = new Map<string, TokenRecord>();

// Production: call KMS.GenerateDataKey() to get a plaintext + ciphertext data key pair.
// Encrypt the token with the plaintext key (AES-256-GCM), then store the ciphertext key blob
// alongside the encrypted token. The plaintext key is never persisted.
function _mockEncrypt(plaintext: string): string {
  // REPLACE: AES-256-GCM encryption using a KMS data key
  return `enc::${btoa(unescape(encodeURIComponent(plaintext)))}`;
}

// Production: fetch the KMS data key via the stored keyId, decrypt to plaintext key,
// then use it to AES-256-GCM-decrypt the ciphertext blob.
function _mockDecrypt(ciphertext: string): string {
  // REPLACE: AES-256-GCM decryption
  if (!ciphertext.startsWith('enc::')) throw new Error('vault: invalid ciphertext format');
  return decodeURIComponent(escape(atob(ciphertext.slice(5))));
}

export interface TokenMetadata {
  hasToken: boolean;
  issuedAt: number | null;
  expiresAt: number | null;
  scope: string | null;    // scope names are not sensitive
  tokenType: string | null;
}

export const tokenVault = {
  /**
   * Encrypt and store credentials for a provider.
   * Called only by oauthService after a successful token exchange.
   * Production: write audit log event with providerId, timestamp, scope — never the token value.
   */
  store(
    providerId: string,
    accessToken: string,
    refreshToken: string | null,
    expiresInSeconds: number | null,
    scope: string,
    tokenType = 'Bearer',
  ): void {
    _vault.set(providerId, {
      providerId,
      encryptedAccessToken: _mockEncrypt(accessToken),
      encryptedRefreshToken: refreshToken ? _mockEncrypt(refreshToken) : null,
      tokenType,
      scope,
      issuedAt: Date.now(),
      expiresAt: expiresInSeconds != null ? Date.now() + expiresInSeconds * 1000 : null,
    });
    // PRODUCTION: await auditLog.write({ event: 'token.stored', providerId, scope, issuedAt });
  },

  /**
   * Internal backend use only — returns the decrypted access token for outbound API calls.
   * This function must NEVER be called from frontend code or returned in an API response.
   * The underscore prefix signals that callers outside this module should treat it as private.
   */
  _getAccessToken(providerId: string): string | null {
    const rec = _vault.get(providerId);
    return rec ? _mockDecrypt(rec.encryptedAccessToken) : null;
  },

  /** Whether credentials exist for this provider. Safe to expose. */
  hasCredentials(providerId: string): boolean {
    return _vault.has(providerId);
  },

  /** Whether the stored access token has passed its expiry timestamp. */
  isExpired(providerId: string): boolean {
    const rec = _vault.get(providerId);
    if (!rec || rec.expiresAt == null) return false;
    return Date.now() >= rec.expiresAt;
  },

  /** Whether the access token will expire within the given window (default: 5 minutes). */
  isExpiringSoon(providerId: string, withinSeconds = 300): boolean {
    const rec = _vault.get(providerId);
    if (!rec || rec.expiresAt == null) return false;
    return Date.now() >= rec.expiresAt - withinSeconds * 1000;
  },

  /**
   * Returns non-sensitive metadata about stored credentials.
   * Safe to surface to the frontend — no token values are included.
   */
  getMetadata(providerId: string): TokenMetadata {
    const rec = _vault.get(providerId);
    if (!rec) {
      return { hasToken: false, issuedAt: null, expiresAt: null, scope: null, tokenType: null };
    }
    return {
      hasToken: true,
      issuedAt: rec.issuedAt,
      expiresAt: rec.expiresAt,
      scope: rec.scope,
      tokenType: rec.tokenType,
    };
  },

  /**
   * Revoke and permanently delete credentials for a provider.
   * Production:
   *   1. Call the provider's token revocation endpoint with the encrypted refresh token.
   *   2. Delete the database row.
   *   3. Write an audit log event.
   */
  revoke(providerId: string): boolean {
    // PRODUCTION: await providerRevocationEndpoints[providerId](refreshToken);
    // PRODUCTION: await auditLog.write({ event: 'token.revoked', providerId });
    return _vault.delete(providerId);
  },

  /**
   * Atomically replace stored credentials with a new token pair (after a refresh grant).
   * Production: write the new record, verify it's readable, then delete the old one.
   * Use KMS key rotation on a separate schedule to re-encrypt stored tokens periodically.
   */
  rotate(
    providerId: string,
    newAccessToken: string,
    newRefreshToken: string | null,
    expiresInSeconds: number | null,
  ): void {
    const existing = _vault.get(providerId);
    // PRODUCTION: atomic swap — write new, verify, delete old; never leave a gap
    this.store(
      providerId,
      newAccessToken,
      newRefreshToken,
      expiresInSeconds,
      existing?.scope ?? '',
      existing?.tokenType ?? 'Bearer',
    );
    // PRODUCTION: await auditLog.write({ event: 'token.rotated', providerId });
  },
};
