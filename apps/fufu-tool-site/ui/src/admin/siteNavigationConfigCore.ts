import type { AdminConfig, ManagedSite, ManagedSiteURL, NavigationCard, NavigationLink } from './types';

export const defaultManagedSite: ManagedSite = {
  name: '',
  category: 'api',
  urls: [],
  token: '',
  userId: '1',
  kind: 'api',
  skipUserHeader: false,
  quotaUnit: 500000,
  currency: '$',
  rechargeRatio: 1,
  channelListEndpoint: '',
  note: ''
};

export const defaultNavigationCard: NavigationCard = {
  id: '',
  stamp: '工具',
  title: '',
  description: '',
  accent: 'moss',
  lineKind: '',
  href: '',
  links: []
};

export const siteGroups: { category: string; title: string; desc: string }[] = [
  { category: 'api', title: 'API 次数站', desc: '一个站点：配置一次 access token，可加多条 base_url。合卡默认复用第一条线路。' },
  { category: 'token', title: 'Token 站', desc: '一个站点：配置一次 access token，可加多条 base_url。' }
];

export function siteCategory(site: ManagedSite): string {
  return site.category || 'api';
}

function defaultSiteName(category: string): string {
  return category === 'token' ? 'token-fufu' : '次数fufu';
}

function ensureManagedSiteWithIndex(sites: ManagedSite[], category: string): [ManagedSite[], number] {
  const idx = sites.findIndex((site) => siteCategory(site) === category);
  if (idx >= 0) return [sites, idx];
  const next = [...sites, { ...defaultManagedSite, category, name: defaultSiteName(category), urls: [] }];
  return [next, next.length - 1];
}

export function managedSiteForCategory(sites: ManagedSite[], category: string): ManagedSite | undefined {
  return sites.find((site) => siteCategory(site) === category);
}

export function updateManagedSite(sites: ManagedSite[], category: string, patch: Partial<ManagedSite>): ManagedSite[] {
  const [base, idx] = ensureManagedSiteWithIndex(sites, category);
  return base.map((site, i) => (i === idx ? { ...site, ...patch } : site));
}

export function addManagedSiteURL(sites: ManagedSite[], category: string): ManagedSite[] {
  const [base, idx] = ensureManagedSiteWithIndex(sites, category);
  const urls = base[idx].urls ?? [];
  return base.map((site, i) => (
    i === idx ? { ...site, urls: [...urls, { name: `线路 ${urls.length + 1}`, url: '' }] } : site
  ));
}

export function updateManagedSiteURL(
  sites: ManagedSite[],
  category: string,
  urlIndex: number,
  patch: Partial<ManagedSiteURL>
): ManagedSite[] {
  const idx = sites.findIndex((site) => siteCategory(site) === category);
  if (idx < 0) return sites;
  return sites.map((site, i) => (
    i === idx ? { ...site, urls: (site.urls ?? []).map((url, ui) => (ui === urlIndex ? { ...url, ...patch } : url)) } : site
  ));
}

export function removeManagedSiteURL(sites: ManagedSite[], category: string, urlIndex: number): ManagedSite[] {
  const idx = sites.findIndex((site) => siteCategory(site) === category);
  if (idx < 0) return sites;
  return sites.map((site, i) => (
    i === idx ? { ...site, urls: (site.urls ?? []).filter((_, ui) => ui !== urlIndex) } : site
  ));
}

export function patchNavigationCard(cards: NavigationCard[], index: number, patch: Partial<NavigationCard>): NavigationCard[] {
  return cards.map((card, i) => (i === index ? { ...card, ...patch } : card));
}

export function addNavigationCard(cards: NavigationCard[]): NavigationCard[] {
  return [...cards, { ...defaultNavigationCard }];
}

export function removeNavigationCard(cards: NavigationCard[], index: number): NavigationCard[] {
  return cards.filter((_, i) => i !== index);
}

export function addNavigationLink(cards: NavigationCard[], cardIndex: number): NavigationCard[] {
  const card = cards[cardIndex];
  if (!card) return cards;
  const links = card.links ?? [];
  return patchNavigationCard(cards, cardIndex, { links: [...links, { label: `线路 ${links.length + 1}`, href: '' }] });
}

export function patchNavigationLink(
  cards: NavigationCard[],
  cardIndex: number,
  linkIndex: number,
  patch: Partial<NavigationLink>
): NavigationCard[] {
  const card = cards[cardIndex];
  if (!card) return cards;
  return patchNavigationCard(cards, cardIndex, {
    links: (card.links ?? []).map((link, i) => (i === linkIndex ? { ...link, ...patch } : link))
  });
}

export function removeNavigationLink(cards: NavigationCard[], cardIndex: number, linkIndex: number): NavigationCard[] {
  const card = cards[cardIndex];
  if (!card) return cards;
  return patchNavigationCard(cards, cardIndex, { links: (card.links ?? []).filter((_, i) => i !== linkIndex) });
}

export function setNavigationCardLineKind(cards: NavigationCard[], cardIndex: number, lineKind: string): NavigationCard[] {
  const card = cards[cardIndex];
  if (!card) return cards;
  const links = card.links ?? [];
  return patchNavigationCard(cards, cardIndex, {
    lineKind,
    href: lineKind ? '' : card.href,
    links: lineKind ? [] : links
  });
}

export function buildAdminNavigationSavePayload(config: Pick<AdminConfig, 'newapi' | 'navigation'>): Pick<AdminConfig, 'newapi' | 'navigation'> {
  const sites = (config.newapi?.sites ?? [])
    .map((site) => ({ ...site, urls: (site.urls ?? []).filter((url) => (url.url ?? '').trim() !== '') }))
    .filter((site) => site.urls.length > 0);
  const cards = (config.navigation?.cards ?? [])
    .map((card) => {
      const lineKind = (card.lineKind ?? '').trim();
      return {
        ...card,
        lineKind,
        href: lineKind ? '' : card.href,
        links: lineKind
          ? []
          : (card.links ?? [])
            .filter((link) => (link.href ?? '').trim() !== '')
            .map((link) => ({ label: link.label, href: link.href }))
      };
    })
    .filter((card) => (
      (card.title ?? '').trim() !== ''
      && ((card.href ?? '').trim() !== '' || (card.links ?? []).length > 0 || (card.lineKind ?? '') !== '')
    ));
  return { newapi: { sites }, navigation: { cards } };
}
