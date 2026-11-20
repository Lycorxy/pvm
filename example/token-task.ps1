#Requires -Version 5.1
<#
.SYNOPSIS
    任务级真实对比：把 node-hello 变成"可运行的单文件产物"，两条路径都真跑一遍。

.DESCRIPTION
    路径 A（不用 pvm）：探索 → 装依赖 → 跑起来 → esbuild → 写 sea-config →
                       生成 blob → 复制 node 二进制 → postject 注入 → 验证产物。
    路径 B（pvm）：pvm run → pvm run release:dev → 验证产物。

    每一步都是真实执行，stdout+stderr 原样落盘，最后用 tiktoken(cl100k_base)
    统计真实 token。没有估算值。

    依赖：python + tiktoken（脚本自动装到 %TEMP%\tk，不污染环境）。

.EXAMPLE
    .\token-task.ps1
#>
[CmdletBinding()]
param(
    [switch]$SkipInstall
)

$ErrorActionPreference = 'Continue'
$example = $PSScriptRoot
$nodeDir = Join-Path $example 'node-hello'
$workRoot = Join-Path $example '.work'
$taskA = Join-Path $workRoot 'task-a'
$logDir = Join-Path $workRoot 'task-logs'
$steps = New-Object System.Collections.ArrayList
$stepNo = 0

function Resolve-Exe([string]$name) {
    (Get-Command $name -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
}

$pvm = Resolve-Exe 'pvm'
$node = Resolve-Exe 'node'
$npm = Resolve-Exe 'npm'
$npx = Resolve-Exe 'npx'

# ---- tiktoken（真实 tokenizer，装到临时目录）
$tkDir = Join-Path $env:TEMP 'tk'
if (-not (Test-Path (Join-Path $tkDir 'tiktoken'))) {
    Write-Host 'installing tiktoken (one-time)...' -ForegroundColor DarkGray
    & python -m pip install tiktoken --target $tkDir --quiet --disable-pip-version-check | Out-Null
}
$env:PYTHONPATH = $tkDir

function Step($path, $label, $dir, $exe, [string[]]$argv) {
    $script:stepNo++
    $name = $path + ('{0:D2}_' -f $script:stepNo) + ($label -replace '[^\w]+', '-')
    $outFile = Join-Path $logDir ($name + '.log')
    $errFile = Join-Path $logDir ($name + '.err')
    New-Item -ItemType Directory -Force $logDir | Out-Null
    $display = (Split-Path -Leaf $exe) + ' ' + ($argv -join ' ')
    try {
        $p = Start-Process -FilePath $exe -ArgumentList $argv -WorkingDirectory $dir `
            -NoNewWindow -Wait -PassThru -RedirectStandardOutput $outFile -RedirectStandardError $errFile
        $code = $p.ExitCode
    }
    catch {
        $code = -1
        "start failed: $($_.Exception.Message)" | Set-Content $outFile -Encoding utf8
    }
    [void]$steps.Add([pscustomobject]@{
            Path = $path; No = $script:stepNo; Label = $label; Command = $display; Exit = $code
            Bytes = (Get-Item $outFile).Length + (Get-Item $errFile).Length
        })
    Write-Host ("  {0,-30} exit={1} {2}B" -f $label, $code, ((Get-Item $outFile).Length + (Get-Item $errFile).Length)) -ForegroundColor DarkGray
}

# ---------------------------------------------------------------- 准备
New-Item -ItemType Directory -Force $workRoot | Out-Null
Remove-Item -Recurse -Force $taskA -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $logDir -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force (Join-Path $nodeDir 'dist') -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $taskA | Out-Null
Copy-Item (Join-Path $nodeDir 'package.json') $taskA
Copy-Item (Join-Path $nodeDir 'src') $taskA -Recurse
Copy-Item (Join-Path $nodeDir 'scripts') $taskA -Recurse

Write-Host ''
Write-Host '== path A: no pvm (12 steps)' -ForegroundColor Cyan
Step 'A' 'list files' $taskA (Resolve-Exe 'cmd') @('/c', 'dir', '/b')
Step 'A' 'read package.json' $taskA (Resolve-Exe 'cmd') @('/c', 'type', 'package.json')
Step 'A' 'list npm scripts' $taskA $npm @('run')
Step 'A' 'npm install' $taskA $npm @('install', '--no-audit', '--no-fund')
Step 'A' 'gen config' $taskA $node @('scripts/gen-config.js', 'dev')
Step 'A' 'run app (dev)' $taskA $npm @('start', '--', 'pvm')
Step 'A' 'esbuild bundle' $taskA $npx @('--yes', 'esbuild@0.25', (Join-Path $taskA 'src\main.js'),
    '--bundle', '--platform=node', '--format=cjs', ('--outfile=' + (Join-Path $taskA 'bundle.js')))

# AI 还要自己写 sea-config.json —— 这份内容也是它要产出的 token
$seaConfig = @{
    main                          = (Join-Path $taskA 'bundle.js').Replace('\', '/')
    output                        = (Join-Path $taskA 'sea-prep.blob').Replace('\', '/')
    disableExperimentalSEAWarning = $true
    assets                        = @{ 'config.json' = (Join-Path $taskA 'dist\config.json').Replace('\', '/') }
} | ConvertTo-Json -Depth 4
$seaConfigPath = Join-Path $taskA 'sea-config.json'
[IO.File]::WriteAllText($seaConfigPath, $seaConfig, (New-Object Text.UTF8Encoding($false)))
$authoredLog = Join-Path $logDir ('A' + ('{0:D2}_' -f (++$script:stepNo)) + 'write-sea-config.log')
Copy-Item $seaConfigPath $authoredLog
[void]$steps.Add([pscustomobject]@{
        Path = 'A'; No = $script:stepNo; Label = 'write sea-config'; Command = 'author sea-config.json'
        Exit = 0; Bytes = (Get-Item $authoredLog).Length
    })
Write-Host ("  {0,-30} exit=0 {1}B" -f 'write sea-config', (Get-Item $authoredLog).Length) -ForegroundColor DarkGray

Step 'A' 'sea blob' $taskA $node @('--experimental-sea-config', $seaConfigPath)
$nodeExe = (& $node -p 'process.execPath').Trim()
Step 'A' 'copy node binary' $taskA (Resolve-Exe 'cmd') @('/c', 'copy', $nodeExe, (Join-Path $taskA 'app.exe'))
Step 'A' 'postject inject' $taskA $npx @('--yes', 'postject@1.0.0-alpha.4', (Join-Path $taskA 'app.exe'),
    'NODE_SEA_BLOB', (Join-Path $taskA 'sea-prep.blob'),
    '--sentinel-fuse', 'NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2')
Step 'A' 'run artifact' $taskA (Join-Path $taskA 'app.exe') @('from-binary')

Write-Host ''
Write-Host '== path B: with pvm (3 steps)' -ForegroundColor Cyan
Step 'B' 'pvm run (list)' $nodeDir $pvm @('run')
Step 'B' 'pvm run release:dev' $nodeDir $pvm @('run', 'release:dev')
Step 'B' 'run artifact' $nodeDir (Join-Path $nodeDir 'dist\node-hello.exe') @('from-binary')

# ---------------------------------------------------------------- 真实 token 统计
$py = @'
import sys, os, tiktoken
enc = tiktoken.get_encoding("cl100k_base")
for f in sys.argv[1:]:
    text = open(f, "rb").read().decode("utf-8", "replace")
    print(f"{os.path.basename(f)}\t{len(text)}\t{len(enc.encode(text))}")
'@
$pyFile = Join-Path $workRoot 'count_tokens.py'
[IO.File]::WriteAllText($pyFile, $py, (New-Object Text.UTF8Encoding($false)))

$counts = @{}
& python $pyFile (Get-ChildItem $logDir -File).FullName | ForEach-Object {
    $parts = $_ -split "`t"
    $counts[$parts[0]] = @{ Chars = [int]$parts[1]; Tokens = [int]$parts[2] }
}

# 按路径汇总
$aChars = 0; $aTokens = 0; $bChars = 0; $bTokens = 0
foreach ($f in $counts.Keys) {
    if ($f -like 'A*') { $aChars += $counts[$f].Chars; $aTokens += $counts[$f].Tokens }
    else { $bChars += $counts[$f].Chars; $bTokens += $counts[$f].Tokens }
}
$aSteps = ($steps | Where-Object { $_.Path -eq 'A' }).Count
$bSteps = ($steps | Where-Object { $_.Path -eq 'B' }).Count

Write-Host ''
Write-Host '==== task: node-hello -> single executable ====' -ForegroundColor Cyan
Write-Host ("path A (no pvm)  : {0,2} steps  {1,7} chars  {2,6} tokens" -f $aSteps, $aChars, $aTokens)
Write-Host ("path B (pvm)     : {0,2} steps  {1,7} chars  {2,6} tokens" -f $bSteps, $bChars, $bTokens)
Write-Host ''
Write-Host 'per-step (tokens):' -ForegroundColor Cyan
$steps | ForEach-Object {
    $st = $_
    $key = ($counts.Keys | Where-Object { $_ -like ($st.Path + ('{0:D2}_*' -f $st.No)) } | Select-Object -First 1)
    $t = 0
    if ($key) { $t = $counts[$key].Tokens }
    [pscustomobject]@{ Path = $st.Path; No = $st.No; Label = $st.Label; Exit = $st.Exit; Tokens = $t }
} | Format-Table -AutoSize | Out-String -Width 160 | Write-Host

Write-Host 'note: only measured values above (cl100k_base).' -ForegroundColor DarkGray
Write-Host '      not measured: model reasoning tokens, tool-call overhead, prompt/skill loading.' -ForegroundColor DarkGray
$steps | Export-Csv (Join-Path $workRoot 'token-task.csv') -NoTypeInformation -Encoding utf8
Write-Host ("raw logs: {0}" -f $logDir)
