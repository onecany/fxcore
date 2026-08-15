// Exchanges 页面：账户编队卡片网格 + 分卡表单（01 连接 / 02 凭据）。
// 数据层 SWR（创建/切换/删除后 mutate）；凭据字段由 CRED_FIELD_META 元数据驱动（snake_case ↔ camelCase）。
import { useMemo, useState } from 'react';
import useSWR from 'swr';
import * as exchangeApi from '../api/v1/modules/exchanges';
import type { ExchangeAccount } from '../api/v1/types/contract';
import { PageHead, Panel, Alert } from '../components/ui';
import { TmCard } from '../sections/dashboard/primitives';
import { ExchangeIcon } from '../components/exchange-icon';
import { ExchangeSelect } from '../components/exchange-select';

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
  const { data: items = [], mutate, error: loadError } = useSWR<ExchangeAccount[]>(
    ['/exchanges', 'page'],
    () => exchangeApi.listExchanges(),
  );
  const { data: types = [] } = useSWR<exchangeApi.ExchangeTypeOption[]>(
    ['/exchanges/types', 'page'],
    () => exchangeApi.listExchangeTypes().catch(() => []),
  );
  const [form, setForm] = useState<FormState>(EMPTY);
  const [okMsg, setOkMsg] = useState<string | null>(null);
  const [errMsg, setErrMsg] = useState<string | null>(null);

  const typeMeta = useMemo(() => types.find((t) => t.type === form.exchangeType), [types, form.exchangeType]);
  const set = (k: keyof FormState, v: string | boolean) => setForm((f) => ({ ...f, [k]: v }));
  const setCred = (k: string, v: string) => setForm((f) => ({ ...f, creds: { ...f.creds, [k]: v } }));
  const enabledCount = items.filter((i) => i.enabled).length;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setOkMsg(null); setErrMsg(null);
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
      setOkMsg('账户已接入，凭据 RSA 加密存储');
      setForm(EMPTY);
      void mutate();
    } catch (err) {
      setErrMsg(String(err));
    }
  };

  const toggle = async (id: string, enabled: boolean) => {
    try { await exchangeApi.setExchangeEnabled(id, !enabled); void mutate(); } catch (err) { setErrMsg(String(err)); }
  };
  const remove = async (id: string) => {
    if (!window.confirm('断开该交易所账户（删除）？')) return;
    try { await exchangeApi.deleteExchange(id); void mutate(); } catch (err) { setErrMsg(String(err)); }
  };

  const credFields = typeMeta?.fields ?? [];

  return (
    <section>
      <PageHead
        title="交易所接入"
        lead={<>10 家统一适配层 · 凭据 RSA-OAEP 加密落库 · 响应零泄漏</>}
      />
      {loadError && <Alert kind="error">{String(loadError)}</Alert>}
      {errMsg && <Alert kind="error">{errMsg}</Alert>}
      {okMsg && <Alert kind="ok">{okMsg}</Alert>}

      {/* KPI 概览（毛玻璃，与仪表盘同语言） */}
      <div className="dash-kpi acc-kpi">
        <TmCard label="已接入" value={String(items.length)} sub={`${enabledCount} 个启用`} />
        <TmCard label="启用中" value={String(enabledCount)} sub={`${items.length - enabledCount} 个停用`} color="var(--fxcore-up)" />
        <TmCard label="适配器" value={String(types.length)} sub="CEX 7 + DEX 3" />
      </div>

      {/* 新建接入点：分卡表单（01 连接 / 02 凭据） */}
      <Panel title="新建接入点" className="mb">
        <form onSubmit={(e) => void submit(e)} className="acc-form">
          <div className="form-section">
            <div className="form-section-title">
              <span className="form-section-num">01</span>
              连接
            </div>
            <div className="form-grid">
              <div className="field">
                <label>交易所类型</label>
                <ExchangeSelect value={form.exchangeType} options={types} onChange={(v) => set('exchangeType', v)} />
              </div>
              <div className="field">
                <label>账户名称</label>
                <input value={form.accountName} onChange={(e) => set('accountName', e.target.value)} placeholder="e.g. main-futures" required />
              </div>
              <div className="field">
                <label>状态</label>
                <div className="checkbox-row acc-checkbox">
                  <input type="checkbox" checked={form.enabled} onChange={(e) => set('enabled', e.target.checked)} />
                  <span className="dim">接入后立即启用</span>
                </div>
              </div>
            </div>
          </div>
          <div className="form-section">
            <div className="form-section-title">
              <span className="form-section-num">02</span>
              凭据
            </div>
            {credFields.length === 0 ? (
              <div className="muted mono empty-hint">// 选择交易所类型后显示凭据字段</div>
            ) : (
              <div className="form-grid">
                {credFields.map((f) => {
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
            )}
          </div>
          <div className="mt">
            <button className="btn primary" type="submit">⟳ 接入</button>
          </div>
        </form>
      </Panel>

      {/* 接入编队：账户卡片网格 */}
      <Panel title={`接入编队 (${items.length})`}>
        {items.length === 0 ? (
          <div className="muted mono empty-hint centered">// NO EXCHANGE ACCOUNTS</div>
        ) : (
          <div className="acc-grid">
            {items.map((acc) => (
              <div className="acc-card glass-card" key={acc.id}>
                <div className="acc-head">
                  <ExchangeIcon type={acc.exchangeType} size={22} />
                  <div className="acc-info">
                    <span className="acc-name">{acc.accountName}</span>
                    <span className="acc-meta">{acc.exchangeType.toUpperCase()} · {acc.testnet ? 'TESTNET' : 'CEX/DEX'}</span>
                  </div>
                  <span className={`acc-status ${acc.enabled ? 'on' : 'off'}`}>
                    {acc.enabled ? '● 启用' : '○ 停用'}
                  </span>
                </div>
                <div className="acc-actions">
                  <button className="btn btn-sm" onClick={() => void toggle(acc.id, acc.enabled)}>
                    {acc.enabled ? '停用' : '启用'}
                  </button>
                  <button className="btn danger btn-sm" onClick={() => void remove(acc.id)}>删除</button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Panel>
    </section>
  );
}
