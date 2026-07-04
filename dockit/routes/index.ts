import type { RouterExport, NavItem } from './types';

/**
 * PVM 文档站点路由定义
 * - 首页（/）挂载自定义 HomeLayout（theme/home），由 docs/index.mdx 的 home:true 触发
 * - 内容模块路由从 docs/ 目录结构自动派生
 * - activeMatch 保证在子页面时一级导航高亮正确
 */
export const routerConfig: RouterExport = {
  navItems: [
    { text: '首页', link: '/', activeMatch: '^/$' },
    { text: '指南', link: '/guide/', activeMatch: '^/guide/' },
    { text: '命令参考', link: '/commands/', activeMatch: '^/commands/' },
    { text: '配置', link: '/config/', activeMatch: '^/config/' },
    { text: '常见问题', link: '/faq/', activeMatch: '^/faq/' },
  ] as NavItem[],
  sidebar: {
    '/guide': [
      {
        text: '入门',
        items: [
          { text: 'PVM 简介', link: '/guide/' },
          { text: '安装与初始化', link: '/guide/install' },
          { text: '快速开始', link: '/guide/quick-start' },
        ],
      },
      {
        text: '核心概念',
        items: [
          { text: '版本解析机制', link: '/guide/version-resolution' },
          { text: 'Shim 机制', link: '/guide/shim' },
        ],
      },
    ],
    '/commands': [
      {
        text: '版本管理',
        items: [
          { text: 'install', link: '/commands/install' },
          { text: 'use', link: '/commands/use' },
          { text: 'list / list-remote', link: '/commands/list' },
          { text: 'remove', link: '/commands/remove' },
          { text: 'current / which / where', link: '/commands/current' },
        ],
      },
      {
        text: '项目与系统',
        items: [
          { text: 'config', link: '/commands/config' },
          { text: 'setup / doctor', link: '/commands/setup' },
          { text: 'validate / diagnostics', link: '/commands/doctor-more' },
          { text: 'self-update / uninstall', link: '/commands/self' },
        ],
      },
    ],
    '/config': [
      {
        text: '项目配置',
        items: [
          { text: '配置总览', link: '/config/' },
          { text: '.pvmrc 文件', link: '/config/pvmrc' },
          { text: '运行时与镜像源', link: '/config/runtimes' },
          { text: '环境变量', link: '/config/env' },
        ],
      },
    ],
    '/faq': [
      {
        text: '排障',
        items: [
          { text: '常见问题', link: '/faq/' },
          { text: '故障排查', link: '/faq/troubleshooting' },
        ],
      },
    ],
  },
};

export default routerConfig;
