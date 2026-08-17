// 服务条款页（公共路由 /terms）：内容来自三语 i18n legalPages.terms。
import LegalLayout from '../sections/legal/LegalLayout';
import { useT } from '../stores/i18nStore';

export default function TermsPage() {
  const t = useT();
  const p = t.legalPages.terms;
  return <LegalLayout title={p.title} updated={t.legalPages.lastUpdated} sections={p.sections} />;
}
