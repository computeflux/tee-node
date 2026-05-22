// Auto-generated from ink! ABI
// DO NOT EDIT MANUALLY

export interface Curve {
  Type: number[];
  LinearBegin: number;
  LinearEnd: number;
  LinearLength: number;
  StepBegin: number;
  StepEnd: number;
  StepSize: number;
  StepPeriod: number;
  ReciprocalFactor: number;
  ReciprocalXScale: number;
  ReciprocalXOffset: number;
  ReciprocalYOffset: number;
}

export interface Member {
  Account: UniAddr;
  Balance: U256;
}

export interface Option { None: null } | { Some: Vote }

export interface Proposal {
  Call: Option;
  TrackID: number;
  Caller: UniAddr;
  Status: ProposalStatus;
  SubmitBlock: number;
  Deposit: ProposalDeposit;
}

export interface ProposalDeposit {
  Depositor: UniAddr;
  Amount: U256;
  Block: number;
}

export interface ProposalStatus {
  State: number;
  Block: number;
}

export interface ProposalWithID {
  ID: number;
  Proposal: Proposal;
}

export interface TrackData {
  Name: number[];
  PreparePeriod: number;
  MaxDeciding: number;
  ConfirmPeriod: number;
  DecisionPeriod: number;
  MinEnactmentPeriod: number;
  DecisionDeposit: U256;
  MaxBalance: U256;
  MinApproval: Curve;
  MinSupport: Curve;
}

export interface TrackWithID {
  ID: number;
  Track: TrackData;
}

export interface U256 {
  _7: number[];
}

export interface UniAddr {
  T: number;
  V: number[];
}

export interface Vote {
  Index: number;
  ProposalID: number;
  Caller: UniAddr;
  Pledge: U256;
  OpinionYes: boolean;
  VoteWeight: U256;
  VoteBlock: number;
  Deleted: boolean;
}

export type CancelVoteResponse = void;
export type SetPublicJoinResponse = void;
export type SubmitProposalResponse = void;
export type LeaveResponse = void;
export type SetDefaultTrackResponse = void;
export type VotesResponse = Vote[];
export type DefaultTrackResponse = number;
export type TracksResponse = TrackWithID[];
export type ExecProposalResponse = void;
export type CancelProposalResponse = void;
export type VoteResponse = Option;
export type MembersResponse = Member[];
export type InitResponse = void;
export type PublicJoinResponse = void;
export type GetPublicJoinResponse = boolean;
export type TotalSupplyResponse = U256;
export type ProposalResponse = Option;
export type SpendResponse = void;
export type BalanceOfResponse = U256;
export type LeaveWithBurnResponse = void;
export type MintResponse = void;
export type PayoutResponse = void;
export type TransferResponse = void;
export type TrackResponse = Option;
export type SubmitVoteResponse = void;
export type JoinResponse = void;
export type AddTrackResponse = void;
export type DepositProposalResponse = void;
export type TransferFromResponse = void;
export type ProposalStatusResponse = Option;
export type ProposalsResponse = ProposalWithID[];
export type VotesOfUserResponse = Vote[];
