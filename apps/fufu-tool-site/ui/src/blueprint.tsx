import type { ReactNode } from 'react';
import { Button, Chip, useTheme } from '@heroui/react';

export function BlueprintGuideLine({ variant = 0 }: { variant?: 0 | 1 }) {
  const path = variant === 0
    ? 'M0 6 H280 M40 2 V10 M140 2 V10 M240 2 V10'
    : 'M0 6 H280 M70 3 V9 M210 3 V9';
  return (
    <svg className="blueprint-guide-line" viewBox="0 0 280 12" fill="none" aria-hidden="true">
      <path d={path} strokeWidth={variant === 0 ? 1.2 : 1} strokeLinecap="square" fill="none" />
    </svg>
  );
}

export function BlueprintHeader({
  title,
  subtitle,
  label = 'FUFU TOOLKIT',
  compact = false
}: {
  title: string;
  subtitle: string;
  label?: string;
  compact?: boolean;
}) {
  return (
    <header
      className={`blueprint-header fade-in${compact ? ' blueprint-header--compact' : ''}`}
      data-label={label}
    >
      <h1 className="blueprint-title">{title}</h1>
      <p className="blueprint-subtitle">{subtitle}</p>
    </header>
  );
}

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme('system');
  const isDark = resolvedTheme === 'dark';
  return (
    <Button
      className="blueprint-icon-button"
      aria-label="切换深色模式"
      isIconOnly
      onPress={() => setTheme(isDark ? 'light' : 'dark')}
    >
      {isDark ? '☀' : '☽'}
    </Button>
  );
}

export function TopActions({ children }: { children: ReactNode }) {
  return <div className="blueprint-top-actions" aria-label="快捷入口">{children}</div>;
}

export function NavPill({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} className="blueprint-nav-button">
      {children}
    </a>
  );
}

export function BlueprintStamp({ children }: { children: ReactNode }) {
  return <Chip className="blueprint-stamp" variant="soft">{children}</Chip>;
}

export function MessageLine({ tone, children }: { tone?: 'ok' | 'error' | ''; children?: ReactNode }) {
  if (!children) return <p className="blueprint-message" aria-live="polite" />;
  return (
    <p className={`blueprint-message${tone ? ` blueprint-message--${tone}` : ''}`} aria-live="polite">
      {children}
    </p>
  );
}

export function formatNumber(value: unknown, fallback = '-') {
  if (typeof value === 'number' && Number.isFinite(value)) return new Intl.NumberFormat('zh-CN').format(value);
  if (typeof value === 'string' && value.trim()) return value;
  return fallback;
}
