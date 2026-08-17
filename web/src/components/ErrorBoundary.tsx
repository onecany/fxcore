// 全局渲染错误边界：任何页面/组件渲染异常时显示错误面板而非白屏。
// 根因（2026-08）：KlineChart 数据源降序导致 lightweight-charts setData 抛错、
// 以及历史 client.ts 契约错误（undefined.toLocaleString）都曾让 React 整树卸载白屏。
import { Component, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: { componentStack?: string }) {
    console.error('[FXcore] render error:', error, info.componentStack ?? '');
  }

  render() {
    if (this.state.error) {
      return (
        <div className="app-shell">
          <div className="panel" style={{ maxWidth: 620, margin: '10vh auto 0', padding: 24 }}>
            <div className="panel-title">页面出错了</div>
            <p className="dim" style={{ margin: '0 0 12px' }}>
              页面加载遇到问题，已自动拦截。请尝试刷新；若持续出现，可把技术详情反馈给维护者。
            </p>
            <details style={{ marginBottom: 16 }}>
              <summary className="dim" style={{ cursor: 'pointer', fontSize: 13 }}>
                技术详情（反馈时提供）
              </summary>
              <pre
                className="terminal"
                style={{ fontSize: 12, overflow: 'auto', whiteSpace: 'pre-wrap', padding: 12 }}
              >
                {this.state.error.message}
                {'\n'}
                {this.state.error.stack ?? ''}
              </pre>
            </details>
            <button className="btn primary" onClick={() => window.location.reload()}>
              ⟳ 刷新重试
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
