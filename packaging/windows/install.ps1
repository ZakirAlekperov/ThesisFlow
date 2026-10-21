# Installs ThesisFlow for the current Windows user; no administrator rights.
# RU: Устанавливает ThesisFlow для текущего пользователя без прав администратора.
$ErrorActionPreference = "Stop"
$installDir = Join-Path $env:LOCALAPPDATA "Programs\ThesisFlow"
$source = Join-Path $PSScriptRoot "thesisflow.exe"
if (!(Test-Path -LiteralPath $source)) {
    throw "thesisflow.exe was not found alongside install.ps1"
}
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item -LiteralPath $source -Destination (Join-Path $installDir "thesisflow.exe") -Force
$startMenu = [Environment]::GetFolderPath("Programs")
$linkPath = Join-Path $startMenu "ThesisFlow.lnk"
$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut($linkPath)
$link.TargetPath = Join-Path $installDir "thesisflow.exe"
$link.WorkingDirectory = $installDir
$link.Description = "Локальный помощник преподавателя"
$link.Save()
Write-Host "ThesisFlow installed at $installDir"
Write-Host "Your student database is preserved in your user configuration directory."
