#Requires -Version 5.1
<#
.SYNOPSIS
    真实验证 pvm 的流水线 / 二进制打包 / zip 全部功能，逐项输出 PASS/FAIL。

.DESCRIPTION
    会构建当前仓库的 pvm 到 dist\pvm.exe，并把它放到 PATH 最前面
    （案例里的 `pvm pkg ...` 因此用的是刚构建的版本）。
    产物与临时文件都写在 example 各案例的 dist\ 与 example\.work\ 下，可重复运行。

.EXAMPLE
    .\verify.ps1                 # 全量（go + node + python）
    .\verify.ps1 -SkipNode       # 跳过 Node SEA（首次要下载 esbuild/postject）
    .\verify.ps1 -SkipNode -SkipPython
#>
[CmdletBinding()]
param(
    [switch]$SkipNode,
    [switch]$SkipPython
)

$ErrorActionPreference = 'Continue'
$example = $PSScriptRoot
$repo = Split-Path -Parent $example
$work = Join-Path $example '.work'
$results = New-Object System.Collections.ArrayList

function Section([string]$title) {
    Write-Host ''
    Write-Host "== $title" -ForegroundColor Cyan
}

function Check([string]$name, [bool]$ok, [string]$note = '') {
    [void]$results.Add([pscustomobject]@{ Case = $name; Ok = $ok; Note = $note })
    if ($ok) {
        Write-Host "  PASS  $name" -ForegroundColor Green
    }
    else {
        Write-Host "  FAIL  $name" -ForegroundColor Red
        if ($note) { Write-Host "        $note" -ForegroundColor DarkGray }
    }
}

# 调用 pvm，返回 @{ Code; Out }（stderr 丢弃，避免 PowerShell 把原生命令的 stderr 包装成错误文本）
# 注意：函数名不能叫 Pvm —— PowerShell 不区分大小写，会和外部命令 pvm 撞名并递归调用自己。
function Invoke-Pvm([string[]]$Arguments) {
    $out = & pvm @Arguments 2>$null | Out-String
    [pscustomobject]@{ Code = $LASTEXITCODE; Out = $out }
}

function Enter([string]$dir) { Push-Location $dir }
function Leave() { Pop-Location }

# ---------------------------------------------------------------- 准备
Section 'build pvm from source'
$pvmExe = Join-Path $repo 'dist\pvm.exe'
Enter $repo
& go build -o $pvmExe .
$buildOk = ($LASTEXITCODE -eq 0) -and (Test-Path $pvmExe)
Leave
Check 'go build -o dist/pvm.exe .' $buildOk
if (-not $buildOk) {
    Write-Host 'cannot continue without a built pvm' -ForegroundColor Red
    exit 1
}

# 让案例流水线里的 `pvm pkg` 走刚构建的版本
$env:PATH = (Split-Path -Parent $pvmExe) + ';' + $env:PATH
$resolved = (Get-Command pvm -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1).Source
Check 'pvm resolves to freshly built binary' ($resolved -eq $pvmExe) "exe=[$pvmExe] path0=[$($env:PATH.Split(';')[0])] resolved=[$resolved]"

$v = Invoke-Pvm @('--version')
Check 'pvm --version' ($v.Code -eq 0 -and $v.Out -match '^pvm ') $v.Out.Trim()

Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $work | Out-Null

# ---------------------------------------------------------------- 流水线语义
Section 'pipeline: list / dry-run'
Enter (Join-Path $example 'pipeline-demo')

$r = Invoke-Pvm @('run')
Check 'pvm run (no name) lists pipelines' (
    $r.Code -eq 0 -and $r.Out -match 'pipeline-demo' -and $r.Out -match 'release:pro' -and $r.Out -match 'check'
) $r.Out.Trim()

$r = Invoke-Pvm @('run', 'release:pro', '--dry-run')
Check 'pvm run --dry-run renders a tree' (
    $r.Code -eq 0 -and $r.Out -match '├─' -and $r.Out -match 'parallel' -and $r.Out -match 'packaged'
) $r.Out.Trim()

Section 'pipeline: sequential + parallel + reference'
$r = Invoke-Pvm @('run', 'dev')
Check 'pvm run dev exits 0' ($r.Code -eq 0) $r.Out.Trim()
Check 'sequential steps ran in order' (
    $r.Out -match 'hello from pvm.yaml' -and $r.Out -match 'dev ready'
) 
Check 'parallel branches ran with [name] prefixes' (
    $r.Out -match '\[lint\]' -and $r.Out -match '\[test\]' -and $r.Out -match 'lint ok' -and $r.Out -match 'test ok'
)
Check 'summary line printed' ($r.Out -match '✔ dev')

Section 'pipeline: fail fast'
$r = Invoke-Pvm @('run', 'fail')
Check 'pvm run fail exits non-zero' ($r.Code -ne 0) "exit=$($r.Code)"
Check 'step after failure did not run' ($r.Out -notmatch 'step 3 should not run') $r.Out.Trim()
Check 'first step did run' ($r.Out -match 'step 1 ok')

Section 'pipeline: errors'
$r = Invoke-Pvm @('run', 'nope')
Check 'unknown pipeline exits non-zero' ($r.Code -ne 0)

$cycleDir = Join-Path $work 'cycle'
New-Item -ItemType Directory -Force $cycleDir | Out-Null
[IO.File]::WriteAllText((Join-Path $cycleDir 'pvm.yaml'), "a: [b]`nb: [a]`n", (New-Object Text.UTF8Encoding($false)))
Enter $cycleDir
$r = Invoke-Pvm @('run', 'a')
Leave
Check 'circular reference is rejected' ($r.Code -ne 0)

Section 'pipeline: -C from another directory'
Enter $repo
$r = Invoke-Pvm @('run', '-C', (Join-Path $example 'go-hello'), '--dry-run')
Leave
Check 'pvm run -C <dir> finds that project' ($r.Code -eq 0 -and $r.Out -match 'release:pro') $r.Out.Trim()
Leave

# ---------------------------------------------------------------- go 案例
Section 'go-hello: release:dev (parallel checks + build + run)'
$goDir = Join-Path $example 'go-hello'
Remove-Item -Recurse -Force (Join-Path $goDir 'dist') -ErrorAction SilentlyContinue
Enter $goDir
$r = Invoke-Pvm @('run', 'release:dev')
Check 'release:dev exits 0' ($r.Code -eq 0) $r.Out.Trim()
Check 'parallel check steps reported' ($r.Out -match '\[fmt \]' -and $r.Out -match '\[vet \]' -and $r.Out -match '\[test\]')
Check 'build produced dist\go-hello.exe' (Test-Path (Join-Path $goDir 'dist\go-hello.exe'))
Check 'program output' ($r.Out -match 'go-hello: pvm \(')

Section 'go-hello: release:pro (5 targets + zip)'
Write-Host '  (building 5 platforms, ~30-60s)'
$r = Invoke-Pvm @('run', 'release:pro')
Check 'release:pro exits 0' ($r.Code -eq 0) $r.Out.Trim()

$targets = @(
    'go-hello-windows-amd64.exe',
    'go-hello-linux-amd64',
    'go-hello-linux-arm64',
    'go-hello-darwin-amd64',
    'go-hello-darwin-arm64'
)
$missing = @()
foreach ($t in $targets) {
    $exe = Join-Path $goDir "dist\$t"
    $zip = Join-Path $goDir ('dist\' + [IO.Path]::GetFileNameWithoutExtension($t) + '.zip')
    if ($t -notlike '*.exe') { $zip = Join-Path $goDir "dist\$t.zip" }
    if (-not (Test-Path $exe)) { $missing += $t }
    if (-not (Test-Path $zip)) { $missing += "$t.zip" }
}
Check '5 artifacts + 5 zips exist' ($missing.Count -eq 0) ("missing: " + ($missing -join ', '))

$run = & (Join-Path $goDir 'dist\go-hello-windows-amd64.exe') 'pvm' 2>$null | Out-String
Check 'windows artifact runs' ($LASTEXITCODE -eq 0 -and $run -match 'go-hello: pvm \(windows/amd64\)') $run.Trim()

$r = Invoke-Pvm @('unzip', 'dist\go-hello-linux-amd64.zip', '-l')
Check 'artifact zip contains the binary' ($r.Code -eq 0 -and $r.Out -match 'go-hello-linux-amd64')
Leave

# ---------------------------------------------------------------- node 案例
if (-not $SkipNode) {
    Section 'node-hello: release:dev (npm + Node SEA)'
    Write-Host '  (SEA packaging copies the node binary, ~60-90s)'
    $nodeDir = Join-Path $example 'node-hello'
    Remove-Item -Recurse -Force (Join-Path $nodeDir 'dist') -ErrorAction SilentlyContinue
    Enter $nodeDir
    $r = Invoke-Pvm @('run', 'release:dev')
    Check 'release:dev exits 0' ($r.Code -eq 0) $r.Out.Trim()
    Check 'build produced dist\node-hello.exe' (Test-Path (Join-Path $nodeDir 'dist\node-hello.exe'))
    if (Test-Path (Join-Path $nodeDir 'dist\node-hello.exe')) {
        $run = & (Join-Path $nodeDir 'dist\node-hello.exe') 'pvm' 2>$null | Out-String
        Check 'SEA artifact runs' ($LASTEXITCODE -eq 0 -and $run -match 'node-hello: pvm \(') $run.Trim()

        # 复制到没有 dist/config.json 的目录：还能读到配置 => 配置真的内嵌在产物里
        $iso = Join-Path $work 'iso-node'
        New-Item -ItemType Directory -Force $iso | Out-Null
        Copy-Item (Join-Path $nodeDir 'dist\node-hello.exe') $iso -Force
        $run = & (Join-Path $iso 'node-hello.exe') 'embedded' 2>$null | Out-String
        Check 'config.json is embedded (--include)' ($LASTEXITCODE -eq 0 -and $run -match 'env=dev' -and $run -match 'port=3000') $run.Trim()
    }
    Leave
}
else {
    Section 'node-hello: skipped (-SkipNode)'
}

# ---------------------------------------------------------------- python 案例
if (-not $SkipPython) {
    Section 'python-hello: release:dev (PyInstaller --onefile)'
    Write-Host '  (PyInstaller packaging, ~30-60s)'
    $pyDir = Join-Path $example 'python-hello'
    Remove-Item -Recurse -Force (Join-Path $pyDir 'dist') -ErrorAction SilentlyContinue
    Enter $pyDir
    $r = Invoke-Pvm @('run', 'release:dev')
    Check 'release:dev exits 0' ($r.Code -eq 0) $r.Out.Trim()
    Check 'build produced dist\python-hello.exe' (Test-Path (Join-Path $pyDir 'dist\python-hello.exe'))
    if (Test-Path (Join-Path $pyDir 'dist\python-hello.exe')) {
        $run = & (Join-Path $pyDir 'dist\python-hello.exe') 'pvm' 2>$null | Out-String
        Check 'PyInstaller artifact runs' ($LASTEXITCODE -eq 0 -and $run -match 'python-hello: pvm \(') $run.Trim()

        $iso = Join-Path $work 'iso-py'
        New-Item -ItemType Directory -Force $iso | Out-Null
        Copy-Item (Join-Path $pyDir 'dist\python-hello.exe') $iso -Force
        $run = & (Join-Path $iso 'python-hello.exe') 'embedded' 2>$null | Out-String
        Check 'config.json is embedded (--include)' ($LASTEXITCODE -eq 0 -and $run -match 'env=dev' -and $run -match 'port=3000') $run.Trim()
    }
    Leave
}
else {
    Section 'python-hello: skipped (-SkipPython)'
}

# ---------------------------------------------------------------- zip / unzip
Section 'zip / unzip'
Enter $example

$a = Join-Path $work 'a.zip'
$b = Join-Path $work 'b.zip'
$r = Invoke-Pvm @('zip', 'go-hello', '-o', $a, '-x', 'dist', '--reproducible')
Check 'pvm zip with --exclude' ($r.Code -eq 0 -and $r.Out -match 'files') $r.Out.Trim()

$r2 = Invoke-Pvm @('zip', 'go-hello', '-o', $b, '-x', 'dist', '--reproducible')
$h1 = (Get-FileHash $a).Hash
$h2 = (Get-FileHash $b).Hash
Check '--reproducible gives identical bytes' ($r2.Code -eq 0 -and $h1 -eq $h2) "$h1 vs $h2"

$r = Invoke-Pvm @('unzip', $a, '-l')
Check 'pvm unzip --list' ($r.Code -eq 0 -and $r.Out -match 'main.go' -and $r.Out -notmatch 'dist/') $r.Out.Trim()

$outDir = Join-Path $work 'out'
$r = Invoke-Pvm @('unzip', $a, '-o', $outDir)
$same = (Test-Path (Join-Path $outDir 'go-hello\main.go')) -and
    ((Get-FileHash (Join-Path $outDir 'go-hello\main.go')).Hash -eq (Get-FileHash (Join-Path $goDir 'main.go')).Hash)
Check 'pvm unzip writes files (content matches)' ($r.Code -eq 0 -and $same) $r.Out.Trim()

$r = Invoke-Pvm @('zip', 'go-hello/', '-o', (Join-Path $work 'contents.zip'), '-x', 'dist', '--reproducible')
$r = Invoke-Pvm @('unzip', (Join-Path $work 'contents.zip'), '-l')
Check 'trailing slash zips directory contents' ($r.Code -eq 0 -and $r.Out -match '\s+main\.go' -and $r.Out -notmatch 'go-hello/')

Add-Type -AssemblyName System.IO.Compression, System.IO.Compression.FileSystem
$evil = Join-Path $work 'evil.zip'
$fs = [IO.File]::Create($evil)
$za = New-Object IO.Compression.ZipArchive($fs, [IO.Compression.ZipArchiveMode]::Create)
$entry = $za.CreateEntry('../../evil.txt')
$stream = $entry.Open()
$writer = New-Object IO.StreamWriter($stream)
$writer.Write('x')
$writer.Dispose()
$za.Dispose()
$fs.Dispose()
$r = Invoke-Pvm @('unzip', $evil, '-o', (Join-Path $work 'evil'))
Check 'zip-slip is rejected' ($r.Code -ne 0)
Leave

# ---------------------------------------------------------------- pkg 参数校验
Section 'pkg argument validation'
$r = Invoke-Pvm @('pkg', 'python', '--entry', 'python-hello/app.py', '--type', 'linux')
Check 'python cross-compile rejected' ($r.Code -ne 0)
$r = Invoke-Pvm @('pkg', 'go', '--type', 'freebsd')
Check 'invalid --type rejected' ($r.Code -ne 0)
$r = Invoke-Pvm @('pkg', 'node', '--entry', 'nope.js', '--type', 'host')
Check 'missing --entry rejected' ($r.Code -ne 0)
$r = Invoke-Pvm @('pkg', 'rust')
Check 'unsupported runtime rejected' ($r.Code -ne 0)

# ---------------------------------------------------------------- validate
Section 'validate'
Enter $repo
# validate 同时检查 shims/版本/PATH，本机 shims 有问题时整体退出码为 1；
# 这里只断言它走到了 pvm.yaml 检查（logger 输出在 stderr，故看 stdout 的步骤标记）。
$r = Invoke-Pvm @('validate')
Check 'pvm validate reaches the pvm.yaml check' ($r.Out -match '\[5/5\] Checking pvm\.yaml') $r.Out.Trim()
Leave

# ---------------------------------------------------------------- 汇总
Section 'summary'
$pass = @($results | Where-Object { $_.Ok }).Count
$fail = @($results | Where-Object { -not $_.Ok }).Count
$results | Format-Table -AutoSize Case, Ok | Out-String | Write-Host
Write-Host ("  {0} passed, {1} failed" -f $pass, $fail) -ForegroundColor $(if ($fail -eq 0) { 'Green' } else { 'Red' })

if ($fail -gt 0) { exit 1 }
exit 0
