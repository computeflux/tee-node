// Auto-generated from ink! ABI
// DO NOT EDIT MANUALLY

export interface APIKeyInfo {
  Owner: UniAddr;
  KeyHash: number[];
  RateLimit: number;
  IsActive: boolean;
  CreatedAt: number;
}

export interface ModelInfo {
  Name: number[];
  Owner: UniAddr;
  ProviderID: number;
  PointsPer1K: number;
  IsActive: boolean;
  CreatedAt: number;
  InputModalities: number;
  ContextLength: number;
  PromptPricing: number;
  ProviderType: number;
  ModelID: number[];
  URI: number[];
  APIKeys: number[][];
}

export interface Option { None: null } | { Some: number }

export interface ProviderInfo {
  Owner: UniAddr;
  Name: number[];
  LogoURL: number[];
  Description: number[];
  ModelIds: number[];
  Region: number;
  Deposit: number;
  FeeRate: number;
  Priority: number;
  TotalRequests: number;
  SuccessRate: number;
  AvgLatency: number;
  UptimeSLA: number;
  IsActive: boolean;
  CreatedAt: number;
}

export interface UniAddr {
  T: number;
  V: number[];
}

export type GetCreditResponse = number;
export type GetModelCountResponse = number;
export type UpdateProviderResponse = void;
export type WithdrawResponse = void;
export type UpdateModelResponse = void;
export type SettleUsageResponse = void;
export type RegisterModelResponse = void;
export type ListProvidersResponse = ProviderInfo[];
export type GetProviderCountResponse = number;
export type InitResponse = void;
export type GetProviderByOwnerResponse = Option;
export type DeactivateModelResponse = void;
export type QueryModelsResponse = ModelInfo[];
export type RevokeAPIKeyResponse = void;
export type CreateProviderResponse = void;
export type RechargeResponse = void;
export type GetProviderCreditResponse = number;
export type GetProviderResponse = ProviderInfo;
export type ListActiveProvidersResponse = ProviderInfo[];
export type ListModelsResponse = ModelInfo[];
export type LookupAPIKeyResponse = APIKeyInfo;
export type GetModelResponse = ModelInfo;
export type ListActiveModelsResponse = ModelInfo[];
export type RegisterAPIKeyResponse = void;
