# PVM 命令参考

## 安装与管理

| 命令 | 说明 | 示例 |
|------|------|------|
| `pvm install <runtime>@<version>` | 安装运行时 | `pvm install node@20` |
| `pvm use <runtime>@<version>` | 切换版本（用户级） | `pvm use node@20` |
| `pvm use <runtime>@<version> --local` | 切换版本（项目级，写入 .pvmrc） | `pvm use python@3.12 --local` |
| `pvm use` | 按 .pvmrc 自动切换 | `cd project && pvm use` |
| `pvm list` | 列出已安装的所有运行时 | `pvm list` |
| `pvm list <runtime>` | 列出某运行时的可用版本 | `pvm list node` |
| `pvm current` | 查看当前激活的版本 | `pvm current` |
| `pvm remove <runtime>@<version>` | 卸载某运行时版本 | `pvm remove node@18` |

## 环境维护

| 命令 | 说明 |
|------|------|
| `pvm setup` | 初始化/修复环境（PATH 冲突修复） |
| `pvm doctor` | 诊断环境配置问题 |
| `pvm reshim` | 重新生成所有 shim 硬链接 |
| `pvm self-update` | 更新 PVM 自身到最新版 |
| `pvm uninstall` | 彻底卸载 PVM |

## 项目配置

| 命令 | 说明 |
|------|------|
| `pvm init` | 在当前目录初始化 .pvmrc |
| `pvm config` | 查看/管理 .pvmrc |
| `pvm validate` | 验证 .pvmrc 配置是否正确 |

## .pvmrc 格式

```ini
# .pvmrc
node = 20.11.0
python = 3.12.0
pnpm = 9
bun = 1.1.0
```

## 支持的运行时

| 运行时 | 安装 | 用户级切换 | 项目级 (.pvmrc) | 国内镜像 |
|--------|:---:|:---:|:---:|:---:|
| node | ✅ | ✅ | ✅ | npmmirror |
| python | ✅ | ✅ | ✅ | npmmirror |
| bun | ✅ | ✅ | ✅ | npmmirror |
| deno | ✅ | ✅ | ✅ | npmmirror |
| pnpm | ✅ | ✅ | ✅ | npmmirror |
| yarn | ✅ | ✅ | ✅ | npmmirror |
| go | ✅ | ✅ | — | golang.google.cn |
| git | ✅ | ✅ | — | npmmirror |
| rust | ✅ | ✅ | — | rsproxy.cn |

## 版本指定方式

- 精确版本：`pvm install node@20.11.0`
- 主版本：`pvm install node@20`（自动选最新 20.x.x）
- 最新版：`pvm install node@latest`
