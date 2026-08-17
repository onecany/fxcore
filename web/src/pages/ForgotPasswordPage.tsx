// 忘记密码页（独立路由 /forgot-password）：输入注册邮箱 → 发送重置邮件。
// 复用登录页布局（landing-topbar + auth-hero + glass-card）。
import { useState } from 'react';
import { useNavigate } from 'react-router';
import { LangSwitch } from '../components/LangSwitch';
import { Footer } from '../components/Footer';
import { Alert } from '../components/ui';
import { forgotPassword } from '../api/v1/modules/auth';
import { useT } from '../stores/i18nStore';

export default function ForgotPasswordPage() {
  const t = useT();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const [sent, setSent] = useState<{ devLink?: string } | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrMsg(null);
    if (!email.trim()) { setErrMsg(t.auth.errEmpty); return; }
    setBusy(true);
    try {
      const resp = await forgotPassword(email.trim());
      setSent({ devLink: resp.devLink });
    } catch (err) {
      setErrMsg(err instanceof Error ? err.message : String(err));
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
          <button className="btn ghost" onClick={() => navigate('/login')}>
            {t.auth.backLogin}
          </button>
        </div>
      </header>

      <div className="auth-hero">
        <div className="auth-showcase">
          <div className="sys-status ok">
            <span className="led" />
            {t.landing.hero.badge}
          </div>
          <h1 className="auth-title">{t.auth.forgotTitle}</h1>
          <p className="auth-lead">{t.auth.forgotDesc}</p>
        </div>

        <div className="glass-card auth-card">
          <div className="panel-title">{t.auth.forgotTitle}</div>
          <p className="dim auth-desc">{t.auth.forgotDesc}</p>
          {errMsg && <Alert kind="warn">{errMsg}</Alert>}
          {sent ? (
            <div className="auth-success">
              <Alert kind="ok">{t.auth.forgotSentTitle}</Alert>
              <p className="dim auth-desc" style={{ marginTop: 12 }}>{t.auth.forgotSentDesc}</p>
              {sent.devLink && (
                <div className="dev-notice">
                  {t.auth.forgotDevNotice}
                  <br />
                  <code>{sent.devLink}</code>
                </div>
              )}
              <div className="auth-back">
                <button className="btn ghost" onClick={() => navigate('/login')}>
                  {t.auth.backLogin}
                </button>
              </div>
            </div>
          ) : (
            <form onSubmit={(e) => void submit(e)} className="auth-form">
              <div className="field">
                <label className="auth-label">{t.auth.email}</label>
                <input
                  type="email"
                  placeholder={t.auth.email}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  autoComplete="email"
                  autoFocus
                />
              </div>
              <button className="btn primary auth-submit" type="submit" disabled={busy}>
                {busy ? t.auth.busy : t.auth.forgotSend}
              </button>
            </form>
          )}
        </div>
      </div>
      <Footer />
    </div>
  );
}
