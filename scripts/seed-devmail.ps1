# seed-devmail.ps1 - Seed the DevMail API with realistic development data.
# Idempotent: existing resources are left untouched, missing ones are created.
#
# Usage:
#   pwsh ./scripts/seed-devmail.ps1
#   pwsh ./scripts/seed-devmail.ps1 -ApiBase http://127.0.0.1:7000/api/v1 -AdminEmail you@example.com
#
# The administrator account must already exist (first-run /auth/setup or an
# existing super_admin). Pass the password on the command line or via
# SEED_ADMIN_PASSWORD to keep it out of shell history.

[CmdletBinding()]
param(
  [string]$ApiBase = "http://127.0.0.1:7000/api/v1",
  [string]$AdminEmail = "joachim.euphrase@gmail.com",
  [string]$AdminPassword = $env:SEED_ADMIN_PASSWORD
)

$ErrorActionPreference = "Stop"

if (-not $AdminPassword) {
  Write-Warning "SEED_ADMIN_PASSWORD is not set."
  $AdminPassword = Read-Host -AsSecureString "Admin password for $AdminEmail"
  $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($AdminPassword)
  $AdminPassword = [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
  [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
}

$script:session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$script:csrfToken = ""

function Invoke-Api {
  param(
    [string]$Method,
    [string]$Path,
    [object]$Body = $null,
    [switch]$AllowConflict
  )
  $headers = @{}
  if ($script:csrfToken) { $headers["X-CSRF-Token"] = $script:csrfToken }
  $params = @{
    Method      = $Method
    Uri         = "$ApiBase$Path"
    WebSession  = $script:session
    Headers     = $headers
    ErrorAction = "Stop"
  }
  if ($null -ne $Body) {
    $params.Body = ($Body | ConvertTo-Json -Depth 8)
    $params.ContentType = "application/json"
  }
  try {
    $resp = Invoke-RestMethod @params
    return $resp
  } catch {
    if ($AllowConflict -and [int]$_.Exception.Response.StatusCode -eq 409) {
      return $null
    }
    throw
  }
}

function Ensure {
  param(
    [string]$Kind,
    [object]$Item,
    [string]$IdKey,
    [hashtable]$Existing,
    [scriptblock]$Transform = $null
  )
  $id = $Item[$IdKey]
  if ($Existing.ContainsKey([string]$id)) {
    Write-Host "  skip $Kind '$id' (already present)"
    return $Existing[[string]$id]
  }
  $payload = if ($Transform) { & $Transform $Item } else { $Item }
  $resp = Invoke-Api -Method POST -Path "/$Kind" -Body $payload -AllowConflict
  if ($null -eq $resp) {
    Write-Host "  skip $Kind '$id' (already present)"
    return $null
  }
  Write-Host "  created $Kind '$id'"
  return $resp.data
}

Write-Host "Logging in as $AdminEmail ..."
$script:csrfToken = (Invoke-Api -Method GET -Path "/auth/csrf").data.token
$login = Invoke-Api -Method POST -Path "/auth/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
if (-not $login.data) { throw "Login failed." }
$script:csrfToken = (Invoke-Api -Method GET -Path "/auth/csrf").data.token
Write-Host "Authenticated as $($login.data.email) ($($login.data.role))."
Write-Host ""

# ---------------------------------------------------------------------------
Write-Host "Seeding servers ..."
$servers = @(
  @{ name = "Euclid Edge";    hostname = "mx1.acme.example";   ipv4 = "203.0.113.10";  ipv6 = "2001:db8:10::1"; location = "Frankfurt, DE"; environment = "Production"; notes = "Primary edge MX for acme.example." },
  @{ name = "Heracles Host";  hostname = "mx2.acme.example";   ipv4 = "198.51.100.20"; ipv6 = "2001:db8:20::1"; location = "Ashburn, US";   environment = "Production"; notes = "Secondary MX, geographically diverse." },
  @{ name = "Sandbox Sailor"; hostname = "sandbox.acme.example"; ipv4 = "192.0.2.10";   ipv6 = "";               location = "Amsterdam, NL"; environment = "Staging";   notes = "Nightly preview environment; not in production rotation." }
)
$serverIds = @{}
foreach ($s in $servers) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/servers"
    foreach ($item in $list.data) { $existing[[string]$item.hostname] = $item }
  } catch { }
  $resp = Ensure -Kind "servers" -Item $s -IdKey "hostname" -Existing $existing
  if ($resp) { $serverIds[$resp.name] = $resp.id }
}

Write-Host ""
Write-Host "Seeding DNS providers ..."
$providers = @(
  @{ name = "Manual (Acme)";       kind = "manual";     zone = "acme.example"; token = "" },
  @{ name = "Manual (DevMail)";    kind = "manual";     zone = "devmail.example"; token = "" }
)
foreach ($p in $providers) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/providers"
    foreach ($item in $list.data) { $existing[[string]$item.name] = $item }
  } catch { }
  $null = Ensure -Kind "providers" -Item $p -IdKey "name" -Existing $existing
}

Write-Host ""
Write-Host "Seeding domains ..."
$domains = @(
  @{ name = "acme.example";       server = "Euclid Edge";    quotaGB = 20; catchAll = $false },
  @{ name = "relay.acme.example"; server = "Euclid Edge";    quotaGB = 10; catchAll = $true },
  @{ name = "alerts.acme.example"; server = "Heracles Host"; quotaGB = 5;  catchAll = $false },
  @{ name = "staging.acme.example"; server = "Sandbox Sailor"; quotaGB = 2; catchAll = $false }
)
$domainIds = @{}
foreach ($d in $domains) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/domains"
    foreach ($item in $list.data) { $existing[[string]$item.name] = $item }
  } catch { }
  $q = [int64]$d.quotaGB * 1024 * 1024 * 1024
  if ($existing.ContainsKey([string]$d.name)) {
    Write-Host "  skip domain '$($d.name)' (already present)"
    $domainIds[$d.name] = $existing[[string]$d.name].id
    continue
  }
  $payload = @{ name = $d.name; quotaBytes = $q; catchAll = $d.catchAll }
  if ($serverIds[$d.server]) { $payload.serverId = $serverIds[$d.server] }
  $resp = Invoke-Api -Method POST -Path "/domains" -Body $payload -AllowConflict
  if ($resp) {
    Write-Host "  created domain '$($d.name)'"
    $domainIds[$d.name] = $resp.data.id
  }
}

Write-Host ""
Write-Host "Seeding mailboxes ..."
$emailPw = "Str0ngMail2026!"
$mailboxes = @(
  @{ local = "postmaster"; display = "Postmaster";            domain = "acme.example";        quotaGB = 4 },
  @{ local = "info";       display = "General Inquiries";     domain = "acme.example";        quotaGB = 8 },
  @{ local = "sales";      display = "Sales Team";            domain = "acme.example";        quotaGB = 8 },
  @{ local = "noreply";    display = "Automated Mailer";      domain = "relay.acme.example";  quotaGB = 4 },
  @{ local = "oncall";     display = "On-Call Alerts";        domain = "alerts.acme.example"; quotaGB = 2 },
  @{ local = "dev";        display = "Development";           domain = "staging.acme.example"; quotaGB = 1 }
)
foreach ($m in $mailboxes) {
  $address = "$($m.local)@$($m.domain)"
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/domains/$($domainIds[$m.domain])/mailboxes"
    foreach ($item in $list.data) { $existing[[string]$item.address] = $item }
  } catch { }
  if ($existing.ContainsKey($address)) {
    Write-Host "  skip mailbox '$address' (already present)"
    continue
  }
  $q = [int64]$m.quotaGB * 1024 * 1024 * 1024
  $resp = Invoke-Api -Method POST -Path "/domains/$($domainIds[$m.domain])/mailboxes" `
    -Body @{ local = $m.local; displayName = $m.display; quotaBytes = $q; password = $emailPw } -AllowConflict
  if ($resp) { Write-Host "  created mailbox '$address'" }
}

Write-Host ""
Write-Host "Seeding aliases ..."
$aliases = @(
  @{ source = "support"; destination = "info@acme.example";       domain = "acme.example" },
  @{ source = "team";    destination = "sales@acme.example";      domain = "acme.example" },
  @{ source = "alerts";  destination = "oncall@alerts.acme.example"; domain = "alerts.acme.example" }
)
foreach ($a in $aliases) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/domains/$($domainIds[$a.domain])/aliases"
    foreach ($item in $list.data) { $existing[[string]$item.source] = $item }
  } catch { }
  if ($existing.ContainsKey($a.source)) {
    Write-Host "  skip alias '$($a.source)@$($a.domain)' (already present)"
    continue
  }
  $resp = Invoke-Api -Method POST -Path "/domains/$($domainIds[$a.domain])/aliases" `
    -Body @{ source = $a.source; destination = $a.destination; forwarding = $false } -AllowConflict
  if ($resp) { Write-Host "  created alias '$($a.source)@$($a.domain)'" }
}

Write-Host ""
Write-Host "Seeding team members ..."
$userPw = "DevMailTeam2026!"
$users = @(
  @{ email = "nova@acme.example"; name = "Nova Alvarez"; role = "server_admin" },
  @{ email = "theo@acme.example"; name = "Theo Okonkwo"; role = "domain_admin" },
  @{ email = "mae@acme.example";  name = "Mae Chen";     role = "operator" }
)
foreach ($u in $users) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/users"
    foreach ($item in $list.data) { $existing[[string]$item.email] = $item }
  } catch { }
  if ($existing.ContainsKey($u.email)) {
    Write-Host "  skip user '$($u.email)' (already present)"
    continue
  }
  $resp = Invoke-Api -Method POST -Path "/users" `
    -Body @{ email = $u.email; name = $u.name; role = $u.role; password = $userPw } -AllowConflict
  if ($resp) { Write-Host "  created user '$($u.email)' ($($u.role))" }
}

Write-Host ""
Write-Host "Seeding routing rules ..."
$routingRules = @(
  @{ name = "Postmaster forwarder"; enabled = $true; config = @{
      source = "postmaster@acme.example"; destination = "admin@acme.example"; match = "postmaster@acme.example"; type = "Forwarding"; priority = "10"
    } },
  @{ name = "List digest";           enabled = $true; config = @{
      source = "list@acme.example"; destination = "digest@relay.acme.example"; match = "list@acme.example"; type = "Forwarding"; priority = "20"
    } }
)
foreach ($r in $routingRules) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/routing-rules"
    foreach ($item in $list.data) { $existing[[string]$item.name] = $item }
  } catch { }
  $null = Ensure -Kind "routing-rules" -Item $r -IdKey "name" -Existing $existing
}

Write-Host ""
Write-Host "Seeding backup targets ..."
$backups = @(
  @{ name = "Nightly Euclid Edge"; enabled = $true; config = @{ kind = "postgres"; target = "Euclid Edge"; schedule = "0 2 * * *"; retention = "30d"; destination = "s3://devmail-backups/euclid" } },
  @{ name = "Weekly Heracles";     enabled = $true; config = @{ kind = "postgres"; target = "Heracles Host"; schedule = "0 3 * * 0"; retention = "90d"; destination = "gs://devmail-backups/heracles" } }
)
foreach ($b in $backups) {
  $existing = @{}
  try {
    $list = Invoke-Api -Method GET -Path "/backups"
    foreach ($item in $list.data) { $existing[[string]$item.name] = $item }
  } catch { }
  $null = Ensure -Kind "backups" -Item $b -IdKey "name" -Existing $existing
}

Write-Host ""
Write-Host "Updating workspace settings ..."
$settingsBody = @{
  name      = "DevMail"
  timezone  = "Africa/Nairobi"
  webmail   = "https://webmail.acme.example"
}
try {
  $settings = Invoke-Api -Method PATCH -Path "/settings" -Body $settingsBody
  Write-Host "  workspace name set to '$($settings.data.name)', timezone $($settings.data.timezone)"
} catch {
  Write-Warning "  settings update failed: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "Seed complete. Dashboard summary:"
$dash = Invoke-Api -Method GET -Path "/dashboard"
Write-Host "  $($dash.data | ConvertTo-Json -Compress -Depth 4)"