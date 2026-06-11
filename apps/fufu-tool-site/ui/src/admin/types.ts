export type AdminSession = {
  authenticated: boolean;
};

export type ManagedSite = {
  name: string;
  url: string;
  token?: string;
  tokenMasked?: string;
  tokenSet?: boolean;
  userId: string;
  kind: string;
  skipUserHeader?: boolean;
  quotaUnit: number;
  currency: string;
  rechargeRatio: number;
  channelListEndpoint?: string;
  note?: string;
};

export type PrizeRow = {
  type?: string;
  dollars?: number;
  weight?: number;
  [key: string]: unknown;
};

export type ActivityConfig = {
  startText?: string;
  endText?: string;
  startTS?: number;
  endTS?: number;
  targetExpectedValue?: number;
  actualExpectedValue?: number;
  spinMap?: Record<string, number>;
  prizePool?: PrizeRow[];
  tierPools?: Record<string, PrizeRow[]>;
  postJackpotPrizes?: PrizeRow[];
  scratchRewards?: number[];
  [key: string]: unknown;
};

export type AdminConfig = {
  newapi: {
    sites: ManagedSite[];
  };
  activity: ActivityConfig;
};

export type PublicSite = {
  name?: string;
  url?: string;
  displayUrl?: string;
  userId?: string;
};

export type RuntimeSitesResponse = {
  configured: boolean;
  error?: string;
  sites: PublicSite[];
};

export type ActivityStats = Record<string, unknown>;

export type SaleCardPlan = {
  id: string;
  name?: string;
  quota?: number;
  intervalUnit?: string;
  itemId?: string;
  skuId?: string;
  group?: string;
};

export type SaleCardJob = {
  plan: string;
  count?: number;
  enabled?: boolean;
};

export type SaleCardSchedule = {
  enabled?: boolean;
  time?: string;
  timezone?: string;
  jobs?: SaleCardJob[];
};

export type SaleCardConfig = {
  plans?: SaleCardPlan[];
  schedule?: SaleCardSchedule;
};

export type PrizeConfigResponse = ActivityConfig;
