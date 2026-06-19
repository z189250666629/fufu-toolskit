import { useState } from 'react';
import { Button, Input } from '@heroui/react';
import { messageFromError } from '../api';
import { MessageLine } from '../blueprint';
import type { MessageState } from './adminShared';
import type { MCYConfig } from './types';

export function MCYConfigEditor({
  mcy,
  onChange,
  onSave
}: {
  mcy: MCYConfig;
  onChange: (mcy: MCYConfig) => void;
  onSave: (mcy: MCYConfig) => Promise<void>;
}) {
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState<MessageState>({ text: '' });

  async function save() {
    setSaving(true);
    setSaveMsg({ text: '正在保存 MCY 配置…' });
    try {
      await onSave(mcy);
      setSaveMsg({ text: 'MCY 配置已保存', tone: 'ok' });
    } catch (error) {
      setSaveMsg({ text: messageFromError(error, '保存失败'), tone: 'error' });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="business-stack business-stack--compact">
      <div className="field-grid">
        <label className="field">商城地址 (base_url)
          <Input className="blueprint-input" value={mcy.baseUrl || ''} placeholder="https://shop.example.com" onChange={(event) => onChange({ ...mcy, baseUrl: event.target.value })} />
        </label>
        <label className="field">账号 (邮箱 / 用户名)
          <Input className="blueprint-input" value={mcy.username || ''} placeholder="you@example.com" onChange={(event) => onChange({ ...mcy, username: event.target.value })} />
        </label>
        <label className="field">密码
          <Input className="blueprint-input" type="password" value={mcy.password || ''} placeholder={mcy.passwordSet ? (mcy.passwordMasked || '已设置 · 留空不变') : '输入商城密码'} onChange={(event) => onChange({ ...mcy, password: event.target.value })} />
        </label>
      </div>
      <div className="action-row">
        <Button className="blueprint-primary-button" onPress={save} isDisabled={saving}>{saving ? '保存中…' : '保存 MCY 配置'}</Button>
      </div>
      {saveMsg.text ? <MessageLine tone={saveMsg.tone}>{saveMsg.text}</MessageLine> : null}
      <p className="inline-help">MCY 商城登录仍用于活动核销等现有流程；自动补卡与库存检测已暂时下线，不会从这里触发对接。存数据库，密码仅后台可见，留空表示沿用原值。</p>
    </div>
  );
}
