export type IntegrationStatus =
  | 'not_connected'
  | 'connected'
  | 'needs_attention'   // token valid but expiring soon; refresh recommended
  | 'error'
  | 'configuring';

export type AuthType = 'oauth2' | 'api_key' | 'server_adapter' | 'none';
export type SecretStorageMode = 'none' | 'backend_encrypted' | 'env_local_dev';
export type Privacy = 'public' | 'private' | 'unlisted';
export type TokenStatus = 'valid' | 'expiring' | 'expired' | 'none';
export type SettingsTab = 'general' | 'sources' | 'ai' | 'publishing' | 'security';
export type ProviderCategory = 'data_source' | 'ai_provider' | 'publishing';

export interface PermissionScope {
  name: string;
  description: string;
  required: boolean;
}

export interface IntegrationProvider {
  id: string;
  platformKey: string;
  name: string;
  category: ProviderCategory;
  status: IntegrationStatus;
  authType: AuthType;
  scopes: PermissionScope[];
  lastSync: string | null;
  secretStorageMode: SecretStorageMode;
  backendConnectEndpoint: string;
  backendStatusEndpoint: string;
}

export interface AiProviderConfig {
  id: string;
  name: string;
  category: ProviderCategory;
  status: IntegrationStatus;
  model: string;
  usageMode: 'api' | 'local';
  secretStorageMode: SecretStorageMode;
  backendEndpoint: string;
  contextWindow: string;
  costNote: string;
}

export interface PublishingAccount {
  id: string;
  platformKey: string;
  name: string;
  category: ProviderCategory;
  handle: string | null;
  status: IntegrationStatus;
  uploadPermission: boolean;
  publishPermission: boolean;
  tokenExpiry: string | null;
  tokenStatus: TokenStatus;
  defaultPrivacy: Privacy;
  defaultHashtags: string[];
  defaultPostTime: string | null;
  backendConnectEndpoint: string;
}

export interface IntegrationServiceResult {
  success: boolean;
  status: IntegrationStatus;
  message: string;
}

/**
 * Frontend-facing connection record returned by the backend.
 * Raw token values are never included — only non-sensitive metadata.
 */
export interface SecureConnection {
  providerId: string;
  status: IntegrationStatus;
  connectedAt: string | null;   // ISO 8601 timestamp
  expiresAt: string | null;     // ISO 8601 timestamp; null = non-expiring
  scopes: string[];             // scope names are not sensitive
  accountLabel: string | null;  // display name / handle for the connected account
  lastSyncAt: string | null;    // ISO 8601 timestamp of last successful data pull
}

/**
 * Returned by oauthService.startOAuth().
 * The frontend uses authorizeUrl to redirect/open the provider's consent screen.
 * The state value is used for CSRF validation in handleOAuthCallback.
 */
export interface OAuthInitResult {
  authorizeUrl: string;
  state: string;
}

/**
 * Returned by all oauthService methods that modify or query connection state.
 * Never includes token values — the frontend only needs the status and timestamps.
 */
export interface ConnectionStatusResponse {
  success: boolean;
  providerId: string;
  status: 'connected' | 'not_connected' | 'needs_attention' | 'error';
  connectedAt?: string | null;
  expiresAt?: string | null;
  scopes?: string[];
  error?: string;
}
