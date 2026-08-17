// 法律条款页共享布局：topbar（品牌 + 语言切换 + 返回首页）+ 单列内容 + Footer。
// 供隐私政策 / 服务条款 / Cookie 声明三页复用；公共路由，登录与否均可访问。
import { useNavigate } from 'react-router';
import { LangSwitch } from '../../components/LangSwitch';
import { Footer } from '../../components/Footer';
import { useT } from '../../stores/i18nStore';

interface LegalSection {
  t: string;
  b: string[];
}

export default function LegalLayout({
  title,
  updated,
  sections,
}: {
  title: string;
  updated: string;
  sections: LegalSection[];
}) {
  const t = useT();
  const navigate = useNavigate();

  return (
    <div className="legal-page">
      <header className="landing-topbar">
        <div className="app-brand">
          <img src="/fxcore-icon.svg" alt="FXcore" className="brand-mark" width={24} height={24} />
          <span className="logo">{t.brand.name}</span>
          <span className="sub">{t.brand.sub}</span>
        </div>
        <div className="app-user">
          <LangSwitch />
          <button className="btn ghost" onClick={() => navigate('/')}>
            {t.legalPages.backHome}
          </button>
        </div>
      </header>

      <main className="legal-container">
        <h1 className="legal-title">{title}</h1>
        <p className="legal-updated">{updated}</p>
        {sections.map((s, i) => (
          <section key={i} className="legal-section">
            <h2 className="legal-h2">{s.t}</h2>
            {s.b.map((para, j) => (
              <p key={j} className="legal-p">{para}</p>
            ))}
          </section>
        ))}
      </main>

      <Footer />
    </div>
  );
}
