// 登录页安全展示：信任徽章行 + 安全架构四卡。
// 背书内容均为平台真实安全属性（RSA 凭据加密 / HMAC 签名 / 防重放 / 限流审计），不编造外部认证。
import type { TranslationKey } from '../../i18n/translations';

const SEC_ICONS = ['▣', '✒', '⟲', '◫'] as const;

/** 信任背书徽章（hero 区下方横排，mono 终端风格） */
export function TrustBadges({ t }: { t: TranslationKey }) {
  const items = [
    t.landing.trust.storage,
    t.landing.trust.signed,
    t.landing.trust.replay,
    t.landing.trust.audit,
  ];
  return (
    <div className="trust-bar">
      {items.map((label) => (
        <span key={label} className="trust-badge">
          <span className="trust-ok">✓</span>
          {label}
        </span>
      ))}
    </div>
  );
}

/** 安全架构四卡（凭据加密 / 签名校验 / 防重放 / 限流审计） */
export function SecurityFeatures({ t }: { t: TranslationKey }) {
  const items = [
    { icon: SEC_ICONS[0], title: t.landing.security.storageTitle, desc: t.landing.security.storageDesc },
    { icon: SEC_ICONS[1], title: t.landing.security.signedTitle, desc: t.landing.security.signedDesc },
    { icon: SEC_ICONS[2], title: t.landing.security.replayTitle, desc: t.landing.security.replayDesc },
    { icon: SEC_ICONS[3], title: t.landing.security.auditTitle, desc: t.landing.security.auditDesc },
  ];
  return (
    <section className="landing-block">
      <h3 className="block-title">{t.landing.security.title}</h3>
      <p className="block-lead">{t.landing.security.lead}</p>
      <div className="sec-grid">
        {items.map((it) => (
          <div key={it.title} className="sec-card glass-card">
            <span className="sec-icon">{it.icon}</span>
            <div>
              <div className="sec-title">{it.title}</div>
              <div className="sec-desc">{it.desc}</div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
