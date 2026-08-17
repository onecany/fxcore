// 隐私政策页（公共路由 /privacy）：内容来自三语 i18n legalPages.privacy。
import LegalLayout from '../sections/legal/LegalLayout';
import { useT } from '../stores/i18nStore';

export default function PrivacyPage() {
  const t = useT();
  const p = t.legalPages.privacy;
  return <LegalLayout title={p.title} updated={t.legalPages.lastUpdated} sections={p.sections} />;
}
