# 触发示例（pvm）

## ✅ 必须触发（正向）

1. "帮我检查一下 pvm 环境有没有问题，node 好像还是旧的"
   → Step 1 `node pvm-harness.mjs check`，重点看 shims PATH 与 nvm 干扰项。
2. "机器上原来有 nvm，现在要统一用 pvm，确保 pvm 不受影响"
   → check → 若 nvm 劫持则 `fix path` → 复跑 check → 提示新开终端。
3. "读一下我这个项目的 pvm.yaml 和 .pvmrc，然后加一条流水线把 go 打包成全平台"
   → `config read` → 按 pvm-guide.md 编辑 pvm.yaml（pkg:go: 用 `pvm pack go --entry . --output dist/app`）→ `config validate`。

## ❌ 必须不触发（负向）

1. "帮我给 pvm 加个新命令，改 cmd/root.go"
   → pvm 源码开发任务，不触发本 skill。
2. "clone 下来这个项目，装依赖跑起来"
   → 环境准备（拉代码/建分支/装依赖），走 code-env；仅当其中明确要 pvm 管理运行时且出环境问题时才触发。
3. "nvm 怎么设置 .nvmrc 默认版本？"
   → nvm 自身用法咨询，不触发；若用户有意迁移到 pvm（"以后用 pvm"）则触发并按迁移处理。
