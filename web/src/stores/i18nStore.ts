// i18n 状态：语言选择（zustand 全局 + localStorage 持久化）。
import { create } from 'zustand';
import { translations, type Lang, type TranslationKey } from '../i18n/translations';

interface I18nState {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: TranslationKey;
}

const STORAGE_KEY = 'fxcore-lang';

function loadLang(): Lang {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === 'zh' || v === 'en' || v === 'id') return v;
  } catch { /* ignore */ }
  return 'zh';
}

export const useI18n = create<I18nState>((set) => ({
  lang: loadLang(),
  setLang: (l) => {
    try { localStorage.setItem(STORAGE_KEY, l); } catch { /* ignore */ }
    set({ lang: l });
  },
  t: translations[loadLang()],
}));

// 订阅语言变化时刷新 t 的便捷选择器（在组件里用：const t = useI18nSelector()）
export function useT(): TranslationKey {
  return useI18n((s) => translations[s.lang]);
}
