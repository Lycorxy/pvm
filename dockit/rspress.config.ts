import path from 'node:path';

import { pluginSass } from '@rsbuild/plugin-sass';

import { routerConfig } from './routes';

/**
 * 公共路径配置
 * 开发环境 (rspress dev):  PVM_DEPLOY 未定义 → /
 * 生产环境 (rspress build): PVM_DEPLOY=1    → /pvm/
 */
const publicPath = process.env.PVM_DEPLOY ? '/pvm/' : '/';

/**
 * 需要监听的配置文件列表
 * 当这些文件变化时，rspress 会自动重新编译
 */
const watchFiles = [
  path.join(__dirname, 'routes/**/*.ts'), // 监听所有路由配置文件
  path.join(__dirname, 'views/**/*.tsx'), // 监听 views 中的 tsx 文件
  path.join(__dirname, 'docs/**/*.tsx'), // 监听 docs 中的 tsx 文件
  path.join(__dirname, 'docs/**/*.scss'), // 监听 docs 中的 scss 文件
  path.join(__dirname, 'theme/**/*.tsx'), // 监听 theme 中的 tsx 文件
  path.join(__dirname, 'theme/**/*.scss'), // 监听 theme 中的 scss 文件
];

const config = {
  // 根目录
  root: path.join(__dirname, 'docs'),

  // 输出目录：直接输出到项目根目录的 docs 目录（用于 GitHub Pages）
  outDir: path.join(__dirname, '../docs'),

  // 自定义主题目录（Rspress v2 部分 beta 不识别 themeDir，首页改用 mdx 直接挂载 HomeLayout）
  themeDir: path.join(__dirname, 'theme'),

  // 站点标题
  title: 'PVM',

  // 站点图标
  icon: '/logo.svg',
  // Logo 配置
  logo: {
    light: '/logo.svg',
    dark: '/logo.svg',
  },

  // 公共路径
  base: publicPath,

  // 路由配置
  route: {
    // 包含的文件模式：仅真实文档页（.md/.mdx），避免 .d.ts 等被当作页面路由
    include: ['docs/**/*.md', 'docs/**/*.mdx'],
    // 排除的文件模式
    exclude: ['**/node_modules/**', '**/global.d.ts'],
    // 清洁 URL（去掉 .html 后缀）
    cleanUrls: true,
  },

  // 主题配置
  themeConfig: {
    // 页脚配置
    footer: {
      message: `© ${new Date().getFullYear()} PVM — Polyglot Version Manager. All rights reserved.`,
    },

    // 社交链接
    socialLinks: [
      {
        icon: 'github',
        mode: 'link',
        content: 'https://github.com/pvm/pvm',
      },
    ],

    // 导航菜单（从 TS 配置导入）
    nav: routerConfig.navItems,

    // 侧边栏（从 TS 配置导入）
    sidebar: routerConfig.sidebar,

    // 分页导航文本
    prevPageText: '上一页',
    nextPageText: '下一页',

    // 目录标题
    outlineTitle: '目录',

    // 显示深色模式切换
    enableDarkMode: true,
  },

  // 构建器配置
  builderConfig: {
    plugins: [pluginSass()],

    output: {
      // 资源前缀
      assetPrefix: publicPath,
      // CSS 模块配置 - 自动处理 .scss 文件
      cssModules: {
        auto: /\.scss$/,
      },
      // CSS 模块类名生成规则
      cssModuleLocalIdentName: 'k[local]_[hash:6]',
      // 禁用 svgr
      disableSvgr: false,
    },

    // 自定义 Rsbuild 配置
    modifyRsbuildConfig: (rsbuildConfig: { module?: { rules?: unknown[] } }) => {
      const originalRules = rsbuildConfig.module?.rules ?? [];

      // 为 docs 目录的 SCSS 文件添加特殊规则
      const docsRule = {
        test: /docs.*\.scss$/,
        use: [
          {
            loader: 'css-loader',
            options: {
              modules: {
                getLocalIdent: (ctx: { resourcePath: string }) => {
                  const fileName = ctx.resourcePath.split(/[\\/]/).pop()?.replace('.scss', '');
                  return `dk-${fileName}-[local]_[hash:6]`;
                },
              },
            },
          },
        ],
      };

      // 将新规则添加到配置的开头
      return {
        ...rsbuildConfig,
        module: {
          ...rsbuildConfig.module,
          rules: [docsRule, ...originalRules],
        },
      };
    },

    resolve: {
      // 路径别名
      alias: {
        '@views': path.join(__dirname, './views'),
      },
    },
  },
  // Markdown 配置
  markdown: {
    // 显示行号
    showLineNumbers: true,
    // 代码块默认换行
    defaultWrapCode: false,
    // 使用 Rust MDX 编译器
    mdxRs: false,
  },
  // 中等缩放
  mediumZoom: true,
  // 监听文件变化，实现自动重新编译
  watchFiles,
  // 开发服务器配置
  devServer: {
    // 端口
    port: 3000,
    // 主机
    host: '0.0.0.0',
    // 自动打开浏览器
    open: false,
  },
};

export default config;
