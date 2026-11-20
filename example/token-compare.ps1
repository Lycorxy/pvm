#Requires -Version 5.1
<#
.SYNOPSIS
    命令级实测：统计每条命令的真实输出量（stdout+stderr），用 tiktoken(cl100k_base) 计 token。

.DESCRIPTION
    只做测量，不做估算。每条命令真跑一次，输出落盘到 .work\cmp-logs，
    最后用 tiktoken 逐文件编码，得到真实 token 数。
    依赖 python + tiktoken（自动装到 %TEMP%\tk，不污染环境）。

.EXAMPLE
    .\token-compare.ps1
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'
$example = $PSScriptRoot
$repo = Split-Path -Parent $example
$workRoot = Join-Path $example '.work'
$logDir = Join-Path $workRoot 'cmp-logs'
$rows = New-Object System.Collections.ArrayList
$idx = 0

function Resolve-Exe([string]$name) {
    (Get-Command $name -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
}

$pvm = Resolve-Exe 'pvm'
$node = Resolve-Exe 'node'
$npm = Resolve-Exe 'npm'
$go = Resolve-Exe 'go'
$python = Resolve-Exe 'python'
$gofmt = Resolve-Exe 'gofmt'

$tkDir = Join-Path $env:TEMP 'tk'
if (-not (Test-Path (Join-Path $tkDir 'tiktoken'))) {
    Write-Host 'installing tiktoken (one-time)...' -ForegroundColor DarkGray
    & python -m pip install tiktoken --target $tkDir --quiet --disable-pip-version-check | Out-Null
}
$env:PYTHONPATH = $tkDir

Remove-Item -Recurse -Force $logDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $logDir | Out-Null

function Measure-Cmd($group, $label, $dir, $exe, [string[]]$argv) {
    $script:idx++
    $name = '{0:D2}_{1}' -f $script:idx, ($label -replace '[^\w]+', '-')
    $out = Join-Path $logDir ($name + '.log')
    $err = Join-Path $logDir ($name + '.err')
    $display = (Split-Path -Leaf $exe) + ' ' + ($argv -join ' ')

    # 用 .NET Process 而不是 Start-Process：后者在命令秒退时读不到 ExitCode
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $exe
    $psi.WorkingDirectory = $dir
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.Arguments = (($argv | ForEach-Object { if ($_ -match '\s') { '"' + $_ + '"' } else { $_ } }) -join ' ')
    $enc = New-Object Text.UTF8Encoding($false)
    try {
        $p = [System.Diagnostics.Process]::Start($psi)
        $stdout = $p.StandardOutput.ReadToEnd()
        $stderr = $p.StandardError.ReadToEnd()
        $p.WaitForExit()
        $code = $p.ExitCode
        [IO.File]::WriteAllText($out, $stdout, $enc)
        [IO.File]::WriteAllText($err, $stderr, $enc)
    }
    catch {
        $code = -1
        [IO.File]::WriteAllText($out, "start failed: $($_.Exception.Message)", $enc)
        [IO.File]::WriteAllText($err, '', $enc)
    }
    [void]$rows.Add([pscustomobject]@{
            Group = $group; Case = $label; Command = $display; Exit = $code
            Bytes = (Get-Item $out).Length + (Get-Item $err).Length; File = $name
        })
    Write-Host ("  {0,-26} exit={1}" -f $label, $code) -ForegroundColor DarkGray
}

function Measure-File($group, $label, $path) {
    $script:idx++
    $name = '{0:D2}_{1}' -f $script:idx, ($label -replace '[^\w]+', '-')
    $dst = Join-Path $logDir ($name + '.log')
    Copy-Item $path $dst
    [void]$rows.Add([pscustomobject]@{
            Group = $group; Case = $label; Command = (Resolve-Path -Relative $path); Exit = 0
            Bytes = (Get-Item $dst).Length; File = $name
        })
    Write-Host ("  {0,-26} file" -f $label) -ForegroundColor DarkGray
}

$nodeDir = Join-Path $example 'node-hello'
$goDir = Join-Path $example 'go-hello'
$pyDir = Join-Path $example 'python-hello'
$demoDir = Join-Path $example 'pipeline-demo'

Write-Host 'measuring... (go build/test + npm + pvm pkg, ~40s)' -ForegroundColor DarkGray

# ---- 1. 摸清一个陌生项目
Measure-Cmd 'explore' 'list files' $nodeDir (Resolve-Exe 'cmd') @('/c', 'dir', '/b')
Measure-Cmd 'explore' 'read package.json' $nodeDir (Resolve-Exe 'cmd') @('/c', 'type', 'package.json')
Measure-Cmd 'explore' 'list npm scripts' $nodeDir $npm @('run')
Measure-Cmd 'explore' 'guess wrong script' $nodeDir $npm @('run', 'build')
Measure-Cmd 'explore' 'pvm: capability list' $nodeDir $pvm @('run')
Measure-Cmd 'explore' 'pvm: dry plan' $nodeDir $pvm @('run', 'release:dev', '--dry-run')

# ---- 2. 多项目确认版本
Measure-Cmd 'version' 'node-hello where' $nodeDir (Resolve-Exe 'cmd') @('/c', 'where', 'node')
Measure-Cmd 'version' 'node-hello pvm current' $nodeDir $pvm @('current')
Measure-Cmd 'version' 'go-hello version' $goDir $go @('version')
Measure-Cmd 'version' 'go-hello where' $goDir (Resolve-Exe 'cmd') @('/c', 'where', 'go')
Measure-Cmd 'version' 'go-hello pvm current' $goDir $pvm @('current')
Measure-Cmd 'version' 'python-hello version' $pyDir $python @('--version')
Measure-Cmd 'version' 'python-hello where' $pyDir (Resolve-Exe 'cmd') @('/c', 'where', 'python')
Measure-Cmd 'version' 'python-hello pvm current' $pyDir $pvm @('current')

# ---- 3. 执行一条完整链路
Measure-Cmd 'exec' 'gofmt' $goDir $gofmt @('-l', '.')
Measure-Cmd 'exec' 'go vet' $goDir $go @('vet', './...')
Measure-Cmd 'exec' 'go test' $goDir $go @('test', './...')
Measure-Cmd 'exec' 'go build' $goDir $go @('build', '-trimpath', '-o', 'dist/', '.')
Measure-Cmd 'exec' 'go run' $goDir $go @('run', '.', 'pvm')
Measure-Cmd 'exec' 'pvm one pipeline' $goDir $pvm @('run', 'release:dev')

# ---- 4. 失败信息
Measure-Cmd 'fail' 'npm wrong script' $nodeDir $npm @('run', 'build')
Measure-Cmd 'fail' 'pvm failing step' $demoDir $pvm @('run', 'fail')

# ---- 5. skill 加载成本
Measure-File 'skill' 'SKILL.md' (Join-Path $repo '.codebuddy\skills\pvm\SKILL.md')
Measure-File 'skill' 'pvm-guide.md' (Join-Path $repo '.codebuddy\skills\pvm\references\pvm-guide.md')
Measure-Cmd 'skill' 'harness check' $repo (Resolve-Exe 'node') @('.codebuddy/skills/pvm/scripts/pvm-harness.mjs', 'check')

# ---- 真实 token 统计
$py = @'
import sys, os, tiktoken
enc = tiktoken.get_encoding("cl100k_base")
for f in sys.argv[1:]:
    text = open(f, "rb").read().decode("utf-8", "replace")
    print(f"{os.path.basename(f)}\t{len(enc.encode(text))}")
'@
$pyFile = Join-Path $workRoot 'count_tokens.py'
[IO.File]::WriteAllText($pyFile, $py, (New-Object Text.UTF8Encoding($false)))

$tokens = @{}
& python $pyFile (Get-ChildItem $logDir -File).FullName | ForEach-Object {
    $p = $_ -split "`t"
    $tokens[$p[0]] = [int]$p[1]
}

$rows | ForEach-Object {
    $t = 0
    foreach ($k in $tokens.Keys) {
        if ($k -like ($_.File + '*')) { $t += $tokens[$k] }
    }
    $_ | Add-Member -NotePropertyName Tokens -NotePropertyValue $t
}

$rows | Format-Table -AutoSize Group, Case, Tokens, Exit | Out-String -Width 160 | Write-Host

Write-Host ''
Write-Host '==== summary (tiktoken cl100k_base, measured) ====' -ForegroundColor Cyan

function Group-Stat($group, $filter) {
    $sel = $rows | Where-Object { $_.Group -eq $group -and (& $filter $_) }
    [pscustomobject]@{
        Tokens = ($sel | Measure-Object Tokens -Sum).Sum
        Count  = $sel.Count
    }
}

$et = Group-Stat 'explore' { param($r) $r.Case -notlike 'pvm:*' }
$ep = Group-Stat 'explore' { param($r) $r.Case -like 'pvm:*' }
$vt = Group-Stat 'version' { param($r) $r.Case -notlike '*pvm current' }
$xt = Group-Stat 'exec' { param($r) $r.Case -ne 'pvm one pipeline' }
$xp = Group-Stat 'exec' { param($r) $r.Case -eq 'pvm one pipeline' }

Write-Host ("explore  traditional : {0,5} tok  ({1} cmds)" -f $et.Tokens, $et.Count)
Write-Host ("explore  pvm         : {0,5} tok  ({1} cmds)" -f $ep.Tokens, $ep.Count)
Write-Host ("version  traditional : {0,5} tok  ({1} cmds)" -f $vt.Tokens, $vt.Count)
Write-Host ("exec     traditional : {0,5} tok  ({1} cmds)" -f $xt.Tokens, $xt.Count)
Write-Host ("exec     pvm         : {0,5} tok  ({1} cmd)" -f $xp.Tokens, $xp.Count)
Write-Host ''
$rows | Export-Csv (Join-Path $workRoot 'token-compare.csv') -NoTypeInformation -Encoding utf8
Write-Host "raw logs: $logDir"
