// FXcore 三语 i18n：嵌套键（中文/EN/ID）。语言切换响应在 App 顶栏。
// 契约：所有用户可见文案走这里；新增 key 必须补全三语。
export type Lang = 'zh' | 'en' | 'id';

const zh = {
  brand: { name: 'FXcore', sub: 'TRADING TERMINAL' },
  nav: {
    dashboard: '仪表盘', traders: '交易员', exchanges: '交易所',
    models: '模型', strategies: '策略', backtest: '回测', debate: '辩论',
  },
  auth: {
    title: '身份验证 / AUTH',
    desc: '接入交易引擎需要先验证操作员身份。凭据经 RSA 加密后存储，响应零泄漏。',
    login: '⭘ 登录', register: '✦ 注册', create: '✦ 创建账号',
    email: 'email@example.com', nickname: '昵称（可选）',
    password: '密码', passwordHint: '密码（至少 8 位）',
    busy: '处理中…', devLogin: '⏎ dev 默认账号快捷登录',
    errEmpty: '请填写邮箱与密码', errShort: '密码至少 8 位', errExpired: '登录已过期，请重新登录',
  },
  common: {
    logout: '登出', loading: '· 加载中…', error: '错误',
    noData: '// NO DATA',
  },
  kline: {
    title: 'K线预览', symbol: '币种', interval: '周期', fetch: '拉取',
    loading: '加载 K 线中…', noData: '// NO KLINE DATA', fetchFailed: 'K 线获取失败',
    bars: '根 K 线', range: '时间范围', last: '最新',
  },
};

const en: typeof zh = {
  brand: { name: 'FXcore', sub: 'TRADING TERMINAL' },
  nav: {
    dashboard: 'Dashboard', traders: 'Traders', exchanges: 'Exchanges',
    models: 'Models', strategies: 'Strategies', backtest: 'Backtest', debate: 'Debate',
  },
  auth: {
    title: 'AUTHENTICATION / AUTH',
    desc: 'Operator identity verification is required to access the trading engine. Credentials are RSA-encrypted at rest, zero leakage in responses.',
    login: '⭘ Sign In', register: '✦ Register', create: '✦ Create Account',
    email: 'email@example.com', nickname: 'Nickname (optional)',
    password: 'Password', passwordHint: 'Password (min 8 chars)',
    busy: 'Processing…', devLogin: '⏎ dev quick login',
    errEmpty: 'Email and password required', errShort: 'Password must be at least 8 chars', errExpired: 'Session expired, please sign in again',
  },
  common: {
    logout: 'Sign Out', loading: '· Loading…', error: 'Error',
    noData: '// NO DATA',
  },
  kline: {
    title: 'K-Line Preview', symbol: 'Symbol', interval: 'Interval', fetch: 'Fetch',
    loading: 'Loading K-lines…', noData: '// NO KLINE DATA', fetchFailed: 'Failed to fetch K-lines',
    bars: 'bars', range: 'Range', last: 'Latest',
  },
};

const id: typeof zh = {
  brand: { name: 'FXcore', sub: 'TRADING TERMINAL' },
  nav: {
    dashboard: 'Dasbor', traders: 'Trader', exchanges: 'Bursa',
    models: 'Model', strategies: 'Strategi', backtest: 'Backtest', debate: 'Debat',
  },
  auth: {
    title: 'AUTENTIKASI / AUTH',
    desc: 'Verifikasi identitas operator diperlukan untuk mengakses mesin trading. Kredensial dienkripsi RSA, respons bebas kebocoran.',
    login: '⭘ Masuk', register: '✦ Daftar', create: '✦ Buat Akun',
    email: 'email@example.com', nickname: 'Nama panggilan (opsional)',
    password: 'Kata sandi', passwordHint: 'Kata sandi (min 8 karakter)',
    busy: 'Memproses…', devLogin: '⏎ login cepat dev',
    errEmpty: 'Email dan kata sandi wajib diisi', errShort: 'Kata sandi minimal 8 karakter', errExpired: 'Sesi berakhir, silakan masuk lagi',
  },
  common: {
    logout: 'Keluar', loading: '· Memuat…', error: 'Kesalahan',
    noData: '// TIDAK ADA DATA',
  },
  kline: {
    title: 'Pratinjau K-Line', symbol: 'Simbol', interval: 'Interval', fetch: 'Ambil',
    loading: 'Memuat K-line…', noData: '// TIDAK ADA DATA KLINE', fetchFailed: 'Gagal mengambil K-line',
    bars: 'batang', range: 'Rentang', last: 'Terbaru',
  },
};

export const translations: Record<Lang, typeof zh> = { zh, en, id };
export const LANGS: { key: Lang; label: string }[] = [
  { key: 'zh', label: '中文' },
  { key: 'en', label: 'EN' },
  { key: 'id', label: 'ID' },
];

export type TranslationKey = typeof zh;
