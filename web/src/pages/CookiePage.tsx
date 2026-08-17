// Cookie 声明页（公共路由 /cookies）：内容来自三语 i18n legalPages.cookies。
import LegalLayout from '../sections/legal/LegalLayout';
import { useT } from '../stores/i18nStore';

export default function CookiePage() {
  const t = useT();
  const p = t.legalPages.cookies;
  return <LegalLayout title={p.title} updated={t.legalPages.lastUpdated} sections={p.sections} />;
}
