# Uninstalls the executable and shortcut, NEVER user academic records.
# RU: Удаляет программу и ярлык, но НЕ удаляет базы и файлы ВКР.
$ErrorActionPreference = "Stop"
$installDir = Join-Path $env:LOCALAPPDATA "Programs\ThesisFlow"
$binary = Join-Path $installDir "thesisflow.exe"
$shortcut = Join-Path ([Environment]::GetFolderPath("Programs")) "ThesisFlow.lnk"
if (Test-Path -LiteralPath $shortcut) { Remove-Item -LiteralPath $shortcut }
if (Test-Path -LiteralPath $binary) { Remove-Item -LiteralPath $binary }
if ((Test-Path -LiteralPath $installDir) -and
    @((Get-ChildItem -LiteralPath $installDir -Force)).Count -eq 0) {
    Remove-Item -LiteralPath $installDir
}
Write-Host "ThesisFlow program removed. Student records and backups were NOT deleted."
