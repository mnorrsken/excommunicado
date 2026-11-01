# Starts Excommunicado and makes Windows trust its localhost certificate.
# Run from the repo folder:
#   powershell -ExecutionPolicy Bypass -File scripts\setup.ps1
# Windows shows one security prompt for the certificate. Answer Yes.

$ErrorActionPreference = "Stop"
$base = "https://localhost:47823"
$certFile = Join-Path $env:TEMP "excommunicado-cert.pem"

Set-Location (Split-Path $PSScriptRoot -Parent)

if (Get-Command podman -ErrorAction SilentlyContinue) {
    $runtime = "podman"
} elseif (Get-Command docker -ErrorAction SilentlyContinue) {
    $runtime = "docker"
} else {
    throw "Install Podman or Docker first."
}

Write-Host "Downloading the images with $runtime. The first time is about 900 MB."
& $runtime compose pull
if ($LASTEXITCODE -ne 0) { throw "$runtime compose pull failed." }
& $runtime compose up -d --no-build
if ($LASTEXITCODE -ne 0) { throw "$runtime compose up failed." }

# curl.exe ships with Windows 10 and later. -k is needed here because the certificate
# is what we are about to trust, and the connection only goes to this PC.
Write-Host "Waiting for the model to load..."
$ready = $false
for ($i = 0; $i -lt 90; $i++) {
    $health = (& curl.exe -sk --max-time 3 "$base/api/health") -join ""
    if ($health -match '"llm_ready":true') { $ready = $true; break }
    Start-Sleep -Seconds 2
}
if (-not $ready) { throw "The model did not start. See: $runtime compose logs llama" }

& curl.exe -skf -o $certFile "$base/cert.pem"
if ($LASTEXITCODE -ne 0) { throw "Could not get the certificate." }

# The current user's store needs no admin rights. The certificate is valid for localhost only.
& certutil.exe -user -addstore Root $certFile | Out-Null
if ($LASTEXITCODE -ne 0) { throw "The certificate was not installed. Did you answer No in the prompt?" }
Remove-Item $certFile

& curl.exe -sf --max-time 5 -o NUL "$base/api/health"
if ($LASTEXITCODE -ne 0) {
    Write-Warning "curl does not trust the certificate yet. Restart Outlook, and see Troubleshooting in README.md if the add-in stays blank."
}

Write-Host ""
Write-Host "Done. Now add the add-in in Outlook:"
Write-Host "  1. Open https://aka.ms/olksideload and sign in"
Write-Host "  2. Custom Addins > Add a custom add-in > Add from File"
Write-Host "  3. Choose $(Join-Path (Get-Location) 'internal\server\web\manifest.xml')"
Write-Host "  4. Open a mail, then Apps (or the ribbon) > Bullshit score. Pin the pane to keep it open."
