param([string]$OutputDirectory = '.nsh/out/windows')

$ErrorActionPreference = 'Stop'
if (!$IsWindows -or [Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') {
    throw 'Package 香篆 on Windows x64.'
}
$output = [IO.Path]::GetFullPath($OutputDirectory, $PWD.Path)
$root = Split-Path -Parent $PSScriptRoot
$stage = Join-Path $root ".nsh/windows-stage-$([Guid]::NewGuid().ToString('N'))"
$package = Join-Path $stage 'package'
$config = Get-Content (Join-Path $root 'nsh.json') -Raw -Encoding utf8 | ConvertFrom-Json -AsHashtable

try {
    New-Item -ItemType Directory -Path $stage, $output -Force | Out-Null
    foreach ($entry in 'App.tsx', 'src', 'assets', 'theme.json', 'tsconfig.json') {
        Copy-Item (Join-Path $root $entry) -Destination $stage -Recurse
    }
    Copy-Item (Join-Path $root 'native/windows') -Destination $stage -Recurse
    $config['rust'] = 'windows'
    $config['singleInstance'] = $true
    $config | ConvertTo-Json -Depth 32 | Set-Content (Join-Path $stage 'nsh.json') -Encoding utf8NoBOM

    Push-Location $stage
    try {
        & nsh package App.tsx --target windows -o $package
        if ($LASTEXITCODE -ne 0) { throw "nsh package failed ($LASTEXITCODE)" }
    } finally {
        Pop-Location
    }

    $version = $config['version']
    $name = $config['name']
    $archives = @(Get-ChildItem -LiteralPath $package -File -Filter "*-$version-windows-x86_64.zip")
    if ($archives.Count -eq 0) {
        $archives = @(Get-Item -LiteralPath (Join-Path $package "$name.zip"))
    }
    $installers = @(Get-ChildItem -LiteralPath $package -File -Filter '*-setup.exe')
    if ($archives.Count -ne 1 -or $installers.Count -ne 1) {
        throw 'Expected one Windows update ZIP and one installer. Check the Inno Setup installation.'
    }
    Copy-Item -LiteralPath $archives[0].FullName -Destination (Join-Path $output "xiangzhuan-$version-windows-x86_64.zip") -Force
    Copy-Item -LiteralPath $installers[0].FullName `
        -Destination (Join-Path $output "xiangzhuan-$version-windows-x64-setup.exe") -Force
} finally {
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}
