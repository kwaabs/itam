param(
	[Parameter(Mandatory = $true, Position = 0)]
	[ValidateSet('up', 'down', 'status', 'reset', 'create')]
	[string]$Action,
	[Parameter(Position = 1)]
	[string]$Name = ''
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Migrations = Join-Path $Root 'backend\migrations'
$Image = if ($env:GOOSE_IMAGE) { $env:GOOSE_IMAGE } else { 'itam-migrate' }
$Network = if ($env:GOOSE_NETWORK) { $env:GOOSE_NETWORK } else { 'itam_default' }
$DbUrl = if ($env:GOOSE_DB_URL) { $env:GOOSE_DB_URL } else { 'postgres://supabase_admin:postgres@db:5432/postgres?sslmode=disable' }

$mount = "${Migrations}:/migrations"
$ro = $Action -ne 'create'

$args = @(
	'run', '--rm', '--network', $Network,
	'-v', $(if ($ro) { "${mount}:ro" } else { $mount }),
	'--entrypoint', 'goose',
	$Image,
	'-dir', '/migrations'
)

if ($Action -eq 'create') {
	if (-not $Name) { throw 'Usage: goose-docker.ps1 create -Name add_widgets' }
	$args += 'create', $Name, 'sql'
} else {
	$args += 'postgres', $DbUrl, $Action
}

& docker @args
