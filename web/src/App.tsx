// FXcore-web 最小入口页：验证 API v1 层（SWR + hooks）全链路可用。
// 真实业务页面（Dashboard / Traders / Config）在后续阶段接入。
import { useEffect, useState } from 'react';
import { useDashboard } from './hooks/useDashboard';
import { useTraders } from './hooks/useTrader';
import { useAuth, onUnauthorized } from './hooks/useAuth';

export default function App() {
  const { user, login, logout } = useAuth();
  const { data: dash, error: dashErr, isLoading: dashLoading } = useDashboard();
  const { data: traders, error: tradersErr, isLoading: tradersLoading } = useTraders({ size: 5 });
  const [authMsg, setAuthMsg] = useState<string | null>(null);

  // 401 刷新失败（1101 强制登出）事件订阅（L5）
  useEffect(() => onUnauthorized(() => setAuthMsg('登录已过期，请重新登录')), []);

  return (
    <main style={{ fontFamily: 'system-ui, sans-serif', maxWidth: 720, margin: '40px auto', padding: '0 16px' }}>
      <h1>FXcore-web · API v1</h1>
      {authMsg && <p style={{ color: 'orange' }}>{authMsg}</p>}
      <section>
        <h2>认证</h2>
        {user ? (
          <div>
            <p>已登录：{user.email}（{user.nickname}）</p>
            <button onClick={() => void logout()}>登出</button>
          </div>
        ) : (
          <button onClick={() => void login({ email: 'admin@fxcore.local', password: 'admin123' })}>
            登录（dev 默认账号）
          </button>
        )}
      </section>
      <section>
        <h2>仪表盘（SWR 5s 轮询，后台暂停）</h2>
        {dashLoading && <p>加载中...</p>}
        {dashErr && <p style={{ color: 'red' }}>错误：{String(dashErr)}</p>}
        {dash && (
          <pre>{JSON.stringify(dash, null, 2)}</pre>
        )}
      </section>
      <section>
        <h2>交易员（前 5 个）</h2>
        {tradersLoading && <p>加载中...</p>}
        {tradersErr && <p style={{ color: 'red' }}>错误：{String(tradersErr)}</p>}
        {traders && <pre>{JSON.stringify(traders, null, 2)}</pre>}
      </section>
    </main>
  );
}
