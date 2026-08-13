// 交易所类型选择器（自定义下拉：图标 + 名称，替代原生 select——option 不支持图片）。
// 菜单用 React portal 渲染到 body（脱离 .panel/.scanline 的 backdrop-filter 层叠上下文，
// 否则菜单溢出面板时会被外层元素覆盖），fixed 定位 + 滚动/缩放跟随。
import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { ExchangeIcon } from './exchange-icon';
import type { ExchangeTypeOption } from '../api/v1/modules/exchanges';

interface MenuPos {
  top: number;
  left: number;
  width: number;
}

export function ExchangeSelect({ value, options, onChange }: {
  value: string;
  options: ExchangeTypeOption[];
  onChange: (v: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<MenuPos | null>(null);
  const [menuHeight, setMenuHeight] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const close = useCallback(() => setOpen(false), []);

  // 打开时按触发器几何定位菜单（fixed 坐标系 = 视口）
  const updatePos = useCallback(() => {
    const trig = rootRef.current?.querySelector('.exch-select-trigger') as HTMLElement | null;
    if (!trig) return;
    const r = trig.getBoundingClientRect();
    setPos({ top: r.bottom + 4, left: r.left, width: r.width });
  }, []);

  // 打开后测量菜单高度（render 期禁止读 ref，flip 判断依赖它）
  useEffect(() => {
    if (!open) return;
    const mh = menuRef.current?.offsetHeight ?? 0;
    setMenuHeight(mh);
  }, [open, pos]);

  // 打开：定位 + 滚动/缩放跟随（rAF 延迟到重排后读几何，避免读到滚动中间态）
  useEffect(() => {
    if (!open) return;
    updatePos();
    let raf = 0;
    const onScroll = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(updatePos);
    };
    const onResize = () => updatePos();
    // 捕获阶段监听滚动（fixed 菜单在滚动容器内滚动时自身不滚，需跟随）
    document.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onResize);
    return () => {
      cancelAnimationFrame(raf);
      document.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('resize', onResize);
    };
  }, [open, updatePos]);

  // 外部点击关闭（click 阶段：菜单项的 onClick 先冒泡执行，选中后再关；mousedown 会先关导致菜单项 click 丢失）
  useEffect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close();
    };
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('click', onDocClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [open, close]);

  // 菜单下边界溢出视口且触发器有足够上方空间时翻转向上
  const flipUp = !!(pos && menuHeight > 0 && pos.top + menuHeight > window.innerHeight - 8 && pos.top > 80);

  const selected = options.find((o) => o.type === value);

  return (
    <div className={`exch-select ${open ? 'open' : ''}`} ref={rootRef}>
      <button
        type="button"
        className={`exch-select-trigger ${open ? 'open' : ''}`}
        onClick={() => {
          if (open) { close(); return; }
          updatePos();
          setOpen(true);
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        {selected ? (
          <>
            <ExchangeIcon type={selected.type} size={18} />
            <span className="exch-select-name">{selected.name}</span>
            <span className="exch-select-type">· {selected.type}</span>
            {selected.requiresPassphrase && <span className="exch-select-pass">*</span>}
          </>
        ) : (
          <span className="exch-select-placeholder">SELECT TYPE</span>
        )}
        <span className={`exch-select-arrow ${open ? 'up' : ''}`}>▾</span>
      </button>
      {open && pos && createPortal(
        <div
          ref={menuRef}
          className={`exch-select-menu ${flipUp ? 'flip-up' : ''}`}
          style={{ top: flipUp ? undefined : pos.top, left: pos.left, width: pos.width, bottom: flipUp ? window.innerHeight - pos.top + 4 : undefined }}
          role="listbox"
        >
          {options.map((t) => (
            <button
              key={t.type}
              type="button"
              role="option"
              aria-selected={t.type === value}
              className={`exch-select-item ${t.type === value ? 'selected' : ''}`}
              onClick={() => {
                onChange(t.type);
                close();
              }}
            >
              <ExchangeIcon type={t.type} size={18} />
              <span className="exch-select-name">{t.name}</span>
              <span className="exch-select-type">· {t.type}</span>
              {t.requiresPassphrase && <span className="exch-select-pass" title="需要 Passphrase">*</span>}
            </button>
          ))}
        </div>,
        document.body,
      )}
    </div>
  );
}
