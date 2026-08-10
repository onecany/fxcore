// Exchanges 页面：账户编队 + 动态表单（按类型字段）+ 启用切换 + 删除。
// 凭据字段统一由 CRED_FIELD_META 元数据驱动渲染与提交（snake_case 契约 ↔ camelCase state）。
import { useCallback, useEffect, useMemo, useState } from 'react';
import * as exchangeApi from '../api/v1/modules/exchanges';
import type { ExchangeAccount } from '../api/v1/types/contract';
import { PageHead, Panel, Alert, StatCard } from '../components/ui';
import { ExchangeIcon } from '../components/exchange-icon';

interface FormState {
  exchangeType: string;
  accountName: string;
  enabled: boolean;
  /** 凭据字段（camelCase key → 输入值），由类型字段元数据动态驱动 */
  creds: Record<string, string>;
}

const EMPTY: FormState = { exchangeType: '', accountName: '', enabled: true, creds: {} };

/** 凭据字段元数据：snake_case（后端契约）↔ camelCase（前端 state）+ 输入类型 + 文案 */
const CRED_FIELD_META: Record<string, { snake: string; label: string; type: 'text' | 'password' | 'number'; placeholder?: string }> = {
  apiKey: { snake: 'api_key', label: 'API Key', type: 'text' },
  secretKey: { snake: 'secret_key', label: 'Secret Key', type: 'password' },
  passphrase: { snake: 'passphrase', label: 'Passphrase', type: 'password' },
  hyperliquidWalletAddr: { snake: 'hyperliquid_wallet_addr', label: '钱包地址', type: 'text', placeholder: '0x…' },
  hyperliquidPrivateKey: { snake: 'hyperliquid_private_key', label: '私钥（ed25519 hex）', type: 'password' },
  asterUser: { snake: 'aster_user', label: 'Aster User', type: 'text' },
  asterSigner: { snake: 'aster_signer', label: 'Aster Signer', type: 'text' },
  asterPrivateKey: { snake: 'aster_private_key', label: 'Aster 私钥', type: 'password' },
  lighterWalletAddr: { snake: 'lighter_wallet_addr', label: 'Lighter 钱包地址', type: 'text' },
  lighterPrivateKey: { snake: 'lighter_private_key', label: 'Lighter 私钥', type: 'password' },
  lighterApiKeyPrivateKey: { snake: 'lighter_api_key_private_key', label: 'Lighter API Key 私钥', type: 'password' },
  lighterApiKeyIndex: { snake: 'lighter_api_key_index', label: 'Lighter API Key Index', type: 'number' },
};

/** snake_case → camelCase（与 client.ts 归一化一致） */
function camelKey(k: string): string {
  return k.replace(/_([a-z])/g, (_, ch: string) => ch.toUpperCase());
}

export default function ExchangesPage() {
  const [items, setItems] = useState<ExchangeAccount[]>([]);
  const [types, setTypes] = useState<exchangeApi.ExchangeTypeOption[]>([]);
  const [form, setForm] = useState<FormState>(EMPTY);
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [accounts, typeOpts] = await Promise.all([
        exchangeApi.listExchanges(),
        exchangeApi.listExchangeTypes().catch(() => []),
      ]);
      setItems(accounts);
      setTypes(typeOpts);
      setError(null);
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const typeMeta = useMemo(() => types.find((t) => t.type === form.exchangeType), [types, form.exchangeType]);
  const set = (k: keyof FormState, v: string | boolean) => setForm((f) => ({ ...f, [k]: v }));
  const setCred = (k: string, v: string) => setForm((f) => ({ ...f, creds: { ...f.creds, [k]: v } }));
  const enabledCount = items.filter((i) => i.enabled).length;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setMsg(null); setError(null);
    try {
      const body: Record<string, unknown> = {
        exchange_type: form.exchangeType,
        account_name: form.accountName,
        enabled: form.enabled,
      };
      for (const f of typeMeta?.fields ?? []) {
        const camel = camelKey(f);
        const v = form.creds[camel];
        if (typeof v === 'string' && v.trim()) body[f] = v.trim();
      }
      await exchangeApi.createExchange(body as never);
      setMsg('账户已接入，凭据 RSA 加密存储');
      setForm(EMPTY);
      void load();
    } catch (err) {
      setError(String(err));
    }
  };

  const toggle = async (id: string, enabled: boolean) => {
    try { await exchangeApi.setExchangeEnabled(id, !enabled); void load(); } catch (err) { setError(String(err)); }
  };
  const remove = async (id: string) => {
    if (!window.confirm('断开该交易所账户（删除）？')) return;
    try { await exchangeApi.deleteExchange(id); void load(); } catch (err) { setError(String(err)); }
  };

  return (
    <section>
      <PageHead
        title="交易所接入"
        lead={<>10 家统一适配层 · 凭据 RSA-OAEP 加密落库 · 响应零泄漏</>}
      />
      {error && <Alert kind="error">{error}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="stat-grid">
        <StatCard label="已接入" value={String(items.length)} hint={`${enabledCount} 个启用`} />
        <StatCard label="适配器" value={String(types.length)} hint="CEX 7 + DEX 3" />
      </div>

      <Panel title="新建接入点" className="mb">
        <form onSubmit={(e) => void submit(e)}>
          <div className="form-grid">
            <div className="field">
              <label>交易所类型</label>
              <div className="row" style={{ gap: 8 }}>
                <ExchangeIcon type={form.exchangeType} size={20} />
                <select value={form.exchangeType} onChange={(e) => set('exchangeType', e.target.value)} required>
                  <option value="">SELECT TYPE</option>
                  {types.map((t) => (
                    <option key={t.type} value={t.type}>{t.name} · {t.type}{t.requiresPassphrase ? ' *' : ''}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="field">
              <label>账户名称</label>
              <input value={form.accountName} onChange={(e) => set('accountName', e.target.value)} placeholder="e.g. main-futures" required />
            </div>
            <div className="field">
              <label>状态</label>
              <div className="checkbox-row" style={{ paddingTop: 8 }}>
                <input type="checkbox" checked={form.enabled} onChange={(e) => set('enabled', e.target.checked)} />
                <span className="dim">接入后立即启用</span>
              </div>
            </div>
            {(typeMeta?.fields ?? []).map((f) => {
              const camel = camelKey(f);
              const meta = CRED_FIELD_META[camel];
              if (!meta) return null;
              return (
                <div className="field" key={f}>
                  <label>{meta.label}</label>
                  <input
                    type={meta.type}
                    value={form.creds[camel] ?? ''}
                    onChange={(e) => setCred(camel, e.target.value)}
                    placeholder={meta.placeholder}
                  />
                </div>
              );
            })}
          </div>
          <div className="mt">
            <button className="btn primary" type="submit">⟳ 接入</button>
          </div>
        </form>
      </Panel>

      <Panel title="接入编队">
        <div className="table-wrap"><table className="data-table">
          <thead>
            <tr><th>名称</th><th>类型</th><th>状态</th><th>操作</th></tr>
          </thead>
          <tbody>
            {items.map((acc) => (
              <tr key={acc.id}>
                <td className="mono">{acc.accountName}</td>
                <td>
                  <span className="row" style={{ gap: 8 }}>
                    <ExchangeIcon type={acc.exchangeType} />
                    {acc.exchangeType}
                  </span>
                </td>
                <td>{acc.enabled ? <span className="mono" style={{ color: 'var(--fxcore-up)' }}>● 启用</span> : <span className="mono dim">○ 停用</span>}</td>
                <td>
                  <span className="row">
                    <button className="btn" onClick={() => void toggle(acc.id, acc.enabled)}>
                      {acc.enabled ? '停用' : '启用'}
                    </button>
                    <button className="btn danger" onClick={() => void remove(acc.id)}>删除</button>
                  </span>
                </td>
              </tr>
            ))}
            {items.length === 0 && <tr><td colSpan={5} className="empty">// NO EXCHANGE ACCOUNTS</td></tr>}
          </tbody>
        </table></div>
      </Panel>
    </section>
  );
}
