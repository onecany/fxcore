// 重置密码页（独立路由 /reset-password?token=xxx）：
// 读取邮件链接中的一次性令牌 → 设置新密码 → 成功后引导登录。
import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { LangSwitch } from '../components/LangSwitch';
import { Footer } from '../components/Footer';
import { Alert } from '../components/ui';
import { resetPassword } from '../api/v1/modules/auth';
import { useT } from '../stores/i18nStore';

export default function ResetPasswordPage() {
  const t = useT();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const token = params.get('token') ?? '';

  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrMsg(null);
    if (password.length < 8) { setErrMsg(t.auth.errShort); return; }
    if (password !== confirm) { setErrMsg(t.auth.errMismatch); return; }
    setBusy(true);
    try {
      await resetPassword(token, password);
      setDone(true);
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
          <h1 className="auth-title">{t.auth.resetTitle}</h1>
          <p className="auth-lead">{t.auth.resetDesc}</p>
        </div>

        <div className="glass-card auth-card">
          <div className="panel-title">{t.auth.resetTitle}</div>
          <p className="dim auth-desc">{t.auth.resetDesc}</p>

          {!token ? (
            <>
              <Alert kind="warn">{t.auth.errTokenMissing}</Alert>
              <div className="auth-back">
                <button className="btn ghost" onClick={() => navigate('/login')}>
                  {t.auth.backLogin}
                </button>
              </div>
            </>
          ) : done ? (
            <div className="auth-success">
              <Alert kind="ok">{t.auth.resetSuccessTitle}</Alert>
              <p className="dim auth-desc" style={{ marginTop: 12 }}>{t.auth.resetSuccessDesc}</p>
              <div className="auth-back">
                <button className="btn primary auth-submit" onClick={() => navigate('/login')}>
                  {t.auth.goLogin}
                </button>
              </div>
            </div>
          ) : (
            <>
              {errMsg && <Alert kind="warn">{errMsg}</Alert>}
              <form onSubmit={(e) => void submit(e)} className="auth-form">
                <div className="field">
                  <label className="auth-label">{t.auth.newPassword}</label>
                  <input
                    type="password"
                    placeholder={t.auth.passwordHint}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="new-password"
                    autoFocus
                  />
                </div>
                <div className="field">
                  <label className="auth-label">{t.auth.confirmPassword}</label>
                  <input
                    type="password"
                    placeholder={t.auth.confirmPassword}
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    autoComplete="new-password"
                  />
                </div>
                <button className="btn primary auth-submit" type="submit" disabled={busy}>
                  {busy ? t.auth.busy : t.auth.resetTitle}
                </button>
              </form>
            </>
          )}
        </div>
      </div>
      <Footer />
    </div>
  );
}
