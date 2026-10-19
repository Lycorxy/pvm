# 负向触发示例

以下提示词必须**不**触发 code-env skill（走对应单项能力 skill）：

1. "帮我安装 pvm" — 单独安装 pvm → **pvm skill**（code-env 的 env-check 只是流程中的一环）
2. "pvm use node@20.11.0 切个版本" — 运行时切换 → **pvm skill**
3. "创建 MR" / "打个 tag" — 工蜂 gitflow 操作 → **flowcli skill**
4. "发布测试环境，启动流水线" — 流水线操作 → **flowcli skill**
5. "帮我写个用户列表组件" — 前端开发 → **frontend-codegen**（环境已就绪后的开发任务）
6. "node_modules 装不上，EPERM 报错" — 单点排障 → **pvm skill**（fix-pnpm-eperm 脚本）
7. "这个项目用什么技术栈？" — 咨询类问题，无环境操作

# 要点

负向的共同特征：**单项能力操作**（只装/只切/只 MR/只排障）或**非环境任务**。
code-env 只承接"多步骤环境准备流程"；单项命令走对应能力 skill（pvm/flowcli）。
