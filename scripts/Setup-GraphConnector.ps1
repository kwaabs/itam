<#
.SYNOPSIS
  Create + test Microsoft Graph (app-only) pull connectors against a running ITAM stack.

.DESCRIPTION
  Logs into GoTrue as the bootstrap admin, then for each selected Graph source it:
    1. (re)creates a pull connector  (direction=pull, OAuth client-credentials)
    2. creates the field mapping       (Graph JSON -> asset/person)
    3. calls /test                     (auth + 1-record probe; confirms scopes)
    4. calls /run                      (full pull + reconcile)
    5. reads back the sync run + discovered fields as proof data landed

  The backend performs the outbound Graph calls, so this script only talks to
  localhost. One connector per source means a missing permission on one source
  does not abort the others.

.PARAMETER ClientSecret
  The Entra app registration client secret. Passed in so it is never stored on disk.

.EXAMPLE
  ./scripts/Setup-GraphConnector.ps1 -ClientSecret 'your-secret-here'

.EXAMPLE
  ./scripts/Setup-GraphConnector.ps1 -ClientSecret '...' -Sources intune,entra_users
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $ClientSecret,

    [string] $Tenant   = '711b8fff-9db3-42cc-bc27-7a19ced91f39',
    [string] $ClientId  = '9e08a85c-2d6d-4e39-8fbd-6877fe9ad55c',
    [string] $Scope     = 'https://graph.microsoft.com/.default',

    [string] $ApiUrl    = 'http://localhost:5607',
    [string] $GoTrueUrl = 'http://localhost:5606',
    [string] $AdminEmail    = 'admin@itam.local',
    [string] $AdminPassword = 'admin12345',

    # Concrete asset type key for ingested devices (must exist + not be abstract).
    [string] $DefaultAssetType = 'laptop',

    # Lifecycle state newly discovered devices enter (NOT "procured").
    [string] $DiscoveredState = 'in_use',

    # Create a stub person when a device's primary user isn't found yet.
    # Leave $false and sync entra_users first for proper names/org units.
    [bool] $AutoCreatePeople = $false,

    [ValidateSet('intune', 'entra_users', 'entra_devices', 'defender')]
    [string[]] $Sources = @('intune', 'entra_users', 'entra_devices', 'defender'),

    [int] $ScheduleMinutes = 60
)

$ErrorActionPreference = 'Stop'
$tokenEndpoint = "https://login.microsoftonline.com/$Tenant/oauth2/v2.0/token"

# --- definitions: one entry per selectable source ---------------------------
$catalog = @{
    intune = @{
        key          = 'intune_devices'
        name         = 'Microsoft Intune devices'
        kind         = 'intune'
        source_object = 'managedDevice'
        url          = 'https://graph.microsoft.com/v1.0/deviceManagement/managedDevices'
        target       = 'asset'
        identity     = 'id'
        fallbacks    = @('serialNumber', 'azureADDeviceId')
        fields = @(
            @{ source = 'deviceName';                target = 'name' }
            @{ source = 'serialNumber';              target = 'serial' }
            @{ source = 'id';                        target = 'external_id' }
            @{ source = 'lastSyncDateTime';          target = 'last_seen'; transform = 'parse_date' }
            # Attribution: link the device to its primary user (a person).
            @{ source = 'userPrincipalName';         target = 'assigned_email' }
            @{ source = 'userDisplayName';           target = 'assigned_name' }
            # Hardware
            @{ source = 'operatingSystem';           target = 'attr.os' }
            @{ source = 'osVersion';                 target = 'attr.os_version' }
            @{ source = 'model';                     target = 'attr.model' }
            @{ source = 'manufacturer';              target = 'attr.manufacturer' }
            @{ source = 'totalStorageSpaceInBytes';  target = 'attr.storage_total_bytes' }
            @{ source = 'freeStorageSpaceInBytes';   target = 'attr.storage_free_bytes' }
            @{ source = 'physicalMemoryInBytes';     target = 'attr.memory_bytes' }
            @{ source = 'wiFiMacAddress';            target = 'attr.wifi_mac' }
            @{ source = 'ethernetMacAddress';        target = 'attr.ethernet_mac' }
            @{ source = 'imei';                      target = 'attr.imei' }
            @{ source = 'phoneNumber';               target = 'attr.phone' }
            # Management / posture
            @{ source = 'userDisplayName';           target = 'attr.primary_user' }
            @{ source = 'emailAddress';              target = 'attr.primary_user_email' }
            @{ source = 'complianceState';           target = 'attr.compliance' }
            @{ source = 'managementState';           target = 'attr.management_state' }
            @{ source = 'managementAgent';           target = 'attr.management_agent' }
            @{ source = 'managedDeviceOwnerType';    target = 'attr.ownership' }
            @{ source = 'deviceEnrollmentType';      target = 'attr.enrollment_type' }
            @{ source = 'enrolledDateTime';          target = 'attr.enrolled_at'; transform = 'parse_date' }
            @{ source = 'isEncrypted';               target = 'attr.encrypted' }
            @{ source = 'jailBroken';                target = 'attr.jailbroken' }
            @{ source = 'azureADDeviceId';           target = 'attr.azure_ad_device_id' }
            @{ source = 'deviceCategoryDisplayName'; target = 'attr.category' }
        )
    }
    entra_users = @{
        key          = 'entra_users'
        name         = 'Microsoft Entra ID users'
        kind         = 'entra'
        source_object = 'user'
        url          = 'https://graph.microsoft.com/v1.0/users?$select=id,userPrincipalName,displayName,givenName,surname,jobTitle,mail,employeeId,accountEnabled'
        target       = 'person'
        identity     = 'id'
        fallbacks    = @('userPrincipalName', 'mail')
        fields = @(
            @{ source = 'userPrincipalName'; target = 'email' }
            @{ source = 'givenName';         target = 'first_name' }
            @{ source = 'surname';           target = 'last_name' }
            @{ source = 'jobTitle';          target = 'title' }
            @{ source = 'employeeId';        target = 'employee_no' }
        )
    }
    entra_devices = @{
        key          = 'entra_devices'
        name         = 'Microsoft Entra ID devices'
        kind         = 'entra'
        source_object = 'device'
        url          = 'https://graph.microsoft.com/v1.0/devices'
        target       = 'asset'
        identity     = 'id'
        fallbacks    = @('deviceId')
        fields = @(
            @{ source = 'displayName';                   target = 'name' }
            @{ source = 'id';                            target = 'external_id' }
            @{ source = 'operatingSystem';               target = 'attr.os' }
            @{ source = 'operatingSystemVersion';        target = 'attr.os_version' }
            @{ source = 'approximateLastSignInDateTime'; target = 'last_seen'; transform = 'parse_date' }
            @{ source = 'deviceId';                      target = 'attr.azure_device_id' }
            @{ source = 'trustType';                     target = 'attr.trust_type' }
            @{ source = 'isManaged';                     target = 'attr.managed' }
        )
    }
    defender = @{
        key          = 'defender_machines'
        name         = 'Microsoft Defender machines'
        kind         = 'defender'
        source_object = 'machine'
        url          = 'https://graph.microsoft.com/v1.0/security/machines'
        target       = 'asset'
        identity     = 'id'
        fallbacks    = @('computerDnsName')
        fields = @(
            @{ source = 'computerDnsName'; target = 'name' }
            @{ source = 'id';              target = 'external_id' }
            @{ source = 'lastSeen';        target = 'last_seen'; transform = 'parse_date' }
            @{ source = 'osPlatform';      target = 'attr.os' }
            @{ source = 'version';         target = 'attr.os_version' }
            @{ source = 'lastIpAddress';   target = 'attr.ip' }
            @{ source = 'riskScore';       target = 'attr.risk_score' }
            @{ source = 'healthStatus';    target = 'attr.health' }
        )
    }
}

# --- helpers ----------------------------------------------------------------
function Invoke-Api {
    param([string]$Method, [string]$Path, $Body)
    $headers = @{ Authorization = "Bearer $script:token" }
    $args = @{ Method = $Method; Uri = "$ApiUrl$Path"; Headers = $headers }
    if ($PSBoundParameters.ContainsKey('Body') -and $null -ne $Body) {
        $args.ContentType = 'application/json'
        $args.Body = ($Body | ConvertTo-Json -Depth 12)
    }
    Invoke-RestMethod @args
}

function Write-Head($t) { Write-Host "`n=== $t ===" -ForegroundColor Cyan }

# --- 1. authenticate --------------------------------------------------------
Write-Head "Logging in to GoTrue ($AdminEmail)"
$login = Invoke-RestMethod -Method Post -Uri "$GoTrueUrl/token?grant_type=password" `
    -ContentType 'application/json' `
    -Body (@{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json)
$script:token = $login.access_token
Write-Host "  token acquired ($($script:token.Length) chars)" -ForegroundColor Green

# Sanity: confirm the asset type exists and is concrete.
try {
    $types = Invoke-Api GET '/api/metadata/asset-types'
    $t = $types | Where-Object { $_.key -eq $DefaultAssetType }
    if (-not $t) {
        $concrete = $types | Where-Object { -not $_.is_abstract } | Select-Object -First 1
        if ($concrete) {
            Write-Host "  '$DefaultAssetType' not found; using '$($concrete.key)'" -ForegroundColor Yellow
            $DefaultAssetType = $concrete.key
        }
    } elseif ($t.is_abstract) {
        Write-Host "  WARNING: asset type '$DefaultAssetType' is abstract; device ingest will error" -ForegroundColor Yellow
    }
} catch { Write-Host "  (could not verify asset types: $($_.Exception.Message))" -ForegroundColor Yellow }

$existing = @(Invoke-Api GET '/api/integration/connectors')
$summary = @()

foreach ($srcKey in $Sources) {
    $def = $catalog[$srcKey]
    Write-Head "$($def.name)  [$($def.source_object) -> $($def.target)]"

    # Replace any prior connector with the same key so re-runs are clean.
    $prev = $existing | Where-Object { $_.key -eq $def.key }
    if ($prev) {
        Write-Host "  removing existing connector '$($def.key)'" -ForegroundColor DarkGray
        Invoke-Api DELETE "/api/integration/connectors/$($prev.id)" | Out-Null
    }

    $config = @{
        schedule_seconds   = $ScheduleMinutes * 60
        default_asset_type = $DefaultAssetType
        discovered_state   = $DiscoveredState
        auto_create_people = $AutoCreatePeople
        auth = @{ token_endpoint = $tokenEndpoint; client_id = $ClientId; scope = $Scope }
        objects = @(@{ source_object = $def.source_object; url = $def.url })
    }
    $created = Invoke-Api POST '/api/integration/connectors' @{
        key = $def.key; name = $def.name; kind = $def.kind
        direction = 'pull'; enabled = $true; config = $config; pull_secret = $ClientSecret
    }
    $cid = $created.connector.id
    Write-Host "  connector created: $cid" -ForegroundColor Green

    $typeRes = @{}
    if ($def.target -eq 'asset') { $typeRes = @{ default = $DefaultAssetType } }
    Invoke-Api POST "/api/integration/connectors/$cid/mappings" @{
        source_object = $def.source_object; target_entity = $def.target
        identity = $def.identity; match_fallbacks = $def.fallbacks
        type_resolution = $typeRes; fields = $def.fields; filters = @()
        enabled = $true; sort = 1
    } | Out-Null
    Write-Host "  mapping created ($($def.fields.Count) fields)" -ForegroundColor Green

    # 3. test (auth + probe)
    $test = Invoke-Api POST "/api/integration/connectors/$cid/test"
    $tcolor = if ($test.ok) { 'Green' } else { 'Red' }
    Write-Host "  test: ok=$($test.ok) stage=$($test.stage) :: $($test.message)" -ForegroundColor $tcolor
    if ($test.sample_fields) {
        Write-Host "        sample fields: $((@($test.sample_fields) | Select-Object -First 12) -join ', ')" -ForegroundColor DarkGray
    }

    # 4. run (only if probe succeeded)
    $run = $null
    if ($test.ok) {
        try {
            $run = Invoke-Api POST "/api/integration/connectors/$cid/run"
            Write-Host "  run: status=$($run.status) seen=$($run.seen) created=$($run.created) updated=$($run.updated) skipped=$($run.skipped) errors=$($run.errors)" -ForegroundColor Green
            if ($run.detail.errors) { $run.detail.errors | Select-Object -First 5 | ForEach-Object { Write-Host "        ! $_" -ForegroundColor Yellow } }
        } catch {
            Write-Host "  run FAILED: $($_.Exception.Message)" -ForegroundColor Red
        }
    } else {
        Write-Host "  run skipped (test failed - fix the Graph permission first)" -ForegroundColor Yellow
    }

    $summary += [pscustomobject]@{
        Source  = $def.source_object
        TestOK  = $test.ok
        Stage   = $test.stage
        Status  = if ($run) { $run.status } else { 'not run' }
        Seen    = if ($run) { $run.seen } else { 0 }
        Created = if ($run) { $run.created } else { 0 }
        Updated = if ($run) { $run.updated } else { 0 }
        Errors  = if ($run) { $run.errors } else { 0 }
    }
}

Write-Head 'Summary'
$summary | Format-Table -AutoSize

Write-Host "`nReminder: rotate this client secret in Entra once testing is done." -ForegroundColor Yellow
