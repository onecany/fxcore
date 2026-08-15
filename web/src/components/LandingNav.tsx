// 落地页吸顶导航：品牌 + 锚点（功能/行情/安全）+ 语言切换 + 登录 CTA。
import { useNavigate } from 'react-router';
import { LangSwitch } from './LangSwitch';
import { ThemeToggle } from './ThemeToggle';
import { useT } from '../stores/i18nStore';

export function LandingNav() {
  const t = useT();
  const navigate = useNavigate();

  const anchors = [
    { href: '#features', label: t.landing.nav.features },
    { href: '#markets', label: t.landing.nav.markets },
    { href: '#security', label: t.landing.nav.security },
  ];

  return (
    <header className="landing-nav">
      <div className="landing-nav-inner">
        <a className="app-brand ln-brand" href="#top" aria-label={t.brand.name}>
          <img src="/fxcore-icon.svg" alt="FXcore" className="brand-mark" width={24} height={24} />
          <span className="logo" style={{ fontSize: 18 }}>{t.brand.name}</span>
          <span className="sub">{t.brand.sub}</span>
        </a>
        <nav className="ln-links">
          {anchors.map((a) => (
            <a key={a.href} href={a.href} className="ln-link">
              {a.label}
            </a>
          ))}
        </nav>
        <div className="ln-actions">
          <ThemeToggle />
          <LangSwitch />
          <button className="btn primary" onClick={() => navigate('/login')}>
            {t.auth.login}
          </button>
        </div>
      </div>
    </header>
  );
}
