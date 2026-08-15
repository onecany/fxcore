// 语言切换器（中文 / English / Indonesia）：国旗图标触发器 + 下拉选项。
// 下拉菜单 createPortal 到 body 渲染，脱离祖先层叠上下文防遮挡（固定定位 + 高 z-index）；
// 外点关闭用 click 而非 mousedown（portal 后选项在 body，mousedown 会先关导致点击丢失）。
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { LANGS } from '../i18n/translations';
import type { Lang } from '../i18n/translations';
import { useI18n } from '../stores/i18nStore';

const FLAGS: Record<Lang, string> = { zh: '🇨🇳', en: '🇺🇸', id: '🇮🇩' };

interface MenuPos {
  top: number;
  left: number;
  width: number;
}

export function LangSwitch() {
  const lang = useI18n((s) => s.lang);
  const setLang = useI18n((s) => s.setLang);
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<MenuPos | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  const currentLabel = LANGS.find((l) => l.key === lang)?.label ?? '';

  // 展开时测量触发器位置并跟随滚动（fixed 菜单不随容器滚，需捕获阶段监听重排）
  useEffect(() => {
    if (!open) return;
    const measure = () => {
      const el = rootRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      setPos({ top: r.bottom + 6, left: r.left, width: r.width });
    };
    measure();
    let raf = 0;
    const onScroll = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(measure);
    };
    document.addEventListener('scroll', onScroll, true);
    return () => {
      cancelAnimationFrame(raf);
      document.removeEventListener('scroll', onScroll, true);
    };
  }, [open]);

  // 外点关闭（click：菜单项 onClick 先于 document 冒泡）
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current?.contains(e.target as Node)) return;
      setOpen(false);
    };
    document.addEventListener('click', onDown);
    return () => document.removeEventListener('click', onDown);
  }, [open]);

  // Esc 关闭
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <div className="lang-drop" ref={rootRef}>
      <button
        type="button"
        className={`lang-trigger ${open ? 'open' : ''}`}
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="lang-flag">{FLAGS[lang]}</span>
        <span className="lang-label">{currentLabel}</span>
        <span className="lang-caret">▾</span>
      </button>
      {open &&
        pos &&
        createPortal(
          <div className="lang-menu" style={{ top: pos.top, left: pos.left, minWidth: pos.width }} role="listbox">
            {LANGS.map((l) => (
              <button
                key={l.key}
                type="button"
                role="option"
                aria-selected={lang === l.key}
                className={`lang-opt ${lang === l.key ? 'active' : ''}`}
                onClick={() => {
                  setLang(l.key as Lang);
                  setOpen(false);
                }}
              >
                <span className="lang-flag">{FLAGS[l.key as Lang]}</span>
                {l.label}
                {lang === l.key && <span className="lang-check">✓</span>}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </div>
  );
}
