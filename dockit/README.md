# DocKit

基于 Rspress 2.x 的模块化文档与工具导航平台。

## 特性

- ✅ 三大模块导航（开发工具 / 模型工具 / 物料资源）
- ✅ 统一的设计语言
- ✅ TypeScript 支持
- ✅ 响应式设计
- ✅ 深色模式支持
- ✅ Rspress 2.x 稳定版支持

## 快速开始

```bash
# 安装依赖
pnpm install

# 启动开发
pnpm run dev

# 构建
pnpm run build

# 预览
pnpm run preview
```

## 项目结构

```
dockit/
├── docs/                # 文档内容
│   ├── index.mdx         # 首页
│   ├── dev-tools/        # 开发工具模块
│   │   ├── index.mdx
│   │   ├── chrome-plugin/
│   │   ├── build-tools/
│   │   └── app-software/
│   ├── model-tools/      # 模型工具模块
│   │   ├── index.mdx
│   │   ├── mcp/
│   │   ├── skills/
│   │   └── cli/
│   └── materials/        # 物料资源模块
│       ├── index.mdx
│       ├── components/
│       └── scripts/
├── routes/              # 路由配置
│   ├── index.ts          # 主配置
│   └── types.ts          # 类型定义
├── theme/              # 主题定制
│   ├── assets/          # 静态资源
│   ├── home/
│   │   ├── index.tsx     # 首页布局
│   │   └── index.scss    # 首页样式
│   ├── index.tsx         # 主题入口
│   └── index.css         # 主题样式
├── views/              # 视图入口
│   └── index.ts
├── rspress.config.ts     # Rspress 配置
├── tsconfig.json        # TypeScript 配置
└── package.json         # 项目依赖
```

## 路由自动编译

当修改以下文件时，rspress 会自动重新编译：

- `routes/**/*.ts` - 路由配置
- `views/**/*.tsx` - 组件文件
- `docs/**/*.tsx` - 文档中的组件
- `docs/**/*.scss` - 样式文件
- `theme/**/*.tsx` - 主题组件
- `theme/**/*.scss` - 主题样式

**无需手动重启开发服务器！**

## 自定义组件

您可以参考现有组件创建新的：

1. 在 `theme/` 中创建组件文件
2. 在 `theme/index.tsx` 中导出
3. 在文档页面中导入使用

## 开发

```bash
pnpm run dev
```

访问 http://localhost:3000

## 构建

```bash
pnpm run build
```

构建产物在 `doc_build/` 目录（Rspress 2.x 默认输出）。
