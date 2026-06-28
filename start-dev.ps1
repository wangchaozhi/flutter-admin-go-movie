param(
  [switch]$NoDocker,
  [switch]$NoMobile,
  [switch]$NoWorker,
  [switch]$SkipInstall,
  [switch]$Restart,
  [string]$BindHost = "0.0.0.0",
  [string]$HostName = "192.168.1.15",
  [int]$AdminPort = 5173,
  [int]$MobilePort = 5174
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }
$BackendDir = Join-Path $Root "backend"
$AdminDir = Join-Path $Root "front/admin"
$MobileDir = Join-Path $Root "front/mobile"
$RunRoot = Join-Path $Root ".codex-run"
$Stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$RunDir = Join-Path $RunRoot $Stamp
$PidFile = Join-Path $RunRoot "pids.json"
$TranscodeTempDir = Join-Path $RunRoot "transcode-tmp"
$ComposeNetwork = "$(Split-Path -Leaf $Root)_app-network"
$BackendHTTPAddr = "$BindHost`:8080"
$ApiBaseURL = "http://$HostName`:8080"
$VideoBaseURL = "http://$HostName`:8081"
$DefaultFFmpegBin = "E:\dev\ffmpeg-6.0-full_build\bin"

New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
New-Item -ItemType Directory -Force -Path $TranscodeTempDir | Out-Null

$ffmpegBin = if (-not [string]::IsNullOrWhiteSpace($env:FFMPEG_BIN_DIR)) {
  $env:FFMPEG_BIN_DIR
} elseif (Test-Path (Join-Path $DefaultFFmpegBin "ffmpeg.exe")) {
  $DefaultFFmpegBin
} else {
  ""
}
if (-not [string]::IsNullOrWhiteSpace($ffmpegBin) -and (Test-Path (Join-Path $ffmpegBin "ffmpeg.exe"))) {
  $env:PATH = "$ffmpegBin;$env:PATH"
}

function Write-Step {
  param([string]$Message)
  Write-Host ""
  Write-Host "==> $Message"
}

function Require-Command {
  param([string]$Name)
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command not found: $Name"
  }
}

function Invoke-Checked {
  param(
    [string]$Command,
    [string]$WorkingDirectory
  )

  Push-Location $WorkingDirectory
  try {
    & cmd.exe /c $Command
    if ($LASTEXITCODE -ne 0) {
      throw "Command failed with exit code ${LASTEXITCODE}: $Command"
    }
  }
  finally {
    Pop-Location
  }
}

function Test-ListeningPort {
  param([int]$Port)
  try {
    return [bool](Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
  }
  catch {
    return $false
  }
}

function Get-PortOwners {
  param([int[]]$Ports)
  try {
    return @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
      Where-Object { $Ports -contains $_.LocalPort } |
      Select-Object -ExpandProperty OwningProcess -Unique)
  }
  catch {
    return @()
  }
}

function Get-TrackedPids {
  if (-not (Test-Path $PidFile)) {
    return @()
  }

  try {
    $items = Get-Content -Raw $PidFile | ConvertFrom-Json
    return @($items | ForEach-Object { $_.ProcessId } | Where-Object { $_ })
  }
  catch {
    return @()
  }
}

function Stop-DevProcesses {
  param([int[]]$Ports)

  $portPids = @(Get-PortOwners -Ports $Ports | Where-Object { $_ })
  $trackedPids = @(Get-TrackedPids)
  $pids = @($portPids + $trackedPids | Where-Object { $_ } | Sort-Object -Unique)

  if ($pids.Count -eq 0) {
    Write-Host "No app processes found."
    return
  }

  $processes = @(Get-CimInstance Win32_Process | Where-Object { $pids -contains $_.ProcessId })
  foreach ($process in $processes) {
    Write-Host "Stopping $($process.Name) PID $($process.ProcessId)"
  }

  Stop-Process -Id $pids -Force -ErrorAction SilentlyContinue

  $deadline = (Get-Date).AddSeconds(15)
  do {
    $remaining = @(Get-PortOwners -Ports $Ports | Where-Object { $_ })
    if ($remaining.Count -eq 0) {
      Write-Host "App ports stopped."
      return
    }
    Start-Sleep -Milliseconds 500
  } while ((Get-Date) -lt $deadline)

  Write-Warning "Some app ports are still listening: $($remaining -join ',')"
}

function Test-ContainerRunning {
  param([string]$Name)

  try {
    if (-not (Test-ContainerExists $Name)) {
      return $false
    }
    $status = docker inspect -f "{{.State.Running}}" $Name 2>$null
    return ($LASTEXITCODE -eq 0 -and $status -eq "true")
  }
  catch {
    return $false
  }
}

function Test-ContainerExists {
  param([string]$Name)

  try {
    & cmd.exe /c "docker inspect $Name >nul 2>nul"
    return ($LASTEXITCODE -eq 0)
  }
  catch {
    return $false
  }
}

function Get-ContainerNetworks {
  param([string]$Name)

  try {
    $raw = docker inspect $Name 2>$null
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($raw)) {
      return @()
    }

    $inspect = $raw | ConvertFrom-Json
    if (-not $inspect -or -not $inspect[0].NetworkSettings.Networks) {
      return @()
    }

    return @($inspect[0].NetworkSettings.Networks.PSObject.Properties | ForEach-Object { $_.Name })
  }
  catch {
    return @()
  }
}

function Connect-ContainerNetwork {
  param(
    [string]$Container,
    [string]$Network,
    [string]$Alias
  )

  $networks = @(Get-ContainerNetworks $Container)
  if ($networks -contains $Network) {
    return
  }

  docker network connect --alias $Alias $Network $Container 2>$null
  if ($LASTEXITCODE -eq 0) {
    Write-Host "Connected $Container to $Network as $Alias."
  }
  else {
    Write-Warning "Could not connect $Container to $Network. HLS proxy may be unavailable."
  }
}

function Ensure-DockerNetwork {
  param([string]$Network)

  & cmd.exe /c "docker network inspect $Network >nul 2>nul"
  if ($LASTEXITCODE -eq 0) {
    return
  }

  docker network create $Network | Out-Null
}

function Get-ContainerEnvValue {
  param(
    [string]$Container,
    [string]$Key
  )

  try {
    $raw = docker inspect $Container 2>$null
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($raw)) {
      return ""
    }

    $inspect = $raw | ConvertFrom-Json
    $prefix = "$Key="
    $match = @($inspect[0].Config.Env | Where-Object { $_.StartsWith($prefix) } | Select-Object -First 1)
    if ($match.Count -eq 0) {
      return ""
    }
    return $match[0].Substring($prefix.Length)
  }
  catch {
    return ""
  }
}

function Ensure-NginxVideoProxy {
  if (Test-ContainerRunning "flutter-admin-go-nginx") {
    $currentSecret = Get-ContainerEnvValue -Container "flutter-admin-go-nginx" -Key "HLS_SECRET"
    if ($currentSecret -eq $env:HLS_SECRET) {
      Ensure-DockerNetwork -Network $ComposeNetwork
      Connect-ContainerNetwork -Container "flutter-admin-go-minio" -Network $ComposeNetwork -Alias "minio"
      Write-Host "Nginx video proxy is already running."
      return
    }
    Write-Host "Recreating Nginx video proxy to match HLS_SECRET."
    docker rm -f flutter-admin-go-nginx | Out-Null
  }
  elseif (Test-ContainerExists "flutter-admin-go-nginx") {
    $currentSecret = Get-ContainerEnvValue -Container "flutter-admin-go-nginx" -Key "HLS_SECRET"
    if ($currentSecret -ne $env:HLS_SECRET) {
      Write-Host "Recreating Nginx video proxy to match HLS_SECRET."
      docker rm -f flutter-admin-go-nginx | Out-Null
    }
  }

  Ensure-DockerNetwork -Network $ComposeNetwork
  Connect-ContainerNetwork -Container "flutter-admin-go-minio" -Network $ComposeNetwork -Alias "minio"

  Write-Host "Starting Nginx video proxy."
  Push-Location $Root
  try {
    docker compose up -d --no-deps nginx
    if ($LASTEXITCODE -ne 0) {
      Write-Warning "Could not start Nginx video proxy. HLS playback may be unavailable."
      return
    }
  }
  finally {
    Pop-Location
  }

  if (Test-ContainerRunning "flutter-admin-go-nginx") {
    Write-Host "Nginx video proxy is running."
  }
  else {
    Write-Warning "Nginx video proxy is not running on port 8081. HLS playback may be unavailable."
  }
}

function Start-DockerServices {
  $baseRunning = (Test-ContainerRunning "flutter-admin-go-postgres") -and
    (Test-ContainerRunning "flutter-admin-go-minio") -and
    (Test-ContainerRunning "flutter-admin-go-redis")

  if ($baseRunning) {
    Write-Step "Using shared Docker services"
    Write-Host "Postgres, MinIO, and Redis are already running."
    Ensure-NginxVideoProxy
    return
  }

  Write-Step "Starting Docker services"
  Push-Location $Root
  try {
    docker compose up -d postgres minio redis nginx
    if ($LASTEXITCODE -ne 0) {
      throw "Docker compose failed."
    }
  }
  finally {
    Pop-Location
  }
}

function Wait-Http {
  param(
    [string]$Name,
    [string]$Url,
    [int]$Seconds = 60
  )

  $deadline = (Get-Date).AddSeconds($Seconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
      if ([int]$response.StatusCode -ge 200 -and [int]$response.StatusCode -lt 500) {
        Write-Host "$Name ready: $Url"
        return $true
      }
    }
    catch {
      Start-Sleep -Milliseconds 1000
    }
  }

  Write-Warning "$Name did not respond before timeout: $Url"
  return $false
}

function Start-DevProcess {
  param(
    [string]$Name,
    [int]$Port,
    [string]$FilePath,
    [string[]]$Arguments,
    [string]$WorkingDirectory
  )

  $stdout = Join-Path $RunDir "$Name.out.log"
  $stderr = Join-Path $RunDir "$Name.err.log"

  if ($Port -gt 0 -and (Test-ListeningPort $Port)) {
    Write-Host "$Name already listening on port $Port"
    return [pscustomobject]@{
      Name = $Name
      Status = "already_listening"
      Port = $Port
      ProcessId = $null
      Stdout = $stdout
      Stderr = $stderr
    }
  }

  $startParams = @{
    FilePath = $FilePath
    WorkingDirectory = $WorkingDirectory
    RedirectStandardOutput = $stdout
    RedirectStandardError = $stderr
    WindowStyle = "Hidden"
    PassThru = $true
  }

  $cleanArguments = @($Arguments | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
  if ($cleanArguments.Count -gt 0) {
    $startParams.ArgumentList = $cleanArguments
  }

  $process = Start-Process @startParams

  Write-Host "$Name started with PID $($process.Id), logs: $stdout"
  return [pscustomobject]@{
    Name = $Name
    Status = "started"
    Port = $Port
    ProcessId = $process.Id
    Stdout = $stdout
    Stderr = $stderr
  }
}

Write-Step "Checking tools"
Require-Command "go"
Require-Command "node"
Require-Command "cmd.exe"
if (-not $NoDocker) {
  Require-Command "docker"
}
if (Get-Command "ffmpeg" -ErrorAction SilentlyContinue) {
  Write-Host "  FFmpeg: $((Get-Command "ffmpeg").Source)"
}
if (-not $NoMobile) {
  Require-Command "flutter"
}

if ([string]::IsNullOrWhiteSpace($env:APP_ENV)) {
  $env:APP_ENV = "local"
}
if ([string]::IsNullOrWhiteSpace($env:HLS_SECRET)) {
  $env:HLS_SECRET = "your_hls_secret_key_change_in_prod"
}
if ([string]::IsNullOrWhiteSpace($env:HTTP_ADDR)) {
  $env:HTTP_ADDR = $BackendHTTPAddr
}
if ([string]::IsNullOrWhiteSpace($env:API_BASE_URL)) {
  $env:API_BASE_URL = $ApiBaseURL
}
if ([string]::IsNullOrWhiteSpace($env:VIDEO_BASE_URL)) {
  $env:VIDEO_BASE_URL = $VideoBaseURL
}
if ([string]::IsNullOrWhiteSpace($env:APP_PUBLIC_BASE_URL)) {
  $env:APP_PUBLIC_BASE_URL = $ApiBaseURL
}
if ([string]::IsNullOrWhiteSpace($env:TRANSCODE_TEMP_DIR)) {
  $env:TRANSCODE_TEMP_DIR = $TranscodeTempDir
}

if ($Restart) {
  Write-Step "Stopping app processes"
  Stop-DevProcesses -Ports @(8080, $AdminPort, $MobilePort)
}

if (-not $NoDocker) {
  Start-DockerServices
}

if (-not $SkipInstall) {
  if (-not (Test-Path (Join-Path $AdminDir "node_modules"))) {
    Write-Step "Installing admin dependencies"
    Invoke-Checked -Command "npm install" -WorkingDirectory $AdminDir
  }

  if (-not $NoMobile -and -not (Test-Path (Join-Path $MobileDir ".dart_tool/package_config.json"))) {
    Write-Step "Installing mobile dependencies"
    Invoke-Checked -Command "flutter pub get" -WorkingDirectory $MobileDir
  }
}

$services = @()

Write-Step "Building backend"
$backendExe = Join-Path $RunDir "backend-server.exe"
Invoke-Checked -Command "go build -o `"$backendExe`" ./cmd/server" -WorkingDirectory $BackendDir

Write-Step "Starting backend"
$services += Start-DevProcess `
  -Name "backend" `
  -Port 8080 `
  -FilePath $backendExe `
  -Arguments @() `
  -WorkingDirectory $BackendDir

if (-not $NoWorker) {
  Write-Step "Building worker"
  $workerExe = Join-Path $RunDir "video-worker.exe"
  Invoke-Checked -Command "go build -o `"$workerExe`" ./cmd/worker" -WorkingDirectory $BackendDir

  if (-not (Get-Command "ffmpeg" -ErrorAction SilentlyContinue)) {
    Write-Warning "ffmpeg was not found. Worker can start, but video transcoding tasks will fail until ffmpeg is installed."
  }

  Write-Step "Starting worker"
  $services += Start-DevProcess `
    -Name "worker" `
    -Port 0 `
    -FilePath $workerExe `
    -Arguments @() `
    -WorkingDirectory $BackendDir
}

Write-Step "Starting admin web"
$viteBin = Join-Path $AdminDir "node_modules/vite/bin/vite.js"
if (-not (Test-Path $viteBin)) {
  throw "Vite was not found at $viteBin. Run without -SkipInstall to install admin dependencies."
}
$services += Start-DevProcess `
  -Name "admin" `
  -Port $AdminPort `
  -FilePath "node" `
  -Arguments @($viteBin, "--host", $BindHost, "--port", [string]$AdminPort) `
  -WorkingDirectory $AdminDir

if (-not $NoMobile) {
  Write-Step "Starting mobile web"
  $services += Start-DevProcess `
    -Name "mobile" `
    -Port $MobilePort `
    -FilePath "cmd.exe" `
    -Arguments @("/c", "flutter run -d web-server --web-hostname $BindHost --web-port $MobilePort --dart-define API_BASE_URL=$ApiBaseURL") `
    -WorkingDirectory $MobileDir
}

$services | ConvertTo-Json -Depth 4 | Set-Content -Path $PidFile -Encoding UTF8

Write-Step "Checking services"
[void](Wait-Http -Name "Backend" -Url "http://$HostName`:8080/api/health" -Seconds 60)
[void](Wait-Http -Name "Admin" -Url "http://$HostName`:$AdminPort/" -Seconds 60)
if (-not $NoMobile) {
  [void](Wait-Http -Name "Mobile" -Url "http://$HostName`:$MobilePort/" -Seconds 120)
}
if (Test-ListeningPort 8081) {
  [void](Wait-Http -Name "Nginx" -Url "http://$HostName`:8081/health" -Seconds 10)
}

$ports = if ($NoMobile) { @(8080, $AdminPort) } else { @(8080, $AdminPort, $MobilePort) }
$ownerIds = @(Get-PortOwners -Ports $ports)
$trackedIds = @(Get-TrackedPids)
$stopIds = @($ownerIds + $trackedIds | Where-Object { $_ } | Sort-Object -Unique)

Write-Host ""
Write-Host "Ready"
Write-Host "  Listening: $BindHost"
Write-Host "  Backend: http://$HostName`:8080/api/health"
Write-Host "  Admin:   http://$HostName`:$AdminPort/"
if (-not $NoMobile) {
  Write-Host "  Mobile:  http://$HostName`:$MobilePort/"
}
if (Test-ListeningPort 8081) {
  Write-Host "  Nginx:   http://$HostName`:8081/health"
}
Write-Host "  Logs:    $RunDir"
Write-Host "  PIDs:    $PidFile"
if ($stopIds.Count -gt 0) {
  Write-Host ""
  Write-Host "Stop app processes:"
  Write-Host "  Stop-Process -Id $($stopIds -join ',')"
}
