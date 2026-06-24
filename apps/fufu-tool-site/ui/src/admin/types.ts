export type AdminSession = {
  authenticated: boolean;
};

export type ManagedSiteURL = {
  name?: string;
  url: string;
};

export type ManagedSite = {
  name: string;
  category?: string;
  urls: ManagedSiteURL[];
  url?: string;
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
  rank?: string;
  label?: string;
  advertised?: boolean;
  [key: string]: unknown;
};

export type ActivityGameRoute = {
  dollars: number;
  game: 'slot' | 'scratch' | 'dragonboat' | string;
  drawCount?: number;
};

export type ActivityGameConfig = {
  game: 'slot' | 'scratch' | 'dragonboat' | string;
  targetExpectedValue?: number;
};

export type SubscriptionPlanMapping = {
  planId?: number;
  title?: string;
  match?: 'contains' | 'exact' | string;
  dollars: number;
};

export type DynamicPrizePoolTier = {
  dollars: number;
  revenue?: number;
  cost?: number;
};

export type DynamicPrizePoolConfig = {
  enabled?: boolean;
  contributionRate?: number;
  jackpotRate?: number;
  secondRate?: number;
  thirdRate?: number;
  tierEconomics?: DynamicPrizePoolTier[];
};

export type ActivityConfig = {
  enabled?: boolean;
  disabled?: boolean;
  startText?: string;
  endText?: string;
  startTS?: number;
  endTS?: number;
  targetExpectedValue?: number;
  spinMap?: Record<string, number>;
  gameConfigs?: ActivityGameConfig[];
  prizePool?: PrizeRow[];
  dynamicPrizePool?: DynamicPrizePoolConfig;
  scratchRewards?: number[];
  scratchMaxReveals?: number;
  gameRoutes?: ActivityGameRoute[];
  scratchTiers?: number[];
  subscriptionPlanMappings?: SubscriptionPlanMapping[];
  [key: string]: unknown;
};

export type MCYConfig = {
  baseUrl?: string;
  username?: string;
  password?: string;
  passwordSet?: boolean;
  passwordMasked?: string;
  loginEndpoint?: string;
  uploadEndpoint?: string;
};

export type AdminConfig = {
  newapi: {
    sites: ManagedSite[];
  };
  navigation: NavigationConfig;
  activity: ActivityConfig;
  mcy?: MCYConfig;
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

export type NavLine = {
  name?: string;
  url?: string;
};

export type NavLineCategory = {
  kind: string;
  name?: string;
  lines: NavLine[];
};

export type NavLinesResponse = {
  categories: NavLineCategory[];
};

export type NavigationLink = {
  label?: string;
  href: string;
  ping?: string;
};

export type NavigationCard = {
  id?: string;
  stamp: string;
  title: string;
  description?: string;
  accent: 'clay' | 'moss' | 'stone' | string;
  lineKind?: 'api' | 'token' | string;
  href?: string;
  links?: NavigationLink[];
};

export type NavigationConfig = {
  cards: NavigationCard[];
};

export type NavigationToolsResponse = NavigationConfig;

export type ActivityStats = Record<string, unknown>;

export type SaleCardPlan = {
  id: string;
  name?: string;
  quota?: number;
  intervalUnit?: number | string;
  itemId?: number | string;
  skuId?: number | string;
  group?: string;
  slot?: string;
};

export type SaleCardJob = {
  plan: string;
  targetStock?: number;
  enabled?: boolean;
};

export type SaleCardSlot = {
  group: string;
  label?: string;
  time?: string;
  enabled?: boolean;
  jobs?: SaleCardJob[];
};

export type SaleCardSchedule = {
  enabled?: boolean;
  timezone?: string;
  slots?: SaleCardSlot[];
};

export type SaleCardConfig = {
  plans?: SaleCardPlan[];
  schedule?: SaleCardSchedule;
  restockStatus?: SaleCardRestockStatus;
};

export type SaleCardRestockJobStatus = {
  id: number;
  bizDate?: string;
  slotGroup?: string;
  slotTime?: string;
  planId: string;
  targetStock?: number;
  status: 'pending' | 'running' | 'succeeded' | 'failed' | string;
  attempts?: number;
  consecutiveTimeouts?: number;
  currentStock?: number;
  uploaded?: number;
  lastError?: string;
  failureReason?: string;
  updatedAt?: string;
  finishedAt?: string;
};

export type SaleCardRestockStatus = {
  jobs?: SaleCardRestockJobStatus[];
};

export type SaleCardStockEntry = {
  planId: string;
  planName?: string;
  slot?: string;
  currentStock: number;
};

export type SaleCardStockResponse = {
  stock: SaleCardStockEntry[];
};

export type SaleCardRunResult = {
  planId?: string;
  planName?: string;
  currentStock?: number;
  targetStock?: number;
  toUpload?: number;
  uploaded?: number;
  generated?: number;
  keys?: string[];
};

export type SaleCardTestKeyResult = {
  planId?: string;
  planName?: string;
  quota?: number;
  game?: 'slot' | 'scratch' | 'dragonboat' | string;
  drawCount?: number;
  generated?: number;
  keys?: string[];
};

export type PrizeConfigResponse = ActivityConfig & {
  prizes?: PrizeRow[];
  poolBalance?: number;
  scratchPoolBalance?: number;
};
