import { Button, Input, TextArea as Textarea } from '@heroui/react';
import { DataTable } from './adminShared';
import type {
  ManagedSite,
  ManagedSiteURL,
  NavigationCard,
  NavigationLink,
  NavLinesResponse,
  RuntimeSitesResponse
} from './types';

const defaultSite: ManagedSite = {
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

const defaultNavigationCard: NavigationCard = {
  id: '',
  stamp: '工具',
  title: '',
  description: '',
  accent: 'moss',
  lineKind: '',
  href: '',
  links: []
};

const SITE_GROUPS: { category: string; title: string; desc: string }[] = [
  { category: 'api', title: 'API 次数站', desc: '一个站点：配置一次 access token，可加多条 base_url。合卡默认复用第一条线路。' },
  { category: 'token', title: 'Token 站', desc: '一个站点：配置一次 access token，可加多条 base_url。' }
];

export function SiteEditor({
  sites,
  onChange
}: {
  sites: ManagedSite[];
  onChange: (sites: ManagedSite[]) => void;
}) {
  const categoryOf = (site: ManagedSite) => site.category || 'api';
  const indexOf = (category: string) => sites.findIndex((site) => categoryOf(site) === category);
  const siteFor = (category: string) => sites.find((site) => categoryOf(site) === category);

  function ensureSite(category: string): [ManagedSite[], number] {
    const idx = indexOf(category);
    if (idx >= 0) return [sites, idx];
    const name = category === 'token' ? 'token-fufu' : '次数fufu';
    const next = [...sites, { ...defaultSite, category, name, urls: [] }];
    return [next, next.length - 1];
  }
  function updateSite(category: string, patch: Partial<ManagedSite>) {
    const [base, idx] = ensureSite(category);
    onChange(base.map((site, i) => (i === idx ? { ...site, ...patch } : site)));
  }
  function setCategoryToken(category: string, value: string) {
    updateSite(category, { token: value });
  }
  function addUrl(category: string) {
    const [base, idx] = ensureSite(category);
    const urls = base[idx].urls ?? [];
    onChange(base.map((site, i) => (i === idx ? { ...site, urls: [...urls, { name: `线路 ${urls.length + 1}`, url: '' }] } : site)));
  }
  function updateUrl(category: string, urlIndex: number, patch: Partial<ManagedSiteURL>) {
    const idx = indexOf(category);
    if (idx < 0) return;
    onChange(sites.map((site, i) => (i === idx ? { ...site, urls: (site.urls ?? []).map((u, ui) => (ui === urlIndex ? { ...u, ...patch } : u)) } : site)));
  }
  function removeUrl(category: string, urlIndex: number) {
    const idx = indexOf(category);
    if (idx < 0) return;
    onChange(sites.map((site, i) => (i === idx ? { ...site, urls: (site.urls ?? []).filter((_, ui) => ui !== urlIndex) } : site)));
  }

  return (
    <div className="site-groups">
      {SITE_GROUPS.map((group) => {
        const site = siteFor(group.category);
        const urls = site?.urls ?? [];
        return (
          <section key={group.category} className="bp-card">
            <header className="bp-card-titlebar">
              <span>{group.title}</span>
              <Button className="blueprint-button" onPress={() => addUrl(group.category)}>新增 URL</Button>
            </header>
            <div className="bp-card-body">
              <p className="bp-card-desc">{group.desc}</p>
              <label className="field site-token-field">
                access token（整站配置一次）
                <Input
                  className="blueprint-input"
                  type="password"
                  value={site?.token || ''}
                  placeholder={site?.tokenMasked || '留空表示沿用原 token'}
                  onChange={(event) => setCategoryToken(group.category, event.target.value)}
                />
              </label>
              <label className="field">
                备注
                <Textarea
                  className="blueprint-input"
                  value={site?.note || ''}
                  placeholder="可选：用途、来源或维护说明"
                  onChange={(event) => updateSite(group.category, { note: event.target.value })}
                />
              </label>
              <div className="config-subhead">base_url 线路</div>
              {urls.length === 0 ? <p className="inline-help">还没有 base_url，点“新增 URL”添加。</p> : null}
              <div className="site-url-list">
                {urls.map((line, urlIndex) => (
                  <div key={urlIndex} className="site-url-row">
                    <span className="site-url-tag">
                      {urlIndex + 1}
                      {group.category === 'api' && urlIndex === 0 ? ' · 合卡' : ''}
                    </span>
                    <Input
                      className="blueprint-input site-url-name"
                      value={line.name || ''}
                      placeholder={`线路 ${urlIndex + 1}`}
                      onChange={(event) => updateUrl(group.category, urlIndex, { name: event.target.value })}
                    />
                    <Input
                      className="blueprint-input"
                      value={line.url || ''}
                      placeholder="https://api.example.com"
                      onChange={(event) => updateUrl(group.category, urlIndex, { url: event.target.value })}
                    />
                    <Button className="blueprint-danger-button" onPress={() => removeUrl(group.category, urlIndex)}>删除</Button>
                  </div>
                ))}
              </div>
            </div>
          </section>
        );
      })}
      <p className="inline-help">每个站点仅配置一次 access token，可添加多条 base_url（线路名用于首页展示）；首页只明文展示 URL，token 仅后台可见。Token 留空表示沿用原值。</p>
    </div>
  );
}

export function RuntimeSites({ sites }: { sites?: RuntimeSitesResponse }) {
  const rows = (sites?.sites ?? []).map((site, index) => [
    site.name || '-',
    site.displayUrl || site.url || '地址已隐藏',
    site.userId || '-',
    index === 0 ? '是' : '否'
  ]);
  const empty = sites ? (sites.error || '尚未配置带 access token 的 NewAPI 运行站点。') : '正在加载状态页站点。';
  return <DataTable columns={['站点', '显示地址', 'User ID', '合卡复用']} rows={rows} empty={empty} />;
}

export function HomeNavLines({ lines }: { lines?: NavLinesResponse }) {
  const rows = (lines?.categories ?? []).flatMap((category) => (
    (category.lines ?? []).map((line, index) => [
      category.name || category.kind,
      line.name || `线路 ${index + 1}`,
      line.url || '-'
    ])
  ));
  return <DataTable columns={['首页分组', '线路', 'URL']} rows={rows} empty={lines ? '首页导航暂无线路。' : '正在加载首页导航线路。'} />;
}

export function NavigationToolsEditor({
  cards,
  onChange
}: {
  cards: NavigationCard[];
  onChange: (cards: NavigationCard[]) => void;
}) {
  function patchCard(index: number, patch: Partial<NavigationCard>) {
    onChange(cards.map((card, i) => (i === index ? { ...card, ...patch } : card)));
  }
  function addCard() {
    onChange([...cards, { ...defaultNavigationCard }]);
  }
  function removeCard(index: number) {
    onChange(cards.filter((_, i) => i !== index));
  }
  function addLink(cardIndex: number) {
    const card = cards[cardIndex];
    const links = card.links ?? [];
    patchCard(cardIndex, { links: [...links, { label: `线路 ${links.length + 1}`, href: '' }] });
  }
  function patchLink(cardIndex: number, linkIndex: number, patch: Partial<NavigationLink>) {
    const card = cards[cardIndex];
    patchCard(cardIndex, { links: (card.links ?? []).map((link, i) => (i === linkIndex ? { ...link, ...patch } : link)) });
  }
  function removeLink(cardIndex: number, linkIndex: number) {
    const card = cards[cardIndex];
    patchCard(cardIndex, { links: (card.links ?? []).filter((_, i) => i !== linkIndex) });
  }

  return (
    <div className="site-groups">
      {cards.length === 0 ? <p className="inline-help">未配置首页卡片；保存空配置时后端会使用默认首页卡片。</p> : null}
      {cards.map((card, cardIndex) => {
        const links = card.links ?? [];
        const lineKind = card.lineKind || '';
        return (
          <section key={cardIndex} className="bp-card">
            <header className="bp-card-titlebar">
              <span>{card.title || `工具 ${cardIndex + 1}`}</span>
              <Button className="blueprint-danger-button" onPress={() => removeCard(cardIndex)}>删除工具</Button>
            </header>
            <div className="bp-card-body">
              <div className="field-grid">
                <label className="field">ID<Input className="blueprint-input" value={card.id || ''} placeholder="terminal" onChange={(event) => patchCard(cardIndex, { id: event.target.value })} /></label>
                <label className="field">标记<Input className="blueprint-input" value={card.stamp || ''} placeholder="终端" onChange={(event) => patchCard(cardIndex, { stamp: event.target.value })} /></label>
                <label className="field">标题<Input className="blueprint-input" value={card.title || ''} placeholder="Web Terminal" onChange={(event) => patchCard(cardIndex, { title: event.target.value })} /></label>
                <label className="field">色彩
                  <select className="native-select" value={card.accent || 'moss'} onChange={(event) => patchCard(cardIndex, { accent: event.target.value })}>
                    <option value="moss">moss</option>
                    <option value="clay">clay</option>
                    <option value="stone">stone</option>
                  </select>
                </label>
                <label className="field">线路来源
                  <select className="native-select" value={lineKind} onChange={(event) => patchCard(cardIndex, { lineKind: event.target.value, href: event.target.value ? '' : card.href, links: event.target.value ? [] : links })}>
                    <option value="">静态链接</option>
                    <option value="api">API 次数站</option>
                    <option value="token">Token 站</option>
                  </select>
                </label>
              </div>
              <label className="field">说明<Textarea className="blueprint-input" value={card.description || ''} placeholder="展示在首页卡片上的说明" onChange={(event) => patchCard(cardIndex, { description: event.target.value })} /></label>
              {!lineKind ? (
                <>
                  <label className="field">单链接 href<Input className="blueprint-input" value={card.href || ''} placeholder="/status 或 https://example.com" onChange={(event) => patchCard(cardIndex, { href: event.target.value })} /></label>
                  <div className="config-subhead">多线路链接</div>
                  {links.length === 0 ? <p className="inline-help">需要展示多条线路时添加链接；单入口工具只填 href 即可。</p> : null}
                  <div className="site-url-list">
                    {links.map((link, linkIndex) => (
                      <div key={linkIndex} className="site-url-row">
                        <span className="site-url-tag">{linkIndex + 1}</span>
                        <Input className="blueprint-input site-url-name" value={link.label || ''} placeholder={`线路 ${linkIndex + 1}`} onChange={(event) => patchLink(cardIndex, linkIndex, { label: event.target.value })} />
                        <Input className="blueprint-input" value={link.href || ''} placeholder="https://example.com" onChange={(event) => patchLink(cardIndex, linkIndex, { href: event.target.value })} />
                        <Button className="blueprint-danger-button" onPress={() => removeLink(cardIndex, linkIndex)}>删除</Button>
                      </div>
                    ))}
                  </div>
                  <div className="action-row">
                    <Button className="blueprint-button" onPress={() => addLink(cardIndex)}>新增线路</Button>
                  </div>
                </>
              ) : null}
            </div>
          </section>
        );
      })}
      <Button className="blueprint-button" onPress={addCard}>新增首页卡片</Button>
    </div>
  );
}
