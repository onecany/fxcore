// FXcore 交易终端：认证门 + Neo-Gold 导航 + 路由化业务页面 + 三语 i18n。
import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, NavLink, Navigate, useNavigate, useLocation } from 'react-router';
import { useAuth, onUnauthorized } from './hooks/useAuth';
import ExchangesPage from './pages/ExchangesPage';
import TraderDashboardSection from './sections/TraderDashboardSection';
import ModelsPage from './pages/ModelsPage';
import StrategiesPage from './pages/StrategiesPage';
import BacktestPage from './pages/BacktestPage';
import DebatePage from './pages/DebatePage';
import TradersSection from './sections/TradersSection';
import { Alert } from './components/ui';
import { useI18n, useT } from './stores/i18nStore';
import { LANGS } from './i18n/translations';
import type { Lang } from './i18n/translations';

function LangSwitch() {
  const lang = useI18n((s) => s.lang);
  const setLang = useI18n((s) => s.setLang);
  return (
    <div className="lang-switch">
      {LANGS.map((l) => (
        <button
          key={l.key}
          className={`lang-btn ${lang === l.key ? 'active' : ''}`}
          onClick={() => setLang(l.key as Lang)}
        >
          {l.label}
        </button>
      ))}
    </div>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const t = useT();
  const location = useLocation();
  const [clock, setClock] = useState('');
  useEffect(() => {
    const tick = () => {
      const d = new Date();
      const p = (n: number) => String(n).padStart(2, '0');
      setClock(`${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`);
    };
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, []);

  const navItems = [
    { path: '/dashboard', label: t.nav.dashboard, glyph: '◉' },
    { path: '/traders', label: t.nav.traders, glyph: '◈' },
    { path: '/exchanges', label: t.nav.exchanges, glyph: '◐' },
    { path: '/models', label: t.nav.models, glyph: '⬡' },
    { path: '/strategies', label: t.nav.strategies, glyph: '✦' },
    { path: '/backtest', label: t.nav.backtest, glyph: '⌁' },
    { path: '/debate', label: t.nav.debate, glyph: '⚡' },
  ];
  const cur = navItems.find((i) => location.pathname.startsWith(i.path));

  return (
    <div className="app-frame">
      <aside className="app-sidebar">
        <div className="brand-wrap">
          <span className="logo">{t.brand.name}</span>
          <span className="sub">{t.brand.sub}</span>
        </div>
        <nav className="side-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) => `side-tab ${isActive ? 'active' : ''}`}
            >
              <span className="glyph">{item.glyph}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="side-foot">
          <div className="sys-row">
            <span>SYSTEM</span>
            <span className="led" title="system-ready" />
          </div>
          <div className="sys-row">
            <span>TIME</span>
            <span>{clock}</span>
          </div>
        </div>
      </aside>
      <main className="app-main">
        <div className="app-statusbar">
          <div className="st-crumb">
            <span className="crumb">{t.brand.name}</span>
            <span className="slash">/</span>
            <span className="cur">{cur?.label ?? ''}</span>
          </div>
          <div className="st-side">
            <LangSwitch />
            <span className="st-mono">{clock}</span>
            <span className="st-mono" style={{ color: 'var(--fxcore-up)' }}>●</span>
            <span className="st-mono">{user?.email}</span>
            <button className="btn ghost" onClick={() => void logout()}>{t.common.logout}</button>
          </div>
        </div>
        <div className="scanline">{children}</div>
      </main>
    </div>
  );
}

/** 交易员页内部跨页跳转（创建/查看后跳仪表盘等） */
function TradersRoute() {
  const navigate = useNavigate();
  return <TradersSection onNavigate={(tab, opts) => navigate({ pathname: `/${tab}`, search: opts?.traderId ? `?trader=${opts.traderId}` : undefined })} />;
}

function AuthGate() {
  const { user } = useAuth();
  const location = useLocation();
  if (!user) return <AuthPanel />;
  // 登录后根路径重定向到仪表盘
  if (location.pathname === '/') return <Navigate to="/dashboard" replace />;
  return <Shell><Routes>
    <Route path="/dashboard" element={<TraderDashboardSection />} />
    <Route path="/traders" element={<TradersRoute />} />
    <Route path="/exchanges" element={<ExchangesPage />} />
    <Route path="/models" element={<ModelsPage />} />
    <Route path="/strategies" element={<StrategiesPage />} />
    <Route path="/backtest" element={<BacktestPage />} />
    <Route path="/debate" element={<DebatePage />} />
    <Route path="*" element={<Navigate to="/dashboard" replace />} />
  </Routes></Shell>;
}

function AuthPanel() {
  const { login, register } = useAuth();
  const t = useT();
  const [authMsg, setAuthMsg] = useState<string | null>(null);
  const [authMode, setAuthMode] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [nickname, setNickname] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => onUnauthorized(() => setAuthMsg(t.auth.errExpired)), [t]);

  const submitAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthMsg(null);
    if (!email.trim() || !password) { setAuthMsg(t.auth.errEmpty); return; }
    if (authMode === 'register' && password.length < 8) { setAuthMsg(t.auth.errShort); return; }
    setBusy(true);
    try {
      if (authMode === 'login') {
        await login({ email: email.trim(), password });
      } else {
        await register({ email: email.trim(), password, nickname: nickname.trim() || undefined });
      }
    } catch (err) {
      setAuthMsg(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="app-shell">
      <div className="app-topbar">
        <div className="app-brand">
          <span className="logo" style={{ fontSize: 20 }}>{t.brand.name}</span>
          <span className="sub">{t.brand.sub}</span>
        </div>
        <div className="app-user"><LangSwitch /></div>
      </div>
      <div style={{ maxWidth: 420, margin: '10vh auto 0' }} className="panel scanline">
        <div className="panel-title">{t.auth.title}</div>
        <p className="dim" style={{ margin: '0 0 14px' }}>
          {t.auth.desc}
        </p>
        <div className="sub-tabs" style={{ marginBottom: 16 }}>
          <button type="button" className={`sub-tab ${authMode === 'login' ? 'active' : ''}`} onClick={() => { setAuthMode('login'); setAuthMsg(null); }}>
            {t.auth.login}
          </button>
          <button type="button" className={`sub-tab ${authMode === 'register' ? 'active' : ''}`} onClick={() => { setAuthMode('register'); setAuthMsg(null); }}>
            {t.auth.register}
          </button>
        </div>
        {authMsg && <Alert kind="warn">{authMsg}</Alert>}
        <form onSubmit={(e) => void submitAuth(e)} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <input
            className="prompt-area"
            type="email"
            placeholder={t.auth.email}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
          {authMode === 'register' && (
            <input
              className="prompt-area"
              type="text"
              placeholder={t.auth.nickname}
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              maxLength={64}
            />
          )}
          <input
            className="prompt-area"
            type="password"
            placeholder={authMode === 'register' ? t.auth.passwordHint : t.auth.password}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={authMode === 'login' ? 'current-password' : 'new-password'}
          />
          <button className="btn primary" type="submit" disabled={busy} style={{ width: '100%', padding: '10px' }}>
            {busy ? t.auth.busy : authMode === 'login' ? t.auth.login : t.auth.create}
          </button>
        </form>
        <div style={{ marginTop: 14, textAlign: 'center' }}>
          <button className="btn ghost" style={{ width: '100%' }} disabled={busy} onClick={() => void login({ email: 'admin@fxcore.local', password: 'admin123' })}>
            {t.auth.devLogin}
          </button>
        </div>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthGate />
    </BrowserRouter>
  );
}
