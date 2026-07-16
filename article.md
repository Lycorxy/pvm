# 让AI不操心环境版本了，项目隔离不止是doctor

写代码这事儿，我现在基本离不开 AI 助手了。Cursor、Claude Code 轮着用。

但有个事儿越用越烦：代码 AI 唰唰给你生成，结果一跑，挂了。原因十有八九不是代码，是环境。

我琢磨了挺久，把自己踩过的坑理了理，顺手写了个叫 PVM 的小工具。不算什么大东西，但确实把我最烦的几个场景解决了。写出来，你要是也被折腾过，可能对你有用。

## 第一个坑：AI 自己瞎修环境，越修越乱

最常见的情况是这样。AI 在终端里跑你的项目，跑不起来。它不会先问环境对不对，而是盯着报错就上手修：装个 Node 18、升一下 npm、再换个版本试试。折腾半天，终于跑通了。

但你回头一看，代价不小。

时间全花在环境上了，代码没改几行。更坑的是，它很可能顺手改了全局的 Node，或者往 PATH 里塞了点东西。当时没感觉，第二天另一个老项目炸了，你还得查半天。

根子上的问题是：AI 在修一个全局共享的环境。它一动，别处就可能被带歪。

PVM 处理这事儿分两层。

一层是 shim。装完之后，`~/.pvm/shims/` 里那些 `node.exe`、`python.exe` 其实不是真身，是同一个壳。你敲 `node -v`，它先读当前目录的 `.pvmrc`，知道你要的是 20.11.0，再把命令转给 `~/.pvm/installs/node/20.11.0/node.exe`。所以进对目录，版本就对了，AI 压根不用去修环境。

另一层是隔离。AI 在当前项目折腾出来的版本，只活在 `~/.pvm/installs/` 这份目录里，碰不到全局，也碰不到别的项目。而且 `pvm use` 装版本是幂等的，已经下过了就跳过，不会真去反复装卸。就算它装了卸、卸了装，也不会给你明天埋雷。

## 第二个坑：项目一切换，版本就乱套

上午在 A 项目（Node 18），下午切到 B 项目（Node 22），脑子一抽忘了切。`npm install` 跑完推上去，CI 红了。

这种一不留神，真没法靠记性。项目一多，版本就是记不过来。

像 Volta 这类工具，主要在 Node 圈里转。pnpm、yarn 它也能单独装，但不跟 Node 版本联动查兼容性，Python、Rust、Go 更是不管。你要是多语言项目，还得再请几个管理器。

PVM 的做法是把版本写进项目自己的 `.pvmrc`，提交到 Git：

```ini
# .pvmrc
node 20.11.0
python 3.12.0
```

进目录生效，出目录还原。你切到哪个项目，哪个项目的版本就顶上，不用记，也不用手动切。pnpm、yarn 也一起锁进 `.pvmrc`，这俩跟 Node 版本是强绑定的，老 pnpm 跑在新 Node 上会直接崩，PVM 装的时候顺手就帮你查了兼容性。

说到这，有个点我想掰扯一下：项目隔离，真不止是跑个 doctor。

很多人第一反应是装个 doctor 查环境。但 doctor 干的是诊断，它顶多告诉你有问题、你去修，活还是你的。PVM 是靠隔离让问题不发生，版本写进项目，谁进这个目录谁就用对的版本，AI、你、CI，一视同仁。所以你不需要跑 doctor，`.pvmrc` 一锁，坏的那一刻就不存在了。

锁版本就一行命令：

```bash
pvm use node@20 python@3.12
```

它在项目目录里就会把版本写进 `.pvmrc`，没装就当场下载（已装跳过），装完自动重建 shim。提交之后，新人 clone 完 `cd` 进去就能跑，AI 进去也是对的版本。没有安装文档，也没有"你用的到底是哪个版本来着"这种对话。

## 第三个坑：skill 要调一堆语言，环境碎成八瓣

现在 AI 的 skill（Claude Code 的 skill、Cursor 的 rule）经常要调各种运行时跑脚本：python 处理数据、node 跑前端检查、bun 起服务、rust 跑个 CLI。

每个运行时单独装、单独配，作者得疯。而且今天要这几种，明天项目里冒出个新语言，又得折腾一遍。

PVM 管的是运行时这个抽象，不是某一个语言。底层是一套插件框架，每种运行时（node、python、rust、go、bun、deno、pnpm、yarn、git）都是一个插件，各自管自己的下载、安装、验证，以及要拦截哪些命令。想加第 10 种语言，照着 node 写个插件注册一下就行，核心代码一行不用动。这跟 Volta 把 Node 生态焊死在核心里不一样，PVM 天生就是为再加一种设计的。

所以一套语法通吃：

```bash
pvm use node@20 bun@1.1 python@3.12 go@1.22 rust@stable git@latest
```

一个 skill 依赖什么运行时，全写进它的 `.pvmrc`，谁跑这个 skill 谁就自动拿到全套正确环境。

## 如果你用 Windows

这点别的工具基本是空白，我说两句。

系统 PATH 压你一头：Windows 系统级 PATH 比用户级优先级高，你装的 Node 常被 `C:\Program Files\nodejs` 盖掉。PVM 检测到就弹个 UAC，把冲突条目挪到末尾，点一次 Yes 完事。

GitHub 一拉就 403：国内访问 GitHub API 查版本常 403，PVM 遇到就自动切国内镜像重试（npmmirror 管 Bun/Deno，rsproxy.cn 管 Rust），从查版本到下二进制全链路加速，你啥都不用配。

IDE 找不到 Git/Python：装完自动把 exe 复制到 `~/.pvm/bin/`，VS Code 直接认。

## 上手就三步

```bash
# Windows：下 MSI 双击，PATH 自动配好
# macOS / Linux：
curl -fsSL https://raw.githubusercontent.com/lycorxy/pvm/main/scripts/install.sh | bash

cd my-project
pvm use node@20 python@3.12
git add .pvmrc && git commit -m "lock runtimes"
```

更完整的安装和命令说明见官网 [lycorxy.github.io/pvm](https://lycorxy.github.io/pvm)。

AI 助手、新人 clone 之后，`cd` 进来就能跑。没有第四步。

## 几个实在的问题

已经在用 nvm，会冲突吗？不会。PVM 的 shim 在独立目录，优先级可控，两个能共存，慢慢迁。

我的 `.nvmrc` 还能用吗？能。PVM 解析版本时会读 `.nvmrc`、`.python-version`，你已有的文件原样留着。不过 `pvm use` 写的是自己的 `.pvmrc`，老项目能直接用，新项目建议统一交给 `.pvmrc`。

Mac / Linux 能用？能，shim 三系统通用。我主力是 Windows，所以 Windows 这块做得最细。

想管官方没列的语言？能。每种语言就是个 `RuntimePlugin` 接口的插件，照着 node 写一份注册一下就进 `.pvmrc` 了。这点是它跟把 Node 生态写死的工具最大的区别。

## 最后

AI 现在能写一堆代码，但它写之前，得有人把环境这摊事平了，不然生成的代码跑不起来。

PVM 把这事收成了一条 `pvm use` 和一个 `.pvmrc`：版本跟着项目走，进目录就自动对。

仓库 [github.com/Lycorxy/pvm](https://github.com/Lycorxy/pvm)，官网 [lycorxy.github.io/pvm](https://lycorxy.github.io/pvm)，Release 在 [v0.0.0](https://github.com/Lycorxy/pvm/releases/tag/v0.0.0)，MIT。我自己也还在慢慢打磨，欢迎来提 issue 或者拍砖。

---

你被 AI 因为环境版本不对坑得最惨的一次是什么？评论区聊聊。
