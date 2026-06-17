import type { FormEvent, ReactNode } from 'react';
import { useState } from 'react';
import { Button, Card, Table, Input } from '@heroui/react';
import { formatNumber } from '../blueprint';

export type MessageState = {
  text: string;
  tone?: 'ok' | 'error' | '';
};

export function Metric({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="bp-kpi">
      <span className="bp-kpi-label">{label}</span>
      <span className="bp-kpi-num">{formatNumber(value)}</span>
    </div>
  );
}

export function DataTable({
  columns,
  rows,
  empty = '暂无数据'
}: {
  columns: string[];
  rows: Array<Array<unknown>>;
  empty?: string;
}) {
  return (
    <Table className="blueprint-table">
      <div className="table-wrap">
        <table>
          <thead>
            <tr>{columns.map((column) => <th key={column}>{column}</th>)}</tr>
          </thead>
          <tbody>
            {rows.length ? rows.map((row, rowIndex) => (
              <tr key={rowIndex}>
                {row.map((cell, cellIndex) => <td key={cellIndex}>{String(cell ?? '-')}</td>)}
              </tr>
            )) : (
              <tr><td colSpan={columns.length}><span className="inline-help">{empty}</span></td></tr>
            )}
          </tbody>
        </table>
      </div>
    </Table>
  );
}

export function ConfigCard({
  title,
  description,
  action,
  children
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="bp-card admin-panel">
      <header className="bp-card-titlebar">
        <span>{title}</span>
        {action}
      </header>
      <div className="bp-card-body">
        {description ? <p className="bp-card-desc">{description}</p> : null}
        {children}
      </div>
    </section>
  );
}

export function LoginPanel({ onLogin, busy }: { onLogin: (token: string) => Promise<void>; busy: boolean }) {
  const [token, setToken] = useState('');

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onLogin(token);
  }

  return (
    <Card className="auth-panel fade-in">
      <Card.Header>
        <div>
          <Card.Title>管理员登录</Card.Title>
          <Card.Description>先完成登录验证，再展示完整管理面板。口令只提交给本机后台验证。</Card.Description>
        </div>
      </Card.Header>
      <Card.Content>
        <form className="auth-form" onSubmit={submit}>
          <Input
            type="password"
            autoComplete="current-password"
            placeholder="输入管理员口令"
            value={token}
            onChange={(event) => setToken(event.target.value)}
            className="blueprint-input"
          />
          <Button type="submit" className="blueprint-primary-button" isDisabled={busy}>
            {busy ? '验证中' : '登录后台'}
          </Button>
        </form>
      </Card.Content>
    </Card>
  );
}
