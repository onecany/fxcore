// 登录页（独立路由 /login）：两栏——左品牌/安全展示 + 右毛玻璃表单卡。
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import { LangSwitch } from '../components/LangSwitch';
import { Footer } from '../components/Footer';
import { Alert } from '../components/ui';
import { useAuth, onUnauthorized } from '../hooks/useAuth';
import { useT } from '../stores/i18nStore';

const TRUST_KEYS = ['storage', 'signed', 'replay', 'audit'] as const;

export default function LoginPage() {
  const { login, register } = useAuth();
  const t = useT();
  const navigate = useNavigate();
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
    <div className="login-page">
      <header className="landing-topbar">
        <div className="app-brand">
          <img src="/fxcore-icon.svg" alt="FXcore" className="brand-mark" width={24} height={24} />
          <span className="logo">{t.brand.name}</span>
          <span className="sub">{t.brand.sub}</span>
        </div>
        <div className="app-user">
          <LangSwitch />
          <button className="btn ghost" onClick={() => navigate('/')}>
            {t.landing.backHome}
          </button>
        </div>
      </header>

      <div className="auth-hero">
        {/* 左：品牌与安全展示 */}
        <div className="auth-showcase">
          <div className="sys-status ok">
            <span className="led" />
            {t.landing.hero.badge}
          </div>
          <h1 className="auth-title">{t.landing.hero.title}</h1>
          <p className="auth-lead">{t.landing.hero.lead}</p>
          <div className="auth-trust">
            {TRUST_KEYS.map((k) => (
              <span key={k} className="auth-trust-item">{t.landing.trust[k]}</span>
            ))}
          </div>
        </div>

        {/* 右：表单卡 */}
        <div className="glass-card auth-card">
          <div className="panel-title">{t.auth.title}</div>
          <p className="dim auth-desc">
            {t.auth.desc}
          </p>
          <div className="sub-tabs">
            <button type="button" className={`sub-tab ${authMode === 'login' ? 'active' : ''}`} onClick={() => { setAuthMode('login'); setAuthMsg(null); }}>
              {t.auth.login}
            </button>
            <button type="button" className={`sub-tab ${authMode === 'register' ? 'active' : ''}`} onClick={() => { setAuthMode('register'); setAuthMsg(null); }}>
              {t.auth.register}
            </button>
          </div>
          {authMsg && <Alert kind="warn">{authMsg}</Alert>}
          <form onSubmit={(e) => void submitAuth(e)} className="auth-form">
            <div className="field">
              <label className="auth-label">{t.auth.email}</label>
              <input
                type="email"
                placeholder={t.auth.email}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
              />
            </div>
            {authMode === 'register' && (
              <div className="field">
                <label className="auth-label">{t.auth.nickname}</label>
                <input
                  type="text"
                  placeholder={t.auth.nickname}
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                  maxLength={64}
                />
              </div>
            )}
            <div className="field">
              <label className="auth-label">{authMode === 'register' ? t.auth.passwordHint : t.auth.password}</label>
              <input
                type="password"
                placeholder={authMode === 'register' ? t.auth.passwordHint : t.auth.password}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete={authMode === 'login' ? 'current-password' : 'new-password'}
              />
            </div>
            <button className="btn primary auth-submit" type="submit" disabled={busy}>
              {busy ? t.auth.busy : authMode === 'login' ? t.auth.login : t.auth.create}
            </button>
            {authMode === 'login' && (
              <div className="auth-link-row">
                <button type="button" className="link-btn" onClick={() => navigate('/forgot-password')}>
                  {t.auth.forgot}
                </button>
              </div>
            )}
          </form>
        </div>
      </div>
      <Footer />
    </div>
  );
}
