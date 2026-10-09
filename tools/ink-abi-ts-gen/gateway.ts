// Auto-generated from ink! ABI
// DO NOT EDIT MANUALLY

export interface APIKeyInfoV2 {
  OwnerKey: string;
  Prefix: string;
  Owner: UniAddr;
  KeyHash: string;
  RateLimit: number;
  IsActive: boolean;
  CreatedAt: string;
}

export interface ModelAudit {
  ModelID: string;
  TriggerBy: number;
  Passed: boolean;
  Detail: string;
  CreatedAt: string;
}

export interface ModelInfoV2 {
  Name: string;
  Owner: UniAddr;
  ProviderID: string;
  InputModalities: number;
  ContextLength: string;
  InPrice: string;
  CachedPrice: string;
  OutPrice: string;
  Thinking: boolean;
  Meta: string;
  ModelType: number;
  RouteID: string;
  URIS: string;
  APIKeys: string[];
  MaxConcurrency: number;
  Balance: string;
  ReputationScore: number;
  TotalAudits: string;
  PassedAudits: string;
  IsActive: boolean;
  CreatedAt: string;
}

export interface ModelInfoWithID {
  ID: string;
  Data: ModelInfoV2;
}

export interface ModelInfoWithProvider {
  ID: string;
  Model: ModelInfoV2;
  Provider: ProviderInfo;
}

export interface ModelReport {
  ModelID: string;
  Reporter: UniAddr;
  Reason: string;
  Resolved: boolean;
  CreatedAt: string;
}

export interface ModelRoute {
  Name: string;
  RouteType: number;
  InPrice: string;
  CachedPrice: string;
  OutPrice: string;
  ModelIds: string[];
  IsActive: boolean;
  IsOfficial: boolean;
  CreatedAt: string;
}

export interface ModelRouteWithID {
  ID: string;
  Data: ModelRoute;
}

export interface ProviderInfo {
  Owner: UniAddr;
  Name: string;
  LogoURL: string;
  Description: string;
  ModelIds: string[];
  Region: number;
  Deposit: string;
  FeeRate: number;
  Priority: number;
  TotalRequests: string;
  SuccessRate: number;
  AvgLatency: number;
  UptimeSLA: number;
  IsActive: boolean;
  CreatedAt: string;
}

export interface ProviderInfoWithID {
  ID: string;
  Data: ProviderInfo;
}

export interface ProviderSettlementRecord {
  User: UniAddr;
  ModelID: string;
  TokenIn: string;
  TokenOut: string;
  CachedToken: string;
  Cost: string;
  Count: number;
  Time: string;
}

export interface RechargeRecord {
  User: UniAddr;
  Amount: string;
  Time: string;
}

export interface TransferRecord {
  From: UniAddr;
  To: UniAddr;
  Amount: string;
  Time: string;
}

export interface U256 {
  _7: string[];
}

export interface UniAddr {
  T: number;
  V: string;
}

export interface UserSettlementRecord {
  ProviderID: string;
  ModelID: string;
  TokenIn: string;
  TokenOut: string;
  CachedToken: string;
  Cost: string;
  Count: number;
  Time: string;
}

export interface WithdrawApply {
  ChainID: number;
  Contract: string;
  User: UniAddr;
  Credit: U256;
  Nonce: string;
  Status: number;
}

export type AuditModelResponse = void;
export type GetCreditResponse = string;
export type GetModelCountResponse = string;
export type UpdateModelAPIKeyRefResponse = void;
export type ApplyWithdrawResponse = void;
export type UpdateProviderResponse = void;
export type UpdateModelResponse = void;
export type DeleteModelAPIKeyRefResponse = void;
export type RegisterModelResponse = void;
export type ReportProviderCallResponse = void;
export type UpdateAPIKeyResponse = void;
export type ListProvidersResponse = ProviderInfo[];
export type ResolveReportResponse = void;
export type GetAPIKeysByOwnerResponse = APIKeyInfoV2[];
export type TransferCreditResponse = void;
export type GetProviderCountResponse = string;
export type ReportModelResponse = void;
export type UpdateRouteResponse = void;
export type GetRouteResponse = ModelRoute;
export type InitResponse = void;
export type GetModelReportsResponse = ModelReport[];
export type CreateRouteResponse = void;
export type GetProviderByOwnerResponse = string | null;
export type GetUserSettlementHistoryResponse = UserSettlementRecord[];
export type GetModelsByProviderResponse = ModelInfoV2[];
export type DeactivateModelResponse = void;
export type QueryModelsResponse = ModelInfoWithID[];
export type ListRoutesResponse = ModelRouteWithID[];
export type LeaveRouteResponse = void;
export type SettleUsageWithRouteResponse = void;
export type CloseProviderResponse = void;
export type GetSettlementHistoryResponse = ProviderSettlementRecord[];
export type DeleteModelResponse = void;
export type RevokeAPIKeyResponse = void;
export type CreateProviderResponse = void;
export type WithdrawalsByUserResponse = WithdrawApply[];
export type DeleteRouteResponse = void;
export type WithdrawResponse = WithdrawApply;
export type GetRouteByNameResponse = ModelRouteWithID;
export type RechargeResponse = void;
export type ListActiveRoutesByTypeResponse = ModelRouteWithID[];
export type GetModelsResponse = ModelInfoWithProvider[];
export type GetModelAuditsResponse = ModelAudit[];
export type GetTransferHistoryResponse = TransferRecord[];
export type GetProviderCreditResponse = string;
export type GetProviderResponse = ProviderInfo;
export type GetModelByNameResponse = ModelInfoWithID;
export type ListActiveProvidersResponse = ProviderInfoWithID[];
export type WithdrawalCancelledResponse = void;
export type ListModelsResponse = ModelInfoV2[];
export type ListActiveModelsWithinProviderResponse = ModelInfoWithProvider[];
export type LookupAPIKeyResponse = APIKeyInfoV2;
export type GetRechargeHistoryResponse = RechargeRecord[];
export type WithdrawalDoneResponse = void;
export type ListProviderModelsResponse = ModelInfoWithID[];
export type UpdateModelLimitsResponse = void;
export type GetModelResponse = ModelInfoV2;
export type ActiveModelResponse = void;
export type AddDepositResponse = void;
export type ListActiveModelsResponse = ModelInfoWithID[];
export type AddModelAPIKeyRefResponse = void;
export type GetProviderByNameResponse = ProviderInfoWithID;
export type RegisterAPIKeyResponse = void;
export type JoinRouteResponse = void;
