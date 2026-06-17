import { Button, Input, TextArea as Textarea } from '@heroui/react';
import { DataTable } from './adminShared';
import {
  addManagedSiteURL,
  addNavigationCard,
  addNavigationLink,
  managedSiteForCategory,
  patchNavigationCard,
  patchNavigationLink,
  removeManagedSiteURL,
  removeNavigationCard,
  removeNavigationLink,
  setNavigationCardLineKind,
  siteGroups,
  updateManagedSite,
  updateManagedSiteURL
} from './siteNavigationConfigCore';
import type {
  ManagedSite,
  NavigationCard,
  NavLinesResponse,
  RuntimeSitesResponse
} from './types';

export function SiteEditor({
  sites,
  onChange
}: {
  sites: ManagedSite[];
  onChange: (sites: ManagedSite[]) => void;
}) {
  return (
    <div className="site-groups">
      {siteGroups.map((group) => {
        const site = managedSiteForCategory(sites, group.category);
        const urls = site?.urls ?? [];
        return (
          <section key={group.category} className="bp-card">
            <header className="bp-card-titlebar">
              <span>{group.title}</span>
              <Button className="blueprint-button" onPress={() => onChange(addManagedSiteURL(sites, group.category))}>新增 URL</Button>
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
                  onChange={(event) => onChange(updateManagedSite(sites, group.category, { token: event.target.value }))}
                />
              </label>
              <label className="field">
                备注
                <Textarea
                  className="blueprint-input"
                  value={site?.note || ''}
                  placeholder="可选：用途、来源或维护说明"
                  onChange={(event) => onChange(updateManagedSite(sites, group.category, { note: event.target.value }))}
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
                      onChange={(event) => onChange(updateManagedSiteURL(sites, group.category, urlIndex, { name: event.target.value }))}
                    />
                    <Input
                      className="blueprint-input"
                      value={line.url || ''}
                      placeholder="https://api.example.com"
                      onChange={(event) => onChange(updateManagedSiteURL(sites, group.category, urlIndex, { url: event.target.value }))}
                    />
                    <Button className="blueprint-danger-button" onPress={() => onChange(removeManagedSiteURL(sites, group.category, urlIndex))}>删除</Button>
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
              <Button className="blueprint-danger-button" onPress={() => onChange(removeNavigationCard(cards, cardIndex))}>删除工具</Button>
            </header>
            <div className="bp-card-body">
              <div className="field-grid">
                <label className="field">ID<Input className="blueprint-input" value={card.id || ''} placeholder="terminal" onChange={(event) => onChange(patchNavigationCard(cards, cardIndex, { id: event.target.value }))} /></label>
                <label className="field">标记<Input className="blueprint-input" value={card.stamp || ''} placeholder="终端" onChange={(event) => onChange(patchNavigationCard(cards, cardIndex, { stamp: event.target.value }))} /></label>
                <label className="field">标题<Input className="blueprint-input" value={card.title || ''} placeholder="Web Terminal" onChange={(event) => onChange(patchNavigationCard(cards, cardIndex, { title: event.target.value }))} /></label>
                <label className="field">色彩
                  <select className="native-select" value={card.accent || 'moss'} onChange={(event) => onChange(patchNavigationCard(cards, cardIndex, { accent: event.target.value }))}>
                    <option value="moss">moss</option>
                    <option value="clay">clay</option>
                    <option value="stone">stone</option>
                  </select>
                </label>
                <label className="field">线路来源
                  <select className="native-select" value={lineKind} onChange={(event) => onChange(setNavigationCardLineKind(cards, cardIndex, event.target.value))}>
                    <option value="">静态链接</option>
                    <option value="api">API 次数站</option>
                    <option value="token">Token 站</option>
                  </select>
                </label>
              </div>
              <label className="field">说明<Textarea className="blueprint-input" value={card.description || ''} placeholder="展示在首页卡片上的说明" onChange={(event) => onChange(patchNavigationCard(cards, cardIndex, { description: event.target.value }))} /></label>
              {!lineKind ? (
                <>
                  <label className="field">单链接 href<Input className="blueprint-input" value={card.href || ''} placeholder="/status 或 https://example.com" onChange={(event) => onChange(patchNavigationCard(cards, cardIndex, { href: event.target.value }))} /></label>
                  <div className="config-subhead">多线路链接</div>
                  {links.length === 0 ? <p className="inline-help">需要展示多条线路时添加链接；单入口工具只填 href 即可。</p> : null}
                  <div className="site-url-list">
                    {links.map((link, linkIndex) => (
                      <div key={linkIndex} className="site-url-row">
                        <span className="site-url-tag">{linkIndex + 1}</span>
                        <Input className="blueprint-input site-url-name" value={link.label || ''} placeholder={`线路 ${linkIndex + 1}`} onChange={(event) => onChange(patchNavigationLink(cards, cardIndex, linkIndex, { label: event.target.value }))} />
                        <Input className="blueprint-input" value={link.href || ''} placeholder="https://example.com" onChange={(event) => onChange(patchNavigationLink(cards, cardIndex, linkIndex, { href: event.target.value }))} />
                        <Button className="blueprint-danger-button" onPress={() => onChange(removeNavigationLink(cards, cardIndex, linkIndex))}>删除</Button>
                      </div>
                    ))}
                  </div>
                  <div className="action-row">
                    <Button className="blueprint-button" onPress={() => onChange(addNavigationLink(cards, cardIndex))}>新增线路</Button>
                  </div>
                </>
              ) : null}
            </div>
          </section>
        );
      })}
      <Button className="blueprint-button" onPress={() => onChange(addNavigationCard(cards))}>新增首页卡片</Button>
    </div>
  );
}
