// 未登录落地页（标准金融科技落地页结构，保持 FXcore 金色深色体系）：
// 吸顶导航 → hero 两栏（文案 + 行情卡）→ 多币种迷你卡 → 功能 6 卡 → 统计条 → 安全架构 → 钱包集成 → CTA 区 → 公共页脚。
import { useNavigate } from 'react-router';
import { LandingNav } from '../components/LandingNav';
import { Footer } from '../components/Footer';
import { MarketPreview } from '../sections/auth/MarketPreview';
import { MiniTickers } from '../sections/auth/MiniTickers';
import { TrustBadges, SecurityFeatures } from '../sections/auth/SecurityAndTrust';
import { WalletIntegrations } from '../sections/auth/WalletIntegrations';
import { useT } from '../stores/i18nStore';
import type { TranslationKey } from '../i18n/translations';

const FEATURES: { icon: string; title: keyof TranslationKey['landing']['features']; desc: keyof TranslationKey['landing']['features'] }[] = [
  { icon: '◈', title: 'aiTitle', desc: 'aiDesc' },
  { icon: '◉', title: 'marketTitle', desc: 'marketDesc' },
  { icon: '✦', title: 'studioTitle', desc: 'studioDesc' },
  { icon: '⌁', title: 'backtestTitle', desc: 'backtestDesc' },
  { icon: '⚡', title: 'debateTitle', desc: 'debateDesc' },
  { icon: '◐', title: 'exchangeTitle', desc: 'exchangeDesc' },
];

const STATS: { value: keyof TranslationKey['landing']['stats']; label: keyof TranslationKey['landing']['stats'] }[] = [
  { value: 'exchValue', label: 'exchLabel' },
  { value: 'modelValue', label: 'modelLabel' },
  { value: 'uptimeValue', label: 'uptimeLabel' },
  { value: 'refreshValue', label: 'refreshLabel' },
];

export default function LandingPage() {
  const t = useT();
  const navigate = useNavigate();

  return (
    <div className="landing" id="top">
      <LandingNav />
      <main className="landing-main">
        {/* hero：左文案 + 右行情卡 */}
        <section className="hero-grid">
          <div className="hero-text">
            <span className="hero-badge">
              <span className="led" />
              {t.landing.hero.badge}
            </span>
            <h1 className="hero-title">{t.landing.hero.title}</h1>
            <p className="hero-lead">{t.landing.hero.lead}</p>
            <div className="hero-ctas">
              <button className="btn primary" onClick={() => navigate('/login')}>
                {t.landing.hero.ctaPrimary}
              </button>
              <a className="btn ghost" href="#markets">
                {t.landing.hero.ctaSecondary}
              </a>
            </div>
            <TrustBadges t={t} />
          </div>
          <div className="hero-market">
            <MarketPreview t={t} />
          </div>
        </section>

        {/* 多币种迷你行情卡 */}
        <section id="markets" className="landing-block">
          <MiniTickers />
        </section>

        {/* 功能 6 卡 */}
        <section id="features" className="landing-block">
          <div className="block-head">
            <h3 className="block-title">{t.landing.features.title}</h3>
            <p className="block-lead">{t.landing.features.lead}</p>
          </div>
          <div className="feat-grid">
            {FEATURES.map((f) => (
              <div key={f.title} className="feat-card glass-card">
                <span className="feat-icon">{f.icon}</span>
                <div className="feat-title">{t.landing.features[f.title]}</div>
                <div className="feat-desc">{t.landing.features[f.desc]}</div>
              </div>
            ))}
          </div>
        </section>

        {/* 统计条 */}
        <section className="stats-bar glass-card">
          {STATS.map((s) => (
            <div key={s.value} className="stat-item">
              <div className="stat-value">{t.landing.stats[s.value]}</div>
              <div className="stat-label">{t.landing.stats[s.label]}</div>
            </div>
          ))}
        </section>

        {/* 安全架构 */}
        <section id="security" className="landing-block">
          <SecurityFeatures t={t} />
        </section>

        {/* 钱包集成 */}
        <WalletIntegrations t={t} />

        {/* CTA 区 */}
        <section id="cta" className="glass-card cta-card">
          <h3 className="cta-title">{t.landing.cta.title}</h3>
          <p className="cta-lead">{t.landing.cta.lead}</p>
          <div className="cta-actions">
            <button className="btn primary" onClick={() => navigate('/login')}>
              {t.landing.cta.primary}
            </button>
            <a className="btn ghost" href="#features">
              {t.landing.cta.secondary}
            </a>
          </div>
          <div className="cta-notes">
            <span>✓ {t.landing.cta.note1}</span>
            <span>✓ {t.landing.cta.note2}</span>
          </div>
        </section>
      </main>
      <Footer />
    </div>
  );
}
