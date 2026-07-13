import styles from './index.scss';

// base 路径：生产环境由 cross-env 注入 CROSS_BASE=/pvm/，开发环境默认为 /
const base = typeof process !== 'undefined' && process.env.CROSS_BASE ? process.env.CROSS_BASE : '/';

/* Types */
interface FeatureItem {
  icon: React.ReactNode;
  title: string;
  desc: string;
  link: string;
}

interface RuntimeItem {
  name: string;
  scope: string;
  color: string;
}

interface StepItem {
  no: string;
  title: string;
  desc: string;
  code: string;
}

interface LevelItem {
  level: string;
  source: string;
  desc: string;
}

/* 内联图标 */
const IconInstall = function () {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="7 10 12 15 17 10" />
      <line x1="12" y1="15" x2="12" y2="3" />
    </svg>
  );
};

const IconSwap = function () {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="17 1 21 5 17 9" />
      <path d="M3 11V9a4 4 0 0 1 4-4h14" />
      <polyline points="7 23 3 19 7 15" />
      <path d="M21 13v2a4 4 0 0 1-4 4H3" />
    </svg>
  );
};

const IconFolder = function () {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
      <polyline points="9 13 12 16 15 13" />
      <line x1="12" y1="16" x2="12" y2="9" />
    </svg>
  );
};

const IconShield = function () {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="M9 12l2 2 4-4" />
    </svg>
  );
};

const IconBolt = function () {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
    </svg>
  );
};

const IconMirror = function () {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18" />
      <path d="M12 3a15 15 0 0 1 0 18a15 15 0 0 1 0-18z" />
    </svg>
  );
};

const IconArrow = function () {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="5" y1="12" x2="19" y2="12" />
      <polyline points="12 5 19 12 12 19" />
    </svg>
  );
};

/* Hero 插画 — 版本管理语义：版本栈 + 激活 + shim 转发 */
const HeroIllustration = function () {
  return (
    <svg className={styles.heroSvg} viewBox="0 0 460 380" fill="none" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <radialGradient id="glow" cx="50%" cy="40%" r="60%">
          <stop offset="0%" stopColor="#165dff" stopOpacity="0.16" />
          <stop offset="100%" stopColor="#165dff" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="cardActive" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#165dff" />
          <stop offset="100%" stopColor="#3a7bff" />
        </linearGradient>
      </defs>

      <ellipse cx="220" cy="170" rx="220" ry="180" fill="url(#glow)" />

      {/* 版本栈 */}
      <g transform="translate(96 160)">
        <rect width="196" height="52" rx="10" fill="var(--rp-c-bg)" stroke="var(--rp-c-border)" strokeWidth="1.5" />
        <circle cx="24" cy="26" r="9" fill="#165dff" opacity="0.15" />
        <rect x="42" y="18" width="70" height="6" rx="3" fill="var(--rp-c-text-3)" opacity="0.35" />
        <rect x="42" y="30" width="44" height="5" rx="2.5" fill="var(--rp-c-text-3)" opacity="0.2" />
      </g>
      <g transform="translate(84 126)">
        <rect width="196" height="52" rx="10" fill="var(--rp-c-bg)" stroke="var(--rp-c-border)" strokeWidth="1.5" />
        <circle cx="24" cy="26" r="9" fill="#165dff" opacity="0.25" />
        <rect x="42" y="18" width="70" height="6" rx="3" fill="var(--rp-c-text-3)" opacity="0.45" />
        <rect x="42" y="30" width="44" height="5" rx="2.5" fill="var(--rp-c-text-3)" opacity="0.25" />
      </g>
      <g transform="translate(72 92)">
        <rect width="200" height="56" rx="12" fill="url(#cardActive)" />
        <circle cx="26" cy="28" r="10" fill="#fff" opacity="0.25" />
        <path d="M22 28 l3 3 l5-6" stroke="#fff" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" fill="none" />
        <text x="46" y="27" fontSize="15" fontWeight="600" fill="#fff">node 20.11.0</text>
        <text x="46" y="44" fontSize="11" fill="#fff" opacity="0.8">active · shim → resolve</text>
        <rect x="154" y="10" width="34" height="16" rx="8" fill="#fff" opacity="0.22" />
        <text x="162" y="21" fontSize="9" fontWeight="600" fill="#fff">NOW</text>
      </g>

      {/* 箭头 */}
      <path d="M286 158 L328 158" stroke="#165dff" strokeWidth="2.5" strokeLinecap="round" />
      <path d="M318 150 L330 158 L318 166" stroke="#165dff" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />

      {/* shim 框 */}
      <g transform="translate(336 128)">
        <rect width="104" height="64" rx="12" fill="var(--rp-c-bg)" stroke="var(--rp-c-border)" strokeWidth="1.5" />
        <rect x="0" y="0" width="104" height="22" rx="12" fill="var(--rp-c-bg-soft)" />
        <rect y="10" width="104" height="12" fill="var(--rp-c-bg-soft)" />
        <text x="12" y="15" fontSize="10" fontWeight="600" fill="var(--rp-c-text-2)">~/.pvm/shims</text>
        <rect x="14" y="34" width="76" height="8" rx="4" fill="#165dff" opacity="0.18" />
        <rect x="14" y="48" width="48" height="6" rx="3" fill="var(--rp-c-text-3)" opacity="0.2" />
      </g>

      <circle cx="64" cy="262" r="4" fill="#165dff" opacity="0.25" />
      <circle cx="404" cy="250" r="6" fill="#165dff" opacity="0.18" />
      <circle cx="120" cy="312" r="3" fill="#165dff" opacity="0.22" />
      <circle cx="366" cy="74" r="3" fill="#165dff" opacity="0.3" />
    </svg>
  );
};

/* 命令演示条 */
const HeroCode = function () {
  return (
    <div className={styles.heroCode}>
      <div className={styles.heroCodeHeader}>
        <span className={styles.heroCodeDot} style={{ background: '#ff5f57' }} />
        <span className={styles.heroCodeDot} style={{ background: '#febc2e' }} />
        <span className={styles.heroCodeDot} style={{ background: '#28c840' }} />
        <span className={styles.heroCodeLang}>bash</span>
      </div>
      <pre className={styles.heroCodeBody}>
        <code>
          <span className={styles.codeComment}># 安装并锁定项目版本</span>
          {'\n'}
          <span className={styles.codePrompt}>$</span> pvm install node@20.11.0
          {'\n'}
          <span className={styles.codePrompt}>$</span> pvm use node@20.11.0
          {'\n'}
          <span className={styles.codeComment}># ✓ 已切换到 node 20.11.0</span>
        </code>
      </pre>
    </div>
  );
};

/* Hero */
const Hero = function () {
  return (
    <section className={styles.hero}>
      <div className={styles.heroGlow} />
      <div className={styles.heroInner}>
        <div className={styles.heroLeft}>
          <div className={styles.heroBrand}>
            <span className={styles.heroBrandName}>PVM</span>
            <span className={styles.heroBrandSub}>Polyglot Version Manager</span>
          </div>
          <div className={styles.heroBadge}>
            <span className={styles.heroBadgeDot} />
            基于 shim 驱动的多语言版本管理器
          </div>
          <h1 className={styles.heroTitle}>
            一个工具
            <br />
            管好<span className={styles.heroTitleAccent}>所有运行时</span>
          </h1>
          <p className={styles.heroSubtitle}>
            node · python · go · git · pnpm · yarn
            <br />
            基于 shim 驱动，无需管理员权限，项目级版本隔离
          </p>
          <div className={styles.heroActions}>
            <a href={`${base}guide/quick-start`} className={styles.btnPrimary}>
              快速开始 <IconArrow />
            </a>
            <a href={`${base}commands/install`} className={styles.btnLink}>
              命令参考
            </a>
            <a href={`${base}config/pvmrc`} className={styles.btnLink}>
              配置说明
            </a>
          </div>
          <HeroCode />
        </div>
        <div className={styles.heroRight}>
          <HeroIllustration />
        </div>
      </div>
      <div className={styles.heroStats}>
        <div className={styles.heroStat}>
          <span className={styles.heroStatNum}>6</span>
          <span className={styles.heroStatLabel}>种运行时</span>
        </div>
        <div className={styles.heroStatDivider} />
        <div className={styles.heroStat}>
          <span className={styles.heroStatNum}>3</span>
          <span className={styles.heroStatLabel}>级作用域</span>
        </div>
        <div className={styles.heroStatDivider} />
        <div className={styles.heroStat}>
          <span className={styles.heroStatNum}>4</span>
          <span className={styles.heroStatLabel}>级解析优先级</span>
        </div>
        <div className={styles.heroStatDivider} />
        <div className={styles.heroStat}>
          <span className={styles.heroStatNum}>0</span>
          <span className={styles.heroStatLabel}>管理员权限</span>
        </div>
      </div>
    </section>
  );
};

/* 支持的运行时 */
const RuntimeStrip = function () {
  const runtimes: RuntimeItem[] = [
    { name: 'node', scope: '用户 · 项目 · 系统', color: '#539E43' },
    { name: 'python', scope: '用户 · 项目 · 系统', color: '#3776AB' },
    { name: 'pnpm', scope: '用户 · 项目', color: '#F69220' },
    { name: 'yarn', scope: '用户 · 项目', color: '#2C8EBB' },
    { name: 'go', scope: '用户 · 系统', color: '#00ADD8' },
    { name: 'git', scope: '用户 · 系统', color: '#F05033' },
  ];

  return (
    <section className={styles.runtimeStrip}>
      <div className={styles.runtimeInner}>
        <span className={styles.runtimeLabel}>统一管理的运行时</span>
        <div className={styles.runtimeList}>
          {runtimes.map((rt) => (
            <div key={rt.name} className={styles.runtimeTag}>
              <span className={styles.runtimeDot} style={{ background: rt.color }} />
              <span className={styles.runtimeName}>{rt.name}</span>
              <span className={styles.runtimeScope}>{rt.scope}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

/* 核心能力 — 6 卡网格 */
const Features = function () {
  const items: FeatureItem[] = [
    {
      icon: <IconInstall />,
      title: '一键安装',
      desc: '一条 install 装好 node、python、go 等，默认走国内镜像，无需逐个配置源',
      link: `${base}commands/install`,
    },
    {
      icon: <IconSwap />,
      title: '秒级切换',
      desc: 'pvm use 按项目 / 用户 / 系统三级作用域切换，多版本互不污染',
      link: `${base}commands/use`,
    },
    {
      icon: <IconFolder />,
      title: '项目隔离',
      desc: '.pvmrc 声明版本并提交，团队 clone 后 pvm use 即得一致环境',
      link: `${base}config/pvmrc`,
    },
    {
      icon: <IconShield />,
      title: '无需 sudo',
      desc: '运行时全部装在用户目录 ~/.pvm，免管理员权限，CI / 容器内同样可用',
      link: `${base}guide/install`,
    },
    {
      icon: <IconBolt />,
      title: 'Shim 驱动',
      desc: '通过 shim 拦截命令，按需解析版本，shell 启动零开销、切换即时生效',
      link: `${base}guide/shim`,
    },
    {
      icon: <IconMirror />,
      title: '镜像加速',
      desc: '默认国内镜像下载，--official 可切回官方源，--mirror 可强制镜像',
      link: `${base}config/runtimes`,
    },
  ];

  return (
    <section className={styles.features}>
      <div className={styles.sectionHead}>
        <h2 className={styles.sectionTitle}>核心能力</h2>
        <p className={styles.sectionSubtitle}>从安装到隔离，一条命令贯穿研发全流程</p>
      </div>
      <div className={styles.featuresGrid}>
        {items.map((item) => (
          <a key={item.title} href={item.link} className={styles.featureCard}>
            <div className={styles.featureIcon}>{item.icon}</div>
            <div className={styles.featureContent}>
              <h3 className={styles.featureTitle}>{item.title}</h3>
              <p className={styles.featureDesc}>{item.desc}</p>
            </div>
          </a>
        ))}
      </div>
    </section>
  );
};

/* 工作原理 — 4 级解析优先级 */
const Levels: LevelItem[] = [
  { level: '1', source: '环境变量', desc: 'PVM_NODE_VERSION=20.11.0，临时覆盖，优先级最高' },
  { level: '2', source: '.pvmrc / .nvmrc', desc: '从当前目录向上查找，落地项目级版本约定' },
  { level: '3', source: '用户全局', desc: '~/.pvm/versions 中 pvm use 写入的默认版本' },
  { level: '4', source: '系统 PATH', desc: '系统已装同名命令，作为兜底不下载' },
];

const HowItWorks = function () {
  return (
    <section className={styles.how}>
      <div className={styles.sectionHead}>
        <h2 className={styles.sectionTitle}>当你敲下 node，PVM 如何决定版本</h2>
        <p className={styles.sectionSubtitle}>shim 拦截命令后，按 4 级优先级自高向低解析，命中即生效</p>
      </div>
      <div className={styles.howFlow}>
        {Levels.map((lv, idx) => (
          <div key={lv.level} className={styles.howStep}>
            <div className={styles.howBadge}>{lv.level}</div>
            <div className={styles.howBody}>
              <div className={styles.howTopRow}>
                <h3 className={styles.howSource}>{lv.source}</h3>
                {idx === 0 && <span className={styles.howTag}>最高</span>}
                {idx === Levels.length - 1 && <span className={`${styles.howTag} ${styles.howTagGhost}`}>兜底</span>}
              </div>
              <p className={styles.howDesc}>{lv.desc}</p>
            </div>
            {idx < Levels.length - 1 && <div className={styles.howLine} />}
          </div>
        ))}
      </div>
    </section>
  );
};

/* 3 步上手 */
const Steps: StepItem[] = [
  {
    no: '01',
    title: '安装 PVM',
    desc: '一条脚本装好命令行工具，并写入 PATH',
    code: 'curl -fsSL https://pvm.sh/install | sh',
  },
  {
    no: '02',
    title: '安装运行时',
    desc: '按需安装任意版本，默认走国内镜像',
    code: 'pvm install node@20.11.0 python@3.12.0',
  },
  {
    no: '03',
    title: '进入项目即生效',
    desc: '声明版本并提交，团队 clone 后即可一致运行',
    code: 'pvm use node@20.11.0\n# 写入 .pvmrc，提交到仓库',
  },
];

const QuickStart = function () {
  return (
    <section className={styles.steps}>
      <div className={styles.sectionHead}>
        <h2 className={styles.sectionTitle}>3 步上手</h2>
        <p className={styles.sectionSubtitle}>从零到项目级版本锁定，几分钟即可完成</p>
      </div>
      <div className={styles.stepsGrid}>
        {Steps.map((s) => (
          <div key={s.no} className={styles.stepCard}>
            <div className={styles.stepNo}>{s.no}</div>
            <h3 className={styles.stepTitle}>{s.title}</h3>
            <p className={styles.stepDesc}>{s.desc}</p>
            <pre className={styles.stepCode}>
              <code>{s.code}</code>
            </pre>
          </div>
        ))}
      </div>
    </section>
  );
};

/* 子模块导航 */
const SubModules = function () {
  const modules = [
    {
      category: '指南',
      items: [
        { name: 'PVM 简介', icon: '💡', link: `${base}guide/` },
        { name: '安装与初始化', icon: '⬇️', link: `${base}guide/install` },
        { name: '快速开始', icon: '🚀', link: `${base}guide/quick-start` },
        { name: '版本解析机制', icon: '🧭', link: `${base}guide/version-resolution` },
        { name: 'Shim 机制', icon: '🔗', link: `${base}guide/shim` },
      ],
    },
    {
      category: '命令参考',
      items: [
        { name: 'install', icon: '⬇️', link: `${base}commands/install` },
        { name: 'use', icon: '🔀', link: `${base}commands/use` },
        { name: 'list / list-remote', icon: '📋', link: `${base}commands/list` },
        { name: 'remove', icon: '🗑️', link: `${base}commands/remove` },
        { name: 'config', icon: '⚙️', link: `${base}commands/config` },
      ],
    },
    {
      category: '配置',
      items: [
        { name: '.pvmrc 文件', icon: '📄', link: `${base}config/pvmrc` },
        { name: '运行时与镜像源', icon: '🌐', link: `${base}config/runtimes` },
        { name: '环境变量', icon: '🔑', link: `${base}config/env` },
        { name: '常见问题', icon: '❓', link: `${base}faq/` },
      ],
    },
  ];

  return (
    <section className={styles.subModules}>
      <h2 className={styles.subModulesTitle}>快速导航</h2>
      <p className={styles.subModulesSubtitle}>从概念到命令，几分钟上手 PVM</p>
      <div className={styles.subModulesGrid}>
        {modules.map((mod) => (
          <div key={mod.category} className={styles.subModuleCard}>
            <h3 className={styles.subModuleCategory}>{mod.category}</h3>
            <div className={styles.subModuleItems}>
              {mod.items.map((item) => (
                <a key={item.name} href={item.link} className={styles.subModuleItem}>
                  <span className={styles.subModuleItemIcon}>{item.icon}</span>
                  <span className={styles.subModuleItemName}>{item.name}</span>
                  <span className={styles.subModuleItemArrow}>→</span>
                </a>
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
};

/* 底部 CTA */
const BottomCTA = function () {
  return (
    <footer className={styles.homeFooter}>
      <div className={styles.footerCta}>
        <h2 className={styles.footerCtaTitle}>几分钟后，你的团队版本就一致了</h2>
        <p className={styles.footerCtaDesc}>读一遍快速开始，把 .pvmrc 提交进仓库。</p>
        <a href={`${base}guide/quick-start`} className={styles.btnPrimary}>
          快速开始 <IconArrow />
        </a>
      </div>
      <div className={styles.footerContent}>
        <div className={styles.footerSection}>
          <h4 className={styles.footerTitle}>指南</h4>
          <p className={styles.footerDesc}>概念、安装、快速开始</p>
          <a href={`${base}guide/quick-start`} className={styles.footerLink}>
            查看详情 →
          </a>
        </div>
        <div className={styles.footerDivider} />
        <div className={styles.footerSection}>
          <h4 className={styles.footerTitle}>命令参考</h4>
          <p className={styles.footerDesc}>全部子命令与全局参数</p>
          <a href={`${base}commands/`} className={styles.footerLink}>
            查看详情 →
          </a>
        </div>
        <div className={styles.footerDivider} />
        <div className={styles.footerSection}>
          <h4 className={styles.footerTitle}>配置</h4>
          <p className={styles.footerDesc}>.pvmrc、镜像源、环境变量</p>
          <a href={`${base}config/`} className={styles.footerLink}>
            查看详情 →
          </a>
        </div>
      </div>
      <div className={styles.footerCopyright}>
        <p>© {new Date().getFullYear()} PVM · Polyglot Version Manager · 基于 shim 驱动</p>
      </div>
    </footer>
  );
};

/* 主布局 */
const HomeLayout = function () {
  return (
    <div className={styles.homeContainer}>
      <Hero />
      <RuntimeStrip />
      <Features />
      <HowItWorks />
      <QuickStart />
      <SubModules />
      <BottomCTA />
    </div>
  );
};

export default HomeLayout;
