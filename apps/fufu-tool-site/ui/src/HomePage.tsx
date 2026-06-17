import { useEffect, useState } from 'react';
import { Card } from '@heroui/react';
import { TopActions, ThemeToggle, NavPill, BlueprintHeader, BlueprintStamp } from './blueprint';

type ToolLink = {
  label: string;
  href: string;
  ping?: string;
};

type ToolCard = {
  id?: string;
  stamp: string;
  title: string;
  description?: string;
  accent: 'clay' | 'moss' | 'stone';
  href?: string;
  links?: ToolLink[];
};

type NavToolsResponse = { cards?: ToolCard[] };

function useNavTools() {
  const [cards, setCards] = useState<ToolCard[] | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/nav/tools', { signal: controller.signal })
      .then((response) => response.json())
      .then((data: NavToolsResponse) => setCards(Array.isArray(data.cards) ? data.cards : []))
      .catch(() => setCards(null));
    return () => controller.abort();
  }, []);
  return cards;
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
  const latency = useLatency(link.ping || link.href);
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
      {card.links?.length ? (
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
  const toolCards = useNavTools() ?? [];
  return (
    <>
      <TopActions>
        <ThemeToggle />
        <NavPill href="/admin">管理后台</NavPill>
      </TopActions>
      <main className="blueprint-page home-page">
        <BlueprintHeader title="fufu 工 具 站" subtitle="导 航 · 状 态 · 合 卡 · 活 动" />
        <section className="tool-grid" aria-label="fufu 工具导航">
          {toolCards.map((card, index) => <NavigationCard key={card.id || card.title} card={card} index={index} />)}
        </section>
      </main>
    </>
  );
}
