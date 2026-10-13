param([string]$Base = 'http://127.0.0.1:17001/api/v1')
$ErrorActionPreference = 'Stop'
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
function Call-Api([string]$Method, [string]$Path, $Body = $null) {
 $headers = @{}
 if ($Method -notin @('GET','HEAD')) { $csrf = Invoke-RestMethod -Uri "$Base/auth/csrf" -WebSession $session; $headers['X-CSRF-Token'] = $csrf.data.token }
 $params = @{ Headers=$headers; Method=$Method; Uri="$Base$Path"; WebSession=$session; ContentType='application/json' }
 if ($null -ne $Body) { $params.Body = ($Body | ConvertTo-Json -Depth 10 -Compress) }
 return (Invoke-RestMethod @params).data
}
$password = [guid]::NewGuid().ToString() + '!A8'
$email = 'verify-' + [guid]::NewGuid().ToString('N') + '@example.test'
$status = Call-Api GET '/auth/status'
if ($status.hasUsers) { throw 'Verification requires a fresh dedicated database; existing accounts will not be changed.' }
$null = Call-Api POST '/auth/setup' @{email=$email;name='Verification';password=$password}
$null = Call-Api POST '/auth/login' @{email=$email;password=$password}
$domain = Call-Api POST '/domains' @{name=('verify-' + [guid]::NewGuid().ToString('N') + '.example.test')}
$mailbox = Call-Api POST "/domains/$($domain.id)/mailboxes" @{local='alice';displayName='Alice';password=$password;quotaBytes=1073741824}
$list = Call-Api GET "/domains/$($domain.id)/mailboxes"
if ($list.Count -ne 1 -or $list[0].address -ne "alice@$($domain.name)") { throw 'Mailbox listing failed' }
$deadline = (Get-Date).AddSeconds(15)
do {
 Start-Sleep -Milliseconds 250
 $state = Call-Api GET "/mailboxes/$($mailbox.id)/provisioning"
} until ($state.status -eq 'managed' -or (Get-Date) -gt $deadline)
if ($state.status -ne 'managed') { throw "Provisioning did not complete: $($state.status)" }
$null = Call-Api PATCH "/mailboxes/$($mailbox.id)" @{displayName='Updated';quotaBytes=2147483648;password=$password}
$alias = Call-Api POST "/domains/$($domain.id)/aliases" @{source='sales';destination="alice@$($domain.name)";forwarding=$true}
$null = Call-Api DELETE "/aliases/$($alias.id)"
$null = Call-Api DELETE "/mailboxes/$($mailbox.id)"
$null = Call-Api DELETE "/domains/$($domain.id)"
Write-Output 'PASS: authenticated domain/mailbox/alias CRUD, live listing, SQL provisioning, teardown'
