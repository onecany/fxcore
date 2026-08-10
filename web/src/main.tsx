import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import ErrorBoundary from './components/ErrorBoundary';
import './index.css';

// 开发期 Mock（MSW）：先执行 `npx msw init public` 生成 worker，
// 再取消注释即可用 Mock 数据开发（API设计.md 2）。
// async function enableMocking() {
//   if (!import.meta.env.DEV) return;
//   const { worker } = await import('./mocks/browser');
//   await worker.start({ onUnhandledRequest: 'bypass' });
// }
// enableMocking();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
);
