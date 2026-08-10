// i18n 契约测试：三语嵌套键结构必须完全一致（新增 key 必须补全三语）。
import { describe, it, expect } from 'vitest';
import { translations, LANGS } from './translations';

type Nested = Record<string, unknown>;

function keysOf(obj: Nested, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    v && typeof v === 'object' ? keysOf(v as Nested, `${prefix}${k}.`) : [`${prefix}${k}`],
  );
}

describe('i18n translations', () => {
  it('三语结构一致（同一 key 集合）', () => {
    const zh = keysOf(translations.zh as unknown as Nested).sort();
    for (const lang of ['en', 'id'] as const) {
      const other = keysOf(translations[lang] as unknown as Nested).sort();
      expect(other).toEqual(zh);
    }
  });

  it('全部语言无空值文案', () => {
    for (const lang of ['zh', 'en', 'id'] as const) {
      const t = translations[lang] as unknown as Nested;
      const walk = (obj: Nested): string[] =>
        Object.values(obj).flatMap((v) => (v && typeof v === 'object' ? walk(v as Nested) : [String(v)]));
      for (const s of walk(t)) {
        expect(s.trim().length).toBeGreaterThan(0);
      }
    }
  });

  it('LANGS 含三种语言且无重复', () => {
    expect(LANGS.map((l) => l.key)).toEqual(['zh', 'en', 'id']);
  });
});
