<#
.SYNOPSIS
  Create + test a ManageEngine OpManager pull connector against a running ITAM stack.

.DESCRIPTION
  Logs into the ITAM API as the bootstrap admin, then:
    1. (re)creates a pull connector  (direction=pull, API-key auth)
    2. creates the field mapping       (OpManager JSON -> asset)
    3. calls /test                     (fetch probe)
    4. calls /run                      (full pull + reconcile)
    5. reads back the sync run as proof data landed

.EXAMPLE
  ./scripts/Setup-OpsManagerConnector.ps1 `
    -BaseUrl "https://opmanager.example.com:8061/api/json/device/listDevices" `
    -ApiKey  "your-api-key-here"
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $ApiKey,

    [Parameter(Mandatory = $true)]
    [string] $BaseUrl,

    [string] $ApiUrl    = 'http://localhost:5607',
    [string] $AdminEmail    = 'admin@itam.local',
    [string] $AdminPassword = 'admin12345',

    [string] $ConnectorKey  = 'ops_manager_devices',
    [string] $ConnectorName = 'ManageEngine OpManager devices',
    [string] $DefaultAssetType = 'server',
    [string] $DiscoveredState  = 'in_use',
    [int] $ScheduleMinutes = 60
)

$ErrorActionPreference = 'Stop'

$mapping = @{
    source_object   = 'device'
    target_entity   = 'asset'
    identity        = 'id'
    match_fallbacks = @('ipaddress', 'deviceName', 'moid')
    type_resolution = @{
        by      = 'category'
        default = $DefaultAssetType
        map     = @{
            Desktop = 'desktop'
            Server  = 'server'
            Switch  = 'switch'
            Router  = 'router'
            Laptop  = 'laptop'
        }
    }
    fields = @(
        @{ source = 'displayName';       target = 'name' }
        @{ source = 'id';                target = 'external_id' }
        @{ source = 'ipaddress';         target = 'attr.ip' }
        @{ source = 'type';              target = 'attr.device_type' }
        @{ source = 'vendorName';        target = 'attr.manufacturer' }
        @{ source = 'category';          target = 'attr.category' }
        @{ source = 'statusStr';         target = 'attr.monitor_status' }
        @{ source = 'probeName';         target = 'attr.probe' }
        @{ source = 'probeDisplayName'; target = 'attr.probe_display' }
        @{ source = 'mapName';           target = 'attr.network_map' }
        @{ source = 'addedTime';         target = 'attr.added_at'; transform = 'parse_date' }
        @{ source = 'isSNMP';            target = 'attr.snmp' }
        @{ source = 'interfaceCount';    target = 'attr.interface_count' }
        @{ source = 'deviceName';        target = 'attr.opmanager_device_name' }
        @{ source = 'moid';              target = 'attr.moid' }
        @{ source = 'statusNum';         target = 'attr.status_code' }
    )
    filters = @()
    enabled = $true
    sort    = 1
}

$config = @{
    schedule_seconds   = $ScheduleMinutes * 60
    default_asset_type = $DefaultAssetType
    discovered_state   = $DiscoveredState
    auth = @{
        type  = 'api_key'
        in    = 'query'
        param = 'apiKey'
    }
    objects = @(@{
        source_object = 'device'
        url           = $BaseUrl.Trim()
    })
}

function Get-AdminToken {
    $body = @{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json
    $r = Invoke-RestMethod -Method POST -Uri "$ApiUrl/auth/login" `
        -ContentType 'application/json' -Body $body
    return $r.access_token
}

function Invoke-ItamApi {
    param([string]$Method, [string]$Path, [object]$Body = $null)
    $headers = @{ Authorization = "Bearer $script:Token" }
    $uri = "$ApiUrl$Path"
    if ($Body -ne $null) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers `
            -ContentType 'application/json' -Body ($Body | ConvertTo-Json -Depth 20)
    }
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
}

Write-Host "Authenticating as $AdminEmail …"
$script:Token = Get-AdminToken

# Remove existing connector with the same key (idempotent).
$existing = Invoke-ItamApi GET '/api/integration/connectors'
$old = $existing | Where-Object { $_.key -eq $ConnectorKey }
if ($old) {
    Write-Host "Removing existing connector '$ConnectorKey' …"
    Invoke-ItamApi DELETE "/api/integration/connectors/$($old.id)" | Out-Null
}

Write-Host "Creating OpManager pull connector …"
$created = Invoke-ItamApi POST '/api/integration/connectors' @{
    key         = $ConnectorKey
    name        = $ConnectorName
    kind        = 'ops_manager'
    direction   = 'pull'
    config      = $config
    pull_secret = $ApiKey
}
$cid = $created.connector.id
Write-Host "  connector id: $cid"

Write-Host "Creating field mapping …"
Invoke-ItamApi POST "/api/integration/connectors/$cid/mappings" $mapping | Out-Null

Write-Host "Testing connection …"
$test = Invoke-ItamApi POST "/api/integration/connectors/$cid/test"
$test | ConvertTo-Json -Depth 5
if (-not $test.ok) { throw "Test failed: $($test.message)" }

Write-Host "Running full sync …"
$run = Invoke-ItamApi POST "/api/integration/connectors/$cid/run"
Write-Host "  status=$($run.status) seen=$($run.seen) created=$($run.created) updated=$($run.updated) errors=$($run.errors)"

Write-Host "Done."
