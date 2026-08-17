// FXcore 三语 i18n：嵌套键（中文/EN/ID）。语言切换响应在 App 顶栏。
// 契约：所有用户可见文案走这里；新增 key 必须补全三语。
export type Lang = 'zh' | 'en' | 'id';

const zh = {
  brand: { name: 'FXcore', sub: 'TRADING TERMINAL' },
  nav: {
    dashboard: '仪表盘', traders: '交易员', exchanges: '交易所',
    models: '模型', strategies: '策略', backtest: '回测', debate: '辩论',
  },
  landing: {
    nav: {
      features: '功能', markets: '行情', security: '安全',
    },
    hero: {
      badge: '● AI 交易引擎在线',
      title: 'AI 驱动的加密货币交易终端',
      lead: '多模型 AI 实时分析行情、执行交易策略。凭据加密存储、请求签名校验，安全与智能并重。',
      ctaPrimary: '开始交易',
      ctaSecondary: '查看行情',
    },
    trust: {
      storage: 'RSA-2048 加密存储',
      signed: 'HMAC 签名校验',
      replay: '防重放保护',
      audit: '限流与审计',
    },
    market: {
      title: '实时行情预览',
      live: 'LIVE',
      demo: '演示数据',
      high: '24h 高',
      low: '24h 低',
      change: '24h 涨跌',
      updated: '已更新',
      volume: '成交量',
    },
    security: {
      title: '安全架构',
      lead: '从存储到请求，每一环都有防护。',
      storageTitle: '凭据加密存储',
      storageDesc: 'API 密钥经 RSA-2048 公钥加密后落库，任何接口响应零泄漏。',
      signedTitle: '请求签名校验',
      signedDesc: '每次写操作携带 HMAC-SHA256 签名与时间戳，篡改请求直接拒绝。',
      replayTitle: '防重放保护',
      replayDesc: '一次性 nonce 消耗机制，已执行的请求无法被重放利用。',
      auditTitle: '分级限流与审计',
      auditDesc: '多级限流防护与全量请求审计，异常访问有迹可循。',
    },
    wallet: {
      title: '钱包与交易所集成',
      lead: '统一接入主流交易所，密钥加密托管，账户余额一目了然。',
      connected: '已连接',
      demo: '演示',
      balances: '模拟余额',
    },
    features: {
      title: '为专业交易者打造',
      lead: '从多模型 AI 决策到实时行情，专业级工具与安全并重。',
      aiTitle: '多模型 AI 决策',
      aiDesc: '多个 AI 模型同时分析行情并投票决策，输出带完整依据的交易指令。',
      marketTitle: '实时行情监控',
      marketDesc: 'K 线视图与毫秒级行情追踪，绿涨红跌一眼看清市场方向。',
      studioTitle: '策略工作室',
      studioDesc: '可视化配置交易策略与风控参数，一键激活即刻生效。',
      backtestTitle: '历史回测引擎',
      backtestDesc: '用真实历史行情回放策略，进度、权益与每笔交易全程可查。',
      debateTitle: 'AI 辩论引擎',
      debateDesc: '多智能体观点碰撞、轮次推进，置信加权达成共识决策。',
      exchangeTitle: '多交易所接入',
      exchangeDesc: '统一接入主流交易所，密钥 RSA 加密托管，余额一目了然。',
    },
    stats: {
      exchValue: '10+', exchLabel: '支持交易所',
      modelValue: '5+', modelLabel: 'AI 模型提供商',
      uptimeValue: '24/7', uptimeLabel: '全天候交易',
      refreshValue: '2s', refreshLabel: '行情刷新周期',
    },
    cta: {
      title: '开始你的 AI 交易之旅',
      lead: '创建账号，配置你的第一个 AI 交易员。凭据加密存储，安全与智能并重。',
      primary: '创建账号',
      secondary: '查看功能',
      note1: '凭据加密存储',
      note2: '零配置上手',
    },
    footer: {
      desc: 'AI 驱动的加密货币交易终端。多模型 AI 决策、实时行情、策略回测与多交易所接入。',
      colProducts: '产品', colPlatform: '平台', colDocs: '文档', colLegal: '法律',
      products: ['仪表盘', '交易员', '策略', '回测', '辩论'],
      platform: ['实时行情', '核心功能', '安全架构', '立即开始'],
      docs: ['API 文档', '系统状态', '更新日志'],
      legal: ['隐私政策', '服务条款', 'Cookie 声明'],
      rights: '© 2026 FXcore 保留所有权利',
    },
    backHome: '← 返回首页',
  },
  auth: {
    title: '身份验证 / AUTH',
    desc: '接入交易引擎需要先验证操作员身份。凭据经 RSA 加密后存储，响应零泄漏。',
    login: '⭘ 登录', register: '✦ 注册', create: '✦ 创建账号',
    email: 'email@example.com', nickname: '昵称（可选）',
    password: '密码', passwordHint: '密码（至少 8 位）',
    busy: '处理中…',
    errEmpty: '请填写邮箱与密码', errShort: '密码至少 8 位', errExpired: '登录已过期，请重新登录',
    forgot: '忘记密码？',
    forgotTitle: '找回密码', forgotDesc: '输入注册邮箱，我们会发送密码重置链接。',
    forgotSend: '发送重置邮件',
    forgotSentTitle: '邮件已发送',
    forgotSentDesc: '如果该邮箱已注册，重置链接已发送至您的邮箱，链接 1 小时内有效。',
    forgotDevNotice: '开发模式：SMTP 未配置，重置链接如下（仅开发环境返回）',
    backLogin: '← 返回登录',
    resetTitle: '设置新密码', resetDesc: '为您的账号设置新密码（至少 8 位）。',
    newPassword: '新密码', confirmPassword: '确认新密码',
    errMismatch: '两次输入的密码不一致', errTokenMissing: '缺少重置令牌，请从邮件中的链接进入。',
    resetSuccessTitle: '密码已重置', resetSuccessDesc: '新密码已生效，请使用新密码登录。',
    goLogin: '前往登录 →',
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
  legalPages: {
    backHome: '← 返回首页',
    lastUpdated: '最后更新：2026 年 8 月',
    privacy: {
      title: '隐私政策',
      sections: [
        {
          t: '1. 概述',
          b: ['FXcore 是自主加密货币交易智能体平台。本政策说明我们收集、使用与保护哪些信息，以及您对自己的数据拥有哪些权利。使用本平台即表示您同意本政策。'],
        },
        {
          t: '2. 我们收集的信息',
          b: [
            '账号信息：注册邮箱与昵称。密码以 bcrypt 哈希形式存储，永不存储明文；请求签名密钥（sign_secret）仅在登录时下发一次并保存在您浏览器的内存中，刷新页面后需重新获取。',
            '交易所凭据：您在配置交易员时提供的交易所 API Key、Secret 与 Passphrase。这些凭据使用 RSA-2048 加密后存储于服务端数据库，仅用于在您授权的交易所执行交易与查询账户。若加密私钥丢失，已存凭据将无法解密恢复，需重新录入。',
            'AI 服务配置：您选择的 AI 提供商、模型与 API Key（同样加密存储），以及您编写的策略与提示词。',
            '交易数据：订单、成交、持仓、权益快照与 AI 决策记录，以及回测与辩论会话记录，均持久化存储于服务端数据库。',
            '访问数据：服务日志记录请求时间、来源 IP、访问路径与客户端信息，用于安全审计、限流与故障排查。',
          ],
        },
        {
          t: '3. 我们如何使用信息',
          b: ['提供并维护交易服务；执行您配置的 AI 决策与策略；安全防护（身份校验、签名验证、防重放、限流）；改进产品与排查故障。'],
        },
        {
          t: '4. 数据存储与安全',
          b: [
            '数据存储于您部署服务时指定的数据库（SQLite 或 MariaDB/MySQL）；可选 Redis 仅用于令牌、限流等短期数据，未配置时自动降级为进程内存。',
            '传输层默认支持 TLS 加密；写操作经 HMAC-SHA256 签名与一次性 nonce 防重放；会话使用 15 分钟短期访问令牌（HttpOnly Cookie）与 7 天轮换刷新令牌；多用户数据严格隔离。',
          ],
        },
        {
          t: '5. 数据共享',
          b: [
            '我们不向任何第三方出售或出租个人数据。数据仅在以下场景流转：您授权的交易所（执行交易与查询账户）；您选择的 AI 服务商（发送策略提示词与市场数据用于生成决策）；以及服务运行所必需的存储基础设施。',
            'AI 决策请求将携带您配置的提示词与行情上下文发送至所选 AI 服务商，请勿在提示词中包含敏感信息。',
          ],
        },
        {
          t: '6. 数据保留与删除',
          b: ['账号存续期间保留提供服务所需的数据；登出后刷新令牌立即作废。目前平台未提供自助删除账号入口，如需删除账号与相关数据，请通过 GitHub Issues 联系我们处理；删除请求处理完成后数据不可恢复。'],
        },
        {
          t: '7. 您的权利',
          b: ['访问与更正：您可以在页面查看与编辑配置，并可随时修改密码；导出：页面展示的交易与决策记录可由您自行查阅；删除：可请求删除账号与相关数据（见上条）。'],
        },
        {
          t: '8. 未成年人',
          b: ['本平台不面向未满 18 周岁的未成年人，也不收集未成年人个人信息。'],
        },
        {
          t: '9. 政策更新',
          b: ['本政策可能随功能演进更新，更新后将在本页注明日期。重大变更将通过页面提示告知。'],
        },
        {
          t: '10. 联系我们',
          b: ['如对本政策或您的数据有任何疑问，请通过 GitHub Issues（github.com/onecany/fxcore）与我们联系。'],
        },
      ],
    },
    terms: {
      title: '服务条款',
      sections: [
        {
          t: '1. 服务说明',
          b: ['FXcore 提供 AI 驱动的加密货币交易智能体服务，包括多交易所账户管理、AI 决策执行、策略回测与多模型辩论等功能。'],
        },
        {
          t: '2. 账号与责任',
          b: ['您负责保管账号凭据与交易所 API Key 的安全，不应与他人共享账号。请求签名密钥（sign_secret）仅保存在您的浏览器内存中，请勿泄露。因凭据泄露导致的损失由您自行承担。'],
        },
        {
          t: '3. 高风险声明',
          b: [
            '加密货币交易具有极高风险，可能导致全部本金损失。AI 生成的决策不构成投资建议，您应独立判断并自行承担所有交易决策的后果。',
            '回测与辩论结果基于历史数据或模拟推演，不代表未来实际表现。',
          ],
        },
        {
          t: '4. 用户义务',
          b: ['您应确保使用本平台符合所在司法辖区的法律法规；不得利用平台漏洞、干扰服务运行或对服务进行逆向攻击；不得将平台用于任何非法活动。'],
        },
        {
          t: '5. 交易所接入',
          b: ['平台通过您提供的 API 凭据接入交易所。交易所服务中断、下线或 API 变更可能导致交易失败，平台对此不承担责任。建议为 API Key 开启最小必要权限。'],
        },
        {
          t: '6. 知识产权',
          b: ['平台软件与界面归 FXcore 项目所有；您创建的策略、提示词与配置数据归您所有。您授予平台为提供服务所必需的存储与处理授权。'],
        },
        {
          t: '7. 服务可用性',
          b: ['平台按"现状"提供，不保证服务不间断或无错误。维护、升级或不可抗力可能导致服务中断。'],
        },
        {
          t: '8. 免责声明',
          b: ['在法律允许的最大范围内，平台对因使用或无法使用本服务产生的直接或间接损失（包括交易亏损）不承担责任，除非该损失由平台故意或重大过失造成。'],
        },
        {
          t: '9. 条款变更与终止',
          b: ['我们可能更新本条款，更新后将在本页注明日期。您可随时停止使用本平台；平台亦可依约终止向违规用户提供服务。'],
        },
        {
          t: '10. 争议解决',
          b: ['如对本条款有任何疑问，请先通过 GitHub Issues（github.com/onecany/fxcore）与我们联系协商解决。'],
        },
      ],
    },
    cookies: {
      title: 'Cookie 声明',
      sections: [
        {
          t: '1. 什么是 Cookie 与本地存储',
          b: ['Cookie 是网站存储在您设备上的小型文本片段；本平台同时使用浏览器本地存储（localStorage）保存偏好与会话恢复信息。本声明说明平台使用哪些存储机制及其用途。'],
        },
        {
          t: '2. 我们使用的存储',
          b: [
            '认证 Cookie（fx_access_token）：登录后由服务端设置，有效期 15 分钟，带 HttpOnly 属性使脚本无法读取，用于识别已登录会话。',
            '刷新令牌（localStorage：fx_refresh_token）：用于在访问令牌过期后轮换续期（7 天），登出后立即清除。刷新令牌属于安全敏感数据，请勿泄露。',
            '语言偏好（localStorage：fxcore-lang）：记住您选择的界面语言。',
            '主题偏好（localStorage：fxcore-theme）：记住您选择的深色或浅色主题。',
            '请求签名密钥（sign_secret）仅存于内存，不写入任何持久化存储。',
          ],
        },
        {
          t: '3. 用途',
          b: ['以上存储全部服务于认证、会话恢复与界面偏好，均为功能必需或您主动选择，不用于广告投放、行为分析或跨站追踪。'],
        },
        {
          t: '4. 第三方存储',
          b: ['本平台不加载第三方 Cookie，不使用第三方分析或广告追踪脚本。'],
        },
        {
          t: '5. 管理您的存储',
          b: ['清除浏览器 Cookie 与站点数据将使您登出。您可以在浏览器设置中删除上述条目或禁用存储；禁用后无法保持登录与偏好，部分功能可能不可用。'],
        },
        {
          t: '6. 更新与联系',
          b: ['本声明随功能变化更新。如有疑问，请通过 GitHub Issues（github.com/onecany/fxcore）联系我们。'],
        },
      ],
    },
  },
};

const en: typeof zh = {
  brand: { name: 'FXcore', sub: 'TRADING TERMINAL' },
  nav: {
    dashboard: 'Dashboard', traders: 'Traders', exchanges: 'Exchanges',
    models: 'Models', strategies: 'Strategies', backtest: 'Backtest', debate: 'Debate',
  },
  landing: {
    nav: {
      features: 'Features', markets: 'Markets', security: 'Security',
    },
    hero: {
      badge: '● AI Trading Engine Online',
      title: 'AI-Driven Crypto Trading Terminal',
      lead: 'Multi-model AI analyzes markets and executes strategies in real time. Credentials encrypted at rest, requests signature-verified. Intelligence backed by security.',
      ctaPrimary: 'Start Trading',
      ctaSecondary: 'View Markets',
    },
    trust: {
      storage: 'RSA-2048 Encrypted Storage',
      signed: 'HMAC Signature Verified',
      replay: 'Anti-Replay Protected',
      audit: 'Rate-Limited & Audited',
    },
    market: {
      title: 'Live Market Preview',
      live: 'LIVE',
      demo: 'Demo Data',
      high: '24h High',
      low: '24h Low',
      change: '24h Change',
      updated: 'Updated',
      volume: 'Volume',
    },
    security: {
      title: 'Security Architecture',
      lead: 'Protected at every layer, from storage to requests.',
      storageTitle: 'Credential Encryption',
      storageDesc: 'API keys are RSA-2048 encrypted before storage. Zero leakage in any API response.',
      signedTitle: 'Request Signature Verification',
      signedDesc: 'Every write carries an HMAC-SHA256 signature and timestamp. Tampered requests are rejected.',
      replayTitle: 'Anti-Replay Protection',
      replayDesc: 'One-time nonce consumption — executed requests can never be replayed.',
      auditTitle: 'Rate Limits & Audit Logs',
      auditDesc: 'Multi-tier rate limiting with full request audit trails.',
    },
    wallet: {
      title: 'Wallet & Exchange Integration',
      lead: 'Unified access to major exchanges. Keys encrypted and held securely, balances at a glance.',
      connected: 'Connected',
      demo: 'Demo',
      balances: 'Simulated Balances',
    },
    features: {
      title: 'Built for Professional Traders',
      lead: 'From multi-model AI decisions to real-time markets, pro-grade tools backed by security.',
      aiTitle: 'Multi-Model AI Decisions',
      aiDesc: 'Multiple AI models analyze the market in parallel and vote on decisions, each with full reasoning.',
      marketTitle: 'Real-Time Market Monitor',
      marketDesc: 'K-line views with millisecond market tracking. Green up, red down at a glance.',
      studioTitle: 'Strategy Studio',
      studioDesc: 'Visually configure trading strategies and risk controls. Activate with one click.',
      backtestTitle: 'Historical Backtest Engine',
      backtestDesc: 'Replay strategies against real historical data with full progress and equity tracking.',
      debateTitle: 'AI Debate Engine',
      debateDesc: 'Multiple agents collide, rounds advance, confidence-weighted consensus decides.',
      exchangeTitle: 'Multi-Exchange Access',
      exchangeDesc: 'Unified access to major exchanges. Keys RSA-encrypted, balances at a glance.',
    },
    stats: {
      exchValue: '10+', exchLabel: 'Exchanges Supported',
      modelValue: '5+', modelLabel: 'AI Model Providers',
      uptimeValue: '24/7', uptimeLabel: 'Round-the-Clock Trading',
      refreshValue: '2s', refreshLabel: 'Market Refresh',
    },
    cta: {
      title: 'Start Your AI Trading Journey',
      lead: 'Create an account and configure your first AI trader. Credentials encrypted at rest, security and intelligence in one.',
      primary: 'Create Account',
      secondary: 'View Features',
      note1: 'Encrypted credential storage',
      note2: 'Zero-config setup',
    },
    footer: {
      desc: 'AI-driven crypto trading terminal. Multi-model AI decisions, real-time markets, backtesting and multi-exchange access.',
      colProducts: 'Products', colPlatform: 'Platform', colDocs: 'Docs', colLegal: 'Legal',
      products: ['Dashboard', 'Traders', 'Strategies', 'Backtest', 'Debate'],
      platform: ['Live Markets', 'Core Features', 'Security', 'Get Started'],
      docs: ['API Docs', 'System Status', 'Changelog'],
      legal: ['Privacy Policy', 'Terms of Service', 'Cookie Policy'],
      rights: '© 2026 FXcore. All rights reserved.',
    },
    backHome: '← Back to Home',
  },
  auth: {
    title: 'AUTHENTICATION / AUTH',
    desc: 'Operator identity verification is required to access the trading engine. Credentials are RSA-encrypted at rest, zero leakage in responses.',
    login: '⭘ Sign In', register: '✦ Register', create: '✦ Create Account',
    email: 'email@example.com', nickname: 'Nickname (optional)',
    password: 'Password', passwordHint: 'Password (min 8 chars)',
    busy: 'Processing…',
    errEmpty: 'Email and password required', errShort: 'Password must be at least 8 chars', errExpired: 'Session expired, please sign in again',
    forgot: 'Forgot password?',
    forgotTitle: 'Reset Password', forgotDesc: 'Enter your registered email and we will send you a password reset link.',
    forgotSend: 'Send reset email',
    forgotSentTitle: 'Email sent',
    forgotSentDesc: 'If the email is registered, a reset link has been sent to your inbox. The link expires in 1 hour.',
    forgotDevNotice: 'Dev mode: SMTP not configured, reset link below (dev only)',
    backLogin: '← Back to login',
    resetTitle: 'Set New Password', resetDesc: 'Set a new password for your account (at least 8 characters).',
    newPassword: 'New password', confirmPassword: 'Confirm new password',
    errMismatch: 'Passwords do not match', errTokenMissing: 'Missing reset token. Please open the link from your email.',
    resetSuccessTitle: 'Password Reset', resetSuccessDesc: 'Your new password is active. Please sign in with it.',
    goLogin: 'Go to login →',
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
  legalPages: {
    backHome: '← Back to Home',
    lastUpdated: 'Last updated: August 2026',
    privacy: {
      title: 'Privacy Policy',
      sections: [
        {
          t: '1. Overview',
          b: ['FXcore is an autonomous crypto trading agent platform. This policy explains what information we collect, how we use and protect it, and the rights you have over your data. By using the platform you agree to this policy.'],
        },
        {
          t: '2. Information We Collect',
          b: [
            'Account information: your registration email and nickname. Passwords are stored as bcrypt hashes and never in plain text; the request signing secret (sign_secret) is issued once at login and kept in your browser memory only, requiring re-issuance after a page refresh.',
            'Exchange credentials: the exchange API Key, Secret and Passphrase you provide when configuring a trader. These are encrypted with RSA-2048 before being stored in the server database and are used only to execute trades and query accounts on the exchanges you authorize. If the encryption private key is lost, stored credentials cannot be recovered and must be re-entered.',
            'AI service configuration: the AI providers, models and API keys you select (also encrypted at rest), as well as the strategies and prompts you author.',
            'Trading data: orders, fills, positions, equity snapshots and AI decision records, along with backtest and debate session records, all persisted in the server database.',
            'Access data: server logs record request time, source IP, request path and client information for security auditing, rate limiting and troubleshooting.',
          ],
        },
        {
          t: '3. How We Use Information',
          b: ['To provide and maintain the trading service; to execute the AI decisions and strategies you configure; for security protection (identity verification, signature validation, replay protection, rate limiting); and to improve the product and troubleshoot issues.'],
        },
        {
          t: '4. Data Storage and Security',
          b: [
            'Data is stored in the database you choose when deploying the service (SQLite or MariaDB/MySQL); the optional Redis is used only for short-lived data such as tokens and rate limiting, falling back to in-process memory when not configured.',
            'Transport encryption (TLS) is supported by default; write operations are protected by HMAC-SHA256 signatures and one-time nonces against replay; sessions use a 15-minute short-lived access token (HttpOnly Cookie) with a 7-day rotating refresh token; user data is strictly isolated.',
          ],
        },
        {
          t: '5. Data Sharing',
          b: [
            'We do not sell or rent personal data to any third party. Data flows only in the following cases: to exchanges you authorize (executing trades and querying accounts); to the AI providers you select (sending your strategy prompt and market data to generate decisions); and to the storage infrastructure required to run the service.',
            'AI decision requests carry the prompt and market context you configured to the selected AI provider. Please do not include sensitive information in prompts.',
          ],
        },
        {
          t: '6. Data Retention and Deletion',
          b: ['Data required to provide the service is retained while your account is active; refresh tokens are invalidated immediately upon logout. The platform does not currently offer self-service account deletion. To request deletion of your account and related data, contact us via GitHub Issues; once processed, the data cannot be recovered.'],
        },
        {
          t: '7. Your Rights',
          b: ['Access and correction: you can view and edit your configuration on the pages and change your password at any time. Export: trading and decision records shown on the pages can be reviewed by you directly. Deletion: you may request deletion of your account and related data (see above).'],
        },
        {
          t: '8. Minors',
          b: ['The platform is not directed at individuals under the age of 18, and we do not knowingly collect personal information from minors.'],
        },
        {
          t: '9. Policy Updates',
          b: ['This policy may be updated as the product evolves; updates will be dated on this page. Material changes will be announced in the interface.'],
        },
        {
          t: '10. Contact Us',
          b: ['If you have any questions about this policy or your data, contact us via GitHub Issues (github.com/onecany/fxcore).'],
        },
      ],
    },
    terms: {
      title: 'Terms of Service',
      sections: [
        {
          t: '1. Service Description',
          b: ['FXcore provides an AI-driven crypto trading agent service, including multi-exchange account management, AI decision execution, strategy backtesting and multi-model debate.'],
        },
        {
          t: '2. Account and Responsibility',
          b: ['You are responsible for safeguarding your account credentials and exchange API keys and must not share your account with others. The request signing secret (sign_secret) is kept only in your browser memory; do not disclose it. Losses caused by credential leakage are your responsibility.'],
        },
        {
          t: '3. High-Risk Statement',
          b: [
            'Cryptocurrency trading is extremely risky and may result in the loss of your entire capital. AI-generated decisions do not constitute investment advice. You should make your own judgments and bear all consequences of your trading decisions.',
            'Backtest and debate results are based on historical data or simulated reasoning and do not represent future performance.',
          ],
        },
        {
          t: '4. User Obligations',
          b: ['You must ensure your use of the platform complies with the laws of your jurisdiction; you must not exploit platform vulnerabilities, disrupt service operation or reverse-engineer the service; you must not use the platform for any illegal activity.'],
        },
        {
          t: '5. Exchange Integration',
          b: ['The platform connects to exchanges using the API credentials you provide. Exchange outages, delistings or API changes may cause trade failures for which the platform is not responsible. We recommend granting API keys only the minimum required permissions.'],
        },
        {
          t: '6. Intellectual Property',
          b: ['The platform software and interface belong to the FXcore project; the strategies, prompts and configuration data you create belong to you. You grant the platform the storage and processing authorization necessary to provide the service.'],
        },
        {
          t: '7. Service Availability',
          b: ['The platform is provided "as is" and is not guaranteed to be uninterrupted or error-free. Maintenance, upgrades or force majeure may cause service interruptions.'],
        },
        {
          t: '8. Disclaimer',
          b: ['To the maximum extent permitted by law, the platform is not liable for direct or indirect losses (including trading losses) arising from the use of or inability to use the service, unless such losses are caused by the platform\'s willful misconduct or gross negligence.'],
        },
        {
          t: '9. Changes and Termination',
          b: ['We may update these terms; updates will be dated on this page. You may stop using the platform at any time, and the platform may terminate service to users who violate these terms.'],
        },
        {
          t: '10. Dispute Resolution',
          b: ['If you have any questions about these terms, contact us first via GitHub Issues (github.com/onecany/fxcore) to resolve them.'],
        },
      ],
    },
    cookies: {
      title: 'Cookie Policy',
      sections: [
        {
          t: '1. What Are Cookies and Local Storage',
          b: ['Cookies are small text pieces a website stores on your device; the platform also uses browser local storage (localStorage) to save preferences and session recovery information. This statement explains which storage mechanisms the platform uses and for what purpose.'],
        },
        {
          t: '2. Storage We Use',
          b: [
            'Authentication cookie (fx_access_token): set by the server after login, valid for 15 minutes, HttpOnly so scripts cannot read it, used to identify the signed-in session.',
            'Refresh token (localStorage: fx_refresh_token): used to rotate and renew the session after the access token expires (7 days), cleared immediately on logout. It is security-sensitive; do not disclose it.',
            'Language preference (localStorage: fxcore-lang): remembers the interface language you choose.',
            'Theme preference (localStorage: fxcore-theme): remembers the dark or light theme you choose.',
            'The request signing secret (sign_secret) lives in memory only and is never written to persistent storage.',
          ],
        },
        {
          t: '3. Purposes',
          b: ['All of the storage above serves authentication, session recovery and interface preferences. It is either functionally required or explicitly chosen by you; it is not used for advertising, behavioral analytics or cross-site tracking.'],
        },
        {
          t: '4. Third-Party Storage',
          b: ['The platform loads no third-party cookies and uses no third-party analytics or advertising tracking scripts.'],
        },
        {
          t: '5. Managing Your Storage',
          b: ['Clearing browser cookies and site data will sign you out. You can delete the entries above or disable storage in your browser settings; disabling them means logins and preferences cannot be kept, and some features may be unavailable.'],
        },
        {
          t: '6. Updates and Contact',
          b: ['This statement is updated as features change. For questions, contact us via GitHub Issues (github.com/onecany/fxcore).'],
        },
      ],
    },
  },
};

const id: typeof zh = {
  brand: { name: 'FXcore', sub: 'TRADING TERMINAL' },
  nav: {
    dashboard: 'Dasbor', traders: 'Trader', exchanges: 'Bursa',
    models: 'Model', strategies: 'Strategi', backtest: 'Backtest', debate: 'Debat',
  },
  landing: {
    nav: {
      features: 'Fitur', markets: 'Pasar', security: 'Keamanan',
    },
    hero: {
      badge: '● Mesin Trading AI Online',
      title: 'Terminal Trading Kripto Berbasis AI',
      lead: 'AI multi-model menganalisis pasar dan mengeksekusi strategi secara real time. Kredensial terenkripsi, permintaan terverifikasi tanda tangan. Kecerdasan berpadu keamanan.',
      ctaPrimary: 'Mulai Trading',
      ctaSecondary: 'Lihat Pasar',
    },
    trust: {
      storage: 'Penyimpanan Terenkripsi RSA-2048',
      signed: 'Verifikasi Tanda Tangan HMAC',
      replay: 'Perlindungan Anti-Replay',
      audit: 'Pembatasan & Audit',
    },
    market: {
      title: 'Pratinjau Pasar Langsung',
      live: 'LANGSUNG',
      demo: 'Data Demo',
      high: 'Tinggi 24 jam',
      low: 'Rendah 24 jam',
      change: 'Perubahan 24 jam',
      updated: 'Diperbarui',
      volume: 'Volume',
    },
    security: {
      title: 'Arsitektur Keamanan',
      lead: 'Terlindungi di setiap lapisan, dari penyimpanan hingga permintaan.',
      storageTitle: 'Enkripsi Kredensial',
      storageDesc: 'Kunci API dienkripsi RSA-2048 sebelum disimpan. Nol kebocoran dalam respons API.',
      signedTitle: 'Verifikasi Tanda Tangan Permintaan',
      signedDesc: 'Setiap operasi tulis membawa tanda tangan HMAC-SHA256 dan stempel waktu. Permintaan yang diubah ditolak.',
      replayTitle: 'Perlindungan Anti-Replay',
      replayDesc: 'Konsumsi nonce sekali pakai — permintaan yang dieksekusi tidak dapat diputar ulang.',
      auditTitle: 'Pembatasan Laju & Log Audit',
      auditDesc: 'Pembatasan laju berlapis dengan jejak audit permintaan lengkap.',
    },
    wallet: {
      title: 'Integrasi Dompet & Bursa',
      lead: 'Akses terpadu ke bursa utama. Kunci terenkripsi, saldo terlihat sekilas.',
      connected: 'Terhubung',
      demo: 'Demo',
      balances: 'Saldo Simulasi',
    },
    features: {
      title: 'Dibangun untuk Trader Profesional',
      lead: 'Dari keputusan AI multi-model hingga pasar real-time, alat kelas profesional dengan keamanan.',
      aiTitle: 'Keputusan AI Multi-Model',
      aiDesc: 'Beberapa model AI menganalisis pasar secara paralel dan memilih keputusan, masing-masing dengan alasan lengkap.',
      marketTitle: 'Pemantau Pasar Real-Time',
      marketDesc: 'Tampilan K-line dengan pelacakan pasar milidetik. Hijau naik, merah turun sekilas.',
      studioTitle: 'Studio Strategi',
      studioDesc: 'Konfigurasi strategi trading dan kontrol risiko secara visual. Aktifkan sekali klik.',
      backtestTitle: 'Mesin Backtest Historis',
      backtestDesc: 'Putar ulang strategi pada data historis nyata dengan pelacakan progres dan ekuitas lengkap.',
      debateTitle: 'Mesin Debat AI',
      debateDesc: 'Beberapa agen beradu pandangan, putaran berjalan, konsensus berbobot keyakinan memutuskan.',
      exchangeTitle: 'Akses Multi-Bursa',
      exchangeDesc: 'Akses terpadu ke bursa utama. Kunci terenkripsi RSA, saldo terlihat sekilas.',
    },
    stats: {
      exchValue: '10+', exchLabel: 'Bursa Didukung',
      modelValue: '5+', modelLabel: 'Penyedia Model AI',
      uptimeValue: '24/7', uptimeLabel: 'Trading Sepanjang Waktu',
      refreshValue: '2s', refreshLabel: 'Pembaruan Pasar',
    },
    cta: {
      title: 'Mulai Perjalanan Trading AI Anda',
      lead: 'Buat akun dan konfigurasi trader AI pertama Anda. Kredensial terenkripsi, keamanan dan kecerdasan dalam satu.',
      primary: 'Buat Akun',
      secondary: 'Lihat Fitur',
      note1: 'Penyimpanan kredensial terenkripsi',
      note2: 'Setup tanpa konfigurasi',
    },
    footer: {
      desc: 'Terminal trading kripto berbasis AI. Keputusan AI multi-model, pasar real-time, backtest dan akses multi-bursa.',
      colProducts: 'Produk', colPlatform: 'Platform', colDocs: 'Dokumen', colLegal: 'Hukum',
      products: ['Dasbor', 'Trader', 'Strategi', 'Backtest', 'Debat'],
      platform: ['Pasar Langsung', 'Fitur Inti', 'Keamanan', 'Mulai'],
      docs: ['Dokumen API', 'Status Sistem', 'Catatan Perubahan'],
      legal: ['Kebijakan Privasi', 'Ketentuan Layanan', 'Kebijakan Cookie'],
      rights: '© 2026 FXcore. Hak cipta dilindungi.',
    },
    backHome: '← Kembali ke Beranda',
  },
  auth: {
    title: 'AUTENTIKASI / AUTH',
    desc: 'Verifikasi identitas operator diperlukan untuk mengakses mesin trading. Kredensial dienkripsi RSA, respons bebas kebocoran.',
    login: '⭘ Masuk', register: '✦ Daftar', create: '✦ Buat Akun',
    email: 'email@example.com', nickname: 'Nama panggilan (opsional)',
    password: 'Kata sandi', passwordHint: 'Kata sandi (min 8 karakter)',
    busy: 'Memproses…',
    errEmpty: 'Email dan kata sandi wajib diisi', errShort: 'Kata sandi minimal 8 karakter', errExpired: 'Sesi berakhir, silakan masuk lagi',
    forgot: 'Lupa kata sandi?',
    forgotTitle: 'Reset Kata Sandi', forgotDesc: 'Masukkan email terdaftar, kami akan mengirimkan tautan reset kata sandi.',
    forgotSend: 'Kirim email reset',
    forgotSentTitle: 'Email terkirim',
    forgotSentDesc: 'Jika email terdaftar, tautan reset telah dikirim ke inbox Anda. Tautan berlaku 1 jam.',
    forgotDevNotice: 'Mode pengembangan: SMTP tidak dikonfigurasi, tautan reset di bawah (khusus dev)',
    backLogin: '← Kembali ke login',
    resetTitle: 'Atur Kata Sandi Baru', resetDesc: 'Atur kata sandi baru untuk akun Anda (minimal 8 karakter).',
    newPassword: 'Kata sandi baru', confirmPassword: 'Konfirmasi kata sandi baru',
    errMismatch: 'Kata sandi tidak cocok', errTokenMissing: 'Token reset tidak ada. Buka tautan dari email Anda.',
    resetSuccessTitle: 'Kata Sandi Direset', resetSuccessDesc: 'Kata sandi baru Anda aktif. Silakan masuk dengannya.',
    goLogin: 'Ke halaman login →',
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
  legalPages: {
    backHome: '← Kembali ke Beranda',
    lastUpdated: 'Terakhir diperbarui: Agustus 2026',
    privacy: {
      title: 'Kebijakan Privasi',
      sections: [
        {
          t: '1. Ringkasan',
          b: ['FXcore adalah platform agen trading kripto otonom. Kebijakan ini menjelaskan informasi apa yang kami kumpulkan, bagaimana kami menggunakan dan melindunginya, serta hak Anda atas data Anda. Dengan menggunakan platform ini Anda menyetujui kebijakan ini.'],
        },
        {
          t: '2. Informasi yang Kami Kumpulkan',
          b: [
            'Informasi akun: email pendaftaran dan nama panggilan. Kata sandi disimpan sebagai hash bcrypt dan tidak pernah dalam teks biasa; kunci penandatanganan permintaan (sign_secret) hanya diterbitkan sekali saat masuk dan disimpan di memori browser Anda, sehingga perlu diterbitkan ulang setelah halaman dimuat ulang.',
            'Kredensial bursa: API Key, Secret dan Passphrase bursa yang Anda berikan saat mengonfigurasi trader. Kredensial ini dienkripsi dengan RSA-2048 sebelum disimpan di basis data server dan hanya digunakan untuk mengeksekusi trading dan memeriksa akun di bursa yang Anda otorisasi. Jika kunci privat enkripsi hilang, kredensial yang tersimpan tidak dapat dipulihkan dan harus dimasukkan ulang.',
            'Konfigurasi layanan AI: penyedia AI, model dan API key yang Anda pilih (juga dienkripsi saat disimpan), serta strategi dan prompt yang Anda buat.',
            'Data trading: order, pengisian, posisi, snapshot ekuitas dan catatan keputusan AI, beserta catatan sesi backtest dan debat, semuanya disimpan secara persisten di basis data server.',
            'Data akses: log server mencatat waktu permintaan, IP sumber, jalur permintaan dan informasi klien untuk audit keamanan, pembatasan laju dan pemecahan masalah.',
          ],
        },
        {
          t: '3. Cara Kami Menggunakan Informasi',
          b: ['Untuk menyediakan dan memelihara layanan trading; untuk mengeksekusi keputusan AI dan strategi yang Anda konfigurasi; untuk perlindungan keamanan (verifikasi identitas, validasi tanda tangan, perlindungan replay, pembatasan laju); serta untuk meningkatkan produk dan memecahkan masalah.'],
        },
        {
          t: '4. Penyimpanan dan Keamanan Data',
          b: [
            'Data disimpan di basis data yang Anda pilih saat men-deploy layanan (SQLite atau MariaDB/MySQL); Redis opsional hanya digunakan untuk data berumur pendek seperti token dan pembatasan laju, dengan penggantian memori proses jika tidak dikonfigurasi.',
            'Enkripsi transport (TLS) didukung secara bawaan; operasi tulis dilindungi tanda tangan HMAC-SHA256 dan nonce sekali pakai terhadap replay; sesi menggunakan token akses jangka pendek 15 menit (Cookie HttpOnly) dengan token penyegar rotasi 7 hari; data pengguna diisolasi secara ketat.',
          ],
        },
        {
          t: '5. Berbagi Data',
          b: [
            'Kami tidak menjual atau menyewakan data pribadi kepada pihak ketiga mana pun. Data hanya mengalir dalam kasus berikut: ke bursa yang Anda otorisasi (mengeksekusi trading dan memeriksa akun); ke penyedia AI yang Anda pilih (mengirim prompt strategi dan data pasar untuk menghasilkan keputusan); dan ke infrastruktur penyimpanan yang diperlukan untuk menjalankan layanan.',
            'Permintaan keputusan AI membawa prompt dan konteks pasar yang Anda konfigurasi ke penyedia AI yang dipilih. Jangan sertakan informasi sensitif dalam prompt.',
          ],
        },
        {
          t: '6. Retensi dan Penghapusan Data',
          b: ['Data yang diperlukan untuk menyediakan layanan disimpan selama akun Anda aktif; token penyegar langsung dicabut saat keluar. Platform saat ini tidak menyediakan penghapusan akun mandiri. Untuk meminta penghapusan akun dan data terkait, hubungi kami melalui GitHub Issues; setelah diproses, data tidak dapat dipulihkan.'],
        },
        {
          t: '7. Hak Anda',
          b: ['Akses dan koreksi: Anda dapat melihat dan mengedit konfigurasi Anda di halaman serta mengubah kata sandi kapan saja. Ekspor: catatan trading dan keputusan yang ditampilkan di halaman dapat Anda tinjau langsung. Penghapusan: Anda dapat meminta penghapusan akun dan data terkait (lihat di atas).'],
        },
        {
          t: '8. Anak di Bawah Umur',
          b: ['Platform ini tidak ditujukan untuk individu di bawah 18 tahun, dan kami tidak secara sengaja mengumpulkan informasi pribadi dari anak di bawah umur.'],
        },
        {
          t: '9. Pembaruan Kebijakan',
          b: ['Kebijakan ini dapat diperbarui seiring evolusi produk; pembaruan akan diberi tanggal di halaman ini. Perubahan besar akan diumumkan di antarmuka.'],
        },
        {
          t: '10. Hubungi Kami',
          b: ['Jika Anda memiliki pertanyaan tentang kebijakan ini atau data Anda, hubungi kami melalui GitHub Issues (github.com/onecany/fxcore).'],
        },
      ],
    },
    terms: {
      title: 'Ketentuan Layanan',
      sections: [
        {
          t: '1. Deskripsi Layanan',
          b: ['FXcore menyediakan layanan agen trading kripto berbasis AI, termasuk manajemen akun multi-bursa, eksekusi keputusan AI, backtest strategi dan debat multi-model.'],
        },
        {
          t: '2. Akun dan Tanggung Jawab',
          b: ['Anda bertanggung jawab menjaga keamanan kredensial akun dan API key bursa, dan tidak boleh membagikan akun Anda kepada orang lain. Kunci penandatanganan permintaan (sign_secret) hanya disimpan di memori browser Anda; jangan membocorkannya. Kerugian akibat kebocoran kredensial menjadi tanggung jawab Anda.'],
        },
        {
          t: '3. Pernyataan Risiko Tinggi',
          b: [
            'Trading mata uang kripto sangat berisiko dan dapat mengakibatkan hilangnya seluruh modal Anda. Keputusan yang dihasilkan AI bukan merupakan nasihat investasi. Anda harus membuat penilaian sendiri dan menanggung semua konsekuensi dari keputusan trading Anda.',
            'Hasil backtest dan debat didasarkan pada data historis atau penalaran simulasi dan tidak mewakili kinerja masa depan.',
          ],
        },
        {
          t: '4. Kewajiban Pengguna',
          b: ['Anda harus memastikan penggunaan platform ini mematuhi hukum di wilayah hukum Anda; Anda tidak boleh mengeksploitasi kerentanan platform, mengganggu operasi layanan atau melakukan rekayasa balik layanan; Anda tidak boleh menggunakan platform untuk aktivitas ilegal apa pun.'],
        },
        {
          t: '5. Integrasi Bursa',
          b: ['Platform terhubung ke bursa menggunakan kredensial API yang Anda berikan. Gangguan bursa, penghapusan pencatatan atau perubahan API dapat menyebabkan kegagalan trading yang bukan tanggung jawab platform. Kami menyarankan memberikan API key hanya izin minimum yang diperlukan.'],
        },
        {
          t: '6. Kekayaan Intelektual',
          b: ['Perangkat lunak dan antarmuka platform milik proyek FXcore; strategi, prompt dan data konfigurasi yang Anda buat milik Anda. Anda memberikan otorisasi penyimpanan dan pemrosesan yang diperlukan untuk menyediakan layanan.'],
        },
        {
          t: '7. Ketersediaan Layanan',
          b: ['Platform disediakan "sebagaimana adanya" dan tidak dijamin tidak terputus atau bebas kesalahan. Pemeliharaan, peningkatan atau force majeure dapat menyebabkan gangguan layanan.'],
        },
        {
          t: '8. Penyangkalan',
          b: ['Sepanjang diizinkan oleh hukum, platform tidak bertanggung jawab atas kerugian langsung atau tidak langsung (termasuk kerugian trading) yang timbul dari penggunaan atau ketidakmampuan menggunakan layanan, kecuali kerugian tersebut disebabkan oleh kesengajaan atau kelalaian berat platform.'],
        },
        {
          t: '9. Perubahan dan Penghentian',
          b: ['Kami dapat memperbarui ketentuan ini; pembaruan akan diberi tanggal di halaman ini. Anda dapat berhenti menggunakan platform kapan saja, dan platform dapat menghentikan layanan kepada pengguna yang melanggar ketentuan ini.'],
        },
        {
          t: '10. Penyelesaian Sengketa',
          b: ['Jika Anda memiliki pertanyaan tentang ketentuan ini, hubungi kami terlebih dahulu melalui GitHub Issues (github.com/onecany/fxcore) untuk menyelesaikannya.'],
        },
      ],
    },
    cookies: {
      title: 'Kebijakan Cookie',
      sections: [
        {
          t: '1. Apa Itu Cookie dan Penyimpanan Lokal',
          b: ['Cookie adalah potongan teks kecil yang disimpan situs web di perangkat Anda; platform juga menggunakan penyimpanan lokal browser (localStorage) untuk menyimpan preferensi dan informasi pemulihan sesi. Pernyataan ini menjelaskan mekanisme penyimpanan yang digunakan platform dan tujuannya.'],
        },
        {
          t: '2. Penyimpanan yang Kami Gunakan',
          b: [
            'Cookie autentikasi (fx_access_token): disetel oleh server setelah masuk, berlaku 15 menit, HttpOnly sehingga skrip tidak dapat membacanya, digunakan untuk mengidentifikasi sesi yang masuk.',
            'Token penyegar (localStorage: fx_refresh_token): digunakan untuk merotasi dan memperbarui sesi setelah token akses kedaluwarsa (7 hari), langsung dihapus saat keluar. Ini sensitif terhadap keamanan; jangan membocorkannya.',
            'Preferensi bahasa (localStorage: fxcore-lang): mengingat bahasa antarmuka yang Anda pilih.',
            'Preferensi tema (localStorage: fxcore-theme): mengingat tema gelap atau terang yang Anda pilih.',
            'Kunci penandatanganan permintaan (sign_secret) hanya ada di memori dan tidak pernah ditulis ke penyimpanan persisten.',
          ],
        },
        {
          t: '3. Tujuan',
          b: ['Semua penyimpanan di atas melayani autentikasi, pemulihan sesi dan preferensi antarmuka. Semuanya diperlukan secara fungsional atau dipilih secara eksplisit oleh Anda; tidak digunakan untuk iklan, analitik perilaku atau pelacakan lintas situs.'],
        },
        {
          t: '4. Penyimpanan Pihak Ketiga',
          b: ['Platform tidak memuat cookie pihak ketiga dan tidak menggunakan skrip analitik atau pelacakan iklan pihak ketiga.'],
        },
        {
          t: '5. Mengelola Penyimpanan Anda',
          b: ['Menghapus cookie dan data situs browser akan membuat Anda keluar. Anda dapat menghapus entri di atas atau menonaktifkan penyimpanan di pengaturan browser; dengan menonaktifkannya, login dan preferensi tidak dapat dipertahankan, dan sebagian fitur mungkin tidak tersedia.'],
        },
        {
          t: '6. Pembaruan dan Kontak',
          b: ['Pernyataan ini diperbarui seiring perubahan fitur. Untuk pertanyaan, hubungi kami melalui GitHub Issues (github.com/onecany/fxcore).'],
        },
      ],
    },
  },
};

export const translations: Record<Lang, typeof zh> = { zh, en, id };
export const LANGS: { key: Lang; label: string }[] = [
  { key: 'zh', label: '中文' },
  { key: 'en', label: 'EN' },
  { key: 'id', label: 'ID' },
];

export type TranslationKey = typeof zh;
