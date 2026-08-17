// FXcore 交易终端：认证门 + Neo-Gold 导航 + 路由化业务页面 + 三语 i18n。
// 未登录：/ 落地页（LandingPage）、/login 登录页（LoginPage）；登录后进入 Shell 业务路由。
import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, NavLink, Navigate, useNavigate, useLocation } from 'react-router';
import { useAuth } from './hooks/useAuth';
import LandingPage from './pages/LandingPage';
import LoginPage from './pages/LoginPage';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import ResetPasswordPage from './pages/ResetPasswordPage';
import ExchangesPage from './pages/ExchangesPage';
import TraderDashboardSection from './sections/TraderDashboardSection';
import ModelsPage from './pages/ModelsPage';
import StrategiesPage from './pages/StrategiesPage';
import BacktestPage from './pages/BacktestPage';
import DebatePage from './pages/DebatePage';
import TradersSection from './sections/TradersSection';
import { LangSwitch } from './components/LangSwitch';
import { ThemeToggle } from './components/ThemeToggle';
import { Footer } from './components/Footer';
import { useT } from './stores/i18nStore';
import { useTheme } from './stores/themeStore';

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
      <header className="app-topnav">
        <div className="brand-wrap">
          <img src="/fxcore-icon.svg" alt="FXcore" className="brand-mark" width={22} height={22} />
          <span className="logo">{t.brand.name}</span>
          <span className="sub">{t.brand.sub}</span>
        </div>
        <nav className="top-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) => `top-tab ${isActive ? 'active' : ''}`}
            >
              <span className="glyph">{item.glyph}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="top-right">
          <div className="sys-row">
            <span>SYSTEM</span>
            <span className="led" title="system-ready" />
          </div>
          <div className="sys-row">
            <span>TIME</span>
            <span>{clock}</span>
          </div>
          <ThemeToggle />
          <LangSwitch />
          <button className="btn ghost" onClick={() => void logout()}>{t.common.logout}</button>
        </div>
      </header>
      <main className="app-main">
        <div className="app-statusbar">
          <div className="st-crumb">
            <span className="crumb">{t.brand.name}</span>
            <span className="slash">/</span>
            <span className="cur">{cur?.label ?? ''}</span>
          </div>
          <div className="st-side">
            <span className="st-mono" style={{ color: 'var(--fxcore-up)' }}>●</span>
            <span className="st-mono">{user?.email}</span>
          </div>
        </div>
        <div className="scanline">{children}</div>
        <Footer />
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
  // 未登录：/login 登录页，其余路径统一落地页（未登录访问业务路由也回到首页）
  if (!user) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="*" element={<LandingPage />} />
      </Routes>
    );
  }
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

export default function App() {
  const initTheme = useTheme((s) => s.init);
  // 应用持久化主题（根组件挂载即生效，覆盖所有页面含未登录态）
  useEffect(() => { initTheme(); }, [initTheme]);
  return (
    <BrowserRouter>
      <AuthGate />
    </BrowserRouter>
  );
}
