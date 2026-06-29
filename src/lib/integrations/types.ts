export type IntegrationStatus = 'demo' | 'not_connected' | 'connected' | 'error' | 'configuring';
export type AuthType = 'oauth2' | 'api_key' | 'server_adapter' | 'none';
export type SecretStorageMode = 'none' | 'backend_encrypted' | 'env_local_dev';
export type Privacy = 'public' | 'private' | 'unlisted';
export type TokenStatus = 'valid' | 'expiring' | 'expired' | 'none';
export type SettingsTab = 'general' | 'sources' | 'ai' | 'publishing' | 'security';

export interface PermissionScope {
  name: string;
  description: string;
  required: boolean;
}

export interface IntegrationProvider {
  id: string;
  platformKey: string;
  name: string;
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
  status: IntegrationStatus;
  model: string;
  usageMode: 'demo' | 'api' | 'local';
  secretStorageMode: SecretStorageMode;
  backendEndpoint: string;
  contextWindow: string;
  costNote: string;
}

export interface PublishingAccount {
  id: string;
  platformKey: string;
  name: string;
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
