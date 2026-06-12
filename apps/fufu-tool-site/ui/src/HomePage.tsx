import { useEffect, useState } from 'react';
import { Card } from '@heroui/react';
import { TopActions, ThemeToggle, NavPill, BlueprintHeader, BlueprintStamp } from './blueprint';

type ToolLink = {
  label: string;
  href: string;
  ping?: string;
};

type ToolCard = {
  stamp: string;
  title: string;
  description?: string;
  accent: 'clay' | 'moss' | 'stone';
  href?: string;
  links?: ToolLink[];
};

// fallback line urls if /api/nav/lines is empty or unreachable
const STATIC_API_LINES: ToolLink[] = [
  { label: '线路 一', href: 'https://api.fufuapi.top', ping: 'https://api.fufuapi.top' },
  { label: '线路 二', href: 'https://api.fufuapi.online', ping: 'https://api.fufuapi.online' },
  { label: '线路 三', href: 'https://api.fufuflower.top', ping: 'https://api.fufuflower.top' }
];

const STATIC_TOKEN_LINES: ToolLink[] = [
  { label: '线路 一', href: 'https://token.fufuapi.top', ping: 'https://token.fufuapi.top' },
  { label: '线路 二', href: 'https://token.fufuapi.online', ping: 'https://token.fufuapi.online' },
  { label: '线路 三', href: 'https://token.fufuflower.top', ping: 'https://token.fufuflower.top' }
];

const staticCards: ToolCard[] = [
  {
    stamp: '终端',
    title: 'Web Terminal',
    description: '服务器网页管理终端',
    accent: 'moss',
    links: [
      { label: '线路 一', href: 'https://terminal.fufuapi.top', ping: 'https://terminal.fufuapi.top' },
      { label: '线路 二', href: 'https://terminal.if.tc', ping: 'https://terminal.if.tc' }
    ]
  },
  { stamp: '造物', title: 'Build', description: 'AI 画图生成', accent: 'stone', href: 'https://build.fufuapi.online' },
  { stamp: '状态', title: 'API / 模型状态', description: '连通性检测、模型可用性与手动测试', accent: 'moss', href: '/status' },
  { stamp: '合卡', title: '合卡工具', description: '自助合并额度卡，复用统一 NewAPI 配置', accent: 'clay', href: '/combine' },
  { stamp: '活动', title: '活动前台', description: '抽奖、刮刮卡与福利入口', accent: 'stone', href: '/activity' }
];

type NavLine = { name: string; url: string };
type NavCategory = { kind: string; name: string; lines: NavLine[] };

function useNavLines() {
  const [cats, setCats] = useState<NavCategory[] | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/nav/lines', { signal: controller.signal })
      .then((response) => response.json())
      .then((data) => setCats(Array.isArray(data.categories) ? data.categories : []))
      .catch(() => setCats(null));
    return () => controller.abort();
  }, []);
  return cats;
}

function linesFor(cats: NavCategory[] | null, kind: string, fallback: ToolLink[]): ToolLink[] {
  const lines = cats?.find((cat) => cat.kind === kind)?.lines ?? [];
  if (!lines.length) return fallback;
  return lines.map((line, index) => ({ label: line.name || `线路 ${index + 1}`, href: line.url, ping: line.url }));
}

function latencyClass(ms: number) {
  if (ms < 200) return 'good';
  if (ms < 500) return 'slow';
  return 'bad';
}

function useLatency(url?: string) {
  const [result, setResult] = useState<{ text: string; className: string }>({ text: '--', className: '' });

  useEffect(() => {
    if (!url) return;
    const controller = new AbortController();
    const start = performance.now();
    const timer = window.setTimeout(() => controller.abort(), 5000);
    fetch(url, { method: 'HEAD', mode: 'no-cors', cache: 'no-store', signal: controller.signal })
      .then(() => {
        const ms = Math.round(performance.now() - start);
        setResult({ text: `${ms}ms`, className: latencyClass(ms) });
      })
      .catch(() => setResult({ text: controller.signal.aborted ? '超时' : '失败', className: 'bad' }))
      .finally(() => window.clearTimeout(timer));
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [url]);

  return result;
}

function LatencyAnchor({ link }: { link: ToolLink }) {
  const latency = useLatency(link.ping);
  return (
    <a className="tool-link" href={link.href}>
      <span>{link.label}</span>
      <span className={`latency ${latency.className}`}>{latency.text}</span>
      <span className="arrow">›</span>
    </a>
  );
}

function NavigationCard({ card, index }: { card: ToolCard; index: number }) {
  const cardBody = (
    <>
      <BlueprintStamp>{card.stamp}</BlueprintStamp>
      <Card.Title className="tool-card-title">{card.title}</Card.Title>
      {card.description ? <Card.Description className="tool-card-description">{card.description}</Card.Description> : null}
      {card.links ? (
        <div className="tool-links">
          {card.links.map((link) => <LatencyAnchor key={link.href} link={link} />)}
        </div>
      ) : null}
      {card.href ? <span className="tool-enter">进入 ›</span> : null}
    </>
  );
  const cardElement = (
    <Card
      className={`tool-card fade-in accent-${card.accent}`}
      style={{ animationDelay: `${0.1 + index * 0.05}s` }}
    >
      <Card.Content>{cardBody}</Card.Content>
    </Card>
  );
  return card.href ? (
    <a className="tool-card-anchor" href={card.href}>{cardElement}</a>
  ) : cardElement;
}

export function HomePage() {
  const navCats = useNavLines();
  const toolCards: ToolCard[] = [
    { stamp: '次数', title: 'API 次数站', accent: 'clay', links: linesFor(navCats, 'api', STATIC_API_LINES) },
    { stamp: '额度', title: 'Token 站', accent: 'moss', links: linesFor(navCats, 'token', STATIC_TOKEN_LINES) },
    ...staticCards
  ];
  return (
    <>
      <TopActions>
        <ThemeToggle />
        <NavPill href="/admin">管理后台</NavPill>
      </TopActions>
      <main className="blueprint-page home-page">
        <BlueprintHeader title="fufu 工 具 站" subtitle="导 航 · 状 态 · 合 卡 · 活 动" />
        <section className="tool-grid" aria-label="fufu 工具导航">
          {toolCards.map((card, index) => <NavigationCard key={card.title} card={card} index={index} />)}
        </section>
      </main>
    </>
  );
}
