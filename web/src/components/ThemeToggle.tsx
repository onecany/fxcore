// 主题切换按钮（导航栏图标）：☀ 切浅色 / ☾ 切回深色。
import { useTheme } from '../stores/themeStore';

export function ThemeToggle() {
  const theme = useTheme((s) => s.theme);
  const toggle = useTheme((s) => s.toggle);
  const dark = theme === 'dark';
  return (
    <button
      className="btn ghost theme-toggle"
      onClick={toggle}
      title={dark ? '切换到浅色模式' : '切换到深色模式'}
      aria-label={dark ? '切换到浅色模式' : '切换到深色模式'}
    >
      {dark ? '☀' : '☾'}
    </button>
  );
}
