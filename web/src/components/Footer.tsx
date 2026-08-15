// 公共页脚：品牌列 + 四列链接 + 版权行。落地页 / 登录页 / 登录后 Shell 复用。
import { useT } from '../stores/i18nStore';

const PRODUCT_HREFS = ['/dashboard', '/traders', '/strategies', '/backtest', '/debate'];
const PLATFORM_HREFS = ['#markets', '#features', '#security', '#cta'];
const DOCS_HREFS = ['/swagger', '#', '#'];

export function Footer() {
  const t = useT();
  const f = t.landing.footer;

  const cols: { title: string; items: string[]; hrefs: string[] }[] = [
    { title: f.colProducts, items: f.products, hrefs: PRODUCT_HREFS },
    { title: f.colPlatform, items: f.platform, hrefs: PLATFORM_HREFS },
    { title: f.colDocs, items: f.docs, hrefs: DOCS_HREFS },
    { title: f.colLegal, items: f.legal, hrefs: ['#', '#', '#'] },
  ];

  return (
    <footer className="footer">
      <div className="footer-inner">
        <div className="footer-brand">
          <a className="app-brand fb-brand" href="#top" aria-label={t.brand.name}>
            <img src="/fxcore-icon.svg" alt="FXcore" className="brand-mark" width={26} height={26} />
            <span className="logo" style={{ fontSize: 19 }}>{t.brand.name}</span>
            <span className="sub">{t.brand.sub}</span>
          </a>
          <p className="footer-desc">{f.desc}</p>
        </div>
        <div className="footer-cols">
          {cols.map((col) => (
            <div key={col.title} className="footer-col">
              <h4 className="footer-col-title">{col.title}</h4>
              <ul className="footer-links">
                {col.items.map((label, i) => (
                  <li key={label}>
                    <a href={col.hrefs[i]} className="footer-link">
                      {label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
      <div className="footer-bar">
        <span>{f.rights}</span>
      </div>
    </footer>
  );
}
