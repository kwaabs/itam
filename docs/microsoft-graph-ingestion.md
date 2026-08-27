# Microsoft Graph Ingestion (Intune / Entra ID / Defender)

ITAM can automatically pull device and user data from Microsoft Graph and
reconcile it into the asset register. The pipeline is metadata-driven: field
mappings, schedules, and the Graph credentials all live in the database and
can be changed at any time without redeploying.

---

## Architecture

```
Microsoft Graph API
  ├─ /deviceManagement/managedDevices   (Intune)
  ├─ /users                             (Entra ID — people)
  ├─ /devices                           (Entra ID — registered devices)
  └─ /security/machines                 (Defender for Endpoint)
          │
          ▼  OAuth 2.0 client-credentials (app-only)
    ITAM Pull Loop (every N minutes)
          │
          ▼  raw JSON → mapping engine → reconciler
    core.assets  +  core.people  +  integration.raw_records
```

- Raw records are always stored in `integration.raw_records` before mapping, so you can replay them through updated mappings without hitting Graph again.
- Each source object (Intune, Entra devices, Entra users, Defender) is a separate **connector** with its own mapping, schedule, and sync history.
- Person attribution happens automatically: when a managed device carries a `userPrincipalName`, the reconciler links (or auto-creates) the matching `core.people` record.

---

## App registration requirements

The same Entra app registration used for SSO login can be reused, **or** a
dedicated service-principal can be created. For ingestion, **application
permissions** (not delegated) are required because the pull runs without a
signed-in user.

### Required Graph application permissions

| Source | Permission | Purpose |
|---|---|---|
| Intune devices | `DeviceManagementManagedDevices.Read.All` | Read managed device list |
| Entra users | `User.Read.All` | Read directory users |
| Entra devices | `Device.Read.All` | Read registered devices |
| Defender | `Machine.Read.All` | Read Defender machine inventory |

Grant only the permissions you need for the sources you enable.

### How to grant application permissions

1. Entra portal → **App registrations** → select the app → **API permissions**.
2. **Add a permission → Microsoft Graph → Application permissions**.
3. Add the permissions from the table above.
4. Click **Grant admin consent for \<your org\>** — this step is required for
   application permissions.

---

## Setting up connectors

### Option A — ITAM web UI (recommended)

1. Go to **Integrations → + Intune** (or **+ Defender**).
2. Fill in:
   - **Connector name / key** — e.g. `intune_devices`
   - **Default asset type** — e.g. `laptop` (used when the device OS can't be mapped to a more specific type)
   - **Tenant ID** — your Entra directory ID (e.g. `711b8fff-…`)
   - **Client ID** — your app registration client ID (e.g. `9e08a85c-…`)
   - **Client secret** — the secret value
   - **Scope** — leave as `https://graph.microsoft.com/.default`
   - **Schedule** — how often to sync (default 60 minutes)
3. Click **Create connector**.

The UI automatically creates the standard field mapping for the selected source.
Use **Test connection** to verify the OAuth handshake and confirm the Graph
endpoint returns data before the first scheduled sync.

### Option B — PowerShell script

The `scripts/Setup-GraphConnector.ps1` script creates and immediately tests
all four connectors in one pass. Run it from a PowerShell terminal:

```powershell
cd scripts

.\Setup-GraphConnector.ps1 `
  -ApiUrl        "http://localhost:5607" `
  -AdminEmail    "admin@itam.local" `
  -AdminPassword "admin12345" `
  -TenantId      "711b8fff-9db3-42cc-bc27-7a19ced91f39" `
  -ClientId      "9e08a85c-2d6d-4e39-8fbd-6877fe9ad55c" `
  -ClientSecret  "z1K8Q~dx6Vy~DkM9g4Nly_off0tHK5YNhj0MoaXR" `
  -Sources       @("intune","entra_users","entra_devices","defender") `
  -DefaultAssetType "laptop" `
  -DiscoveredState  "in_use" `
  -AutoCreatePeople $true
```

The script is idempotent: it deletes and recreates any connector with the
same key, so it is safe to re-run when credentials change.

---

## What gets mapped — Intune managed devices

The following fields from `/deviceManagement/managedDevices` are mapped
automatically when you create an Intune connector via the UI or script:

| Graph field | Maps to | Notes |
|---|---|---|
| `id` | identity (match key) | Intune's stable GUID |
| `deviceName` | asset name | |
| `serialNumber` | serial; secondary match key | Empty for VMs |
| `azureADDeviceId` | secondary match key | Fallback when serial is blank |
| `lastSyncDateTime` | `last_seen` | ISO-8601, parsed automatically |
| `userPrincipalName` | `assigned_email` | Links the asset to a person |
| `userDisplayName` | `assigned_name` | Used when auto-creating the person |
| `operatingSystem` | `attr.os` | e.g. `Windows` |
| `osVersion` | `attr.os_version` | e.g. `10.0.26100.7985` |
| `model` | `attr.model` | e.g. `OptiPlex Tower 7020` |
| `manufacturer` | `attr.manufacturer` | e.g. `Dell Inc.` |
| `totalStorageSpaceInBytes` | `attr.storage_total_bytes` | |
| `freeStorageSpaceInBytes` | `attr.storage_free_bytes` | |
| `physicalMemoryInBytes` | `attr.memory_bytes` | Often 0 for co-managed devices |
| `wiFiMacAddress` | `attr.wifi_mac` | |
| `ethernetMacAddress` | `attr.ethernet_mac` | |
| `azureADDeviceId` | `attr.azure_ad_device_id` | |
| `complianceState` | `attr.compliance` | `compliant` / `noncompliant` / `unknown` |
| `managementState` | `attr.management_state` | `managed` / `retirePending` / etc. |
| `managedDeviceOwnerType` | `attr.ownership` | `company` / `personal` / `unknown` |
| `deviceEnrollmentType` | `attr.enrollment_type` | |
| `enrolledDateTime` | `attr.enrolled_at` | ISO-8601 |
| `isEncrypted` | `attr.encrypted` | `true` / `false` |
| `deviceCategoryDisplayName` | `attr.category` | |

Additional fields from the Graph response are not discarded — they are stored
in `integration.raw_records` and can be added to the mapping at any time via
**Integrations → \<connector\> → Field discovery**.

---

## Person attribution

When a managed device has a `userPrincipalName` (primary user's UPN/email):

1. The reconciler searches `core.people` for a matching email (case-insensitive).
2. **If found**: the asset's `assigned_person_id` is updated to that person.
3. **If not found and `auto_create_people` is enabled**: a skeletal person record is created from `userPrincipalName` (email) and `userDisplayName` (name), then attributed to the asset.
4. **If not found and `auto_create_people` is disabled**: the asset is left unattributed until a person with that email exists.

Devices with no `userPrincipalName` (e.g. Defender-only machines, shared
kiosks, VMs) are ingested but left unattributed — no phantom person is created.

`auto_create_people` is enabled by default when creating a connector via the
UI preset or the PowerShell script.

---

## Discovered asset state

Devices seen in Intune are live, in-use assets — they should not land in a
"Procured" or "Ordered" state. The connector sets `discovered_state: in_use`
which tells the reconciler to:

- Place **new** assets directly into the `in_use` (or equivalent) lifecycle state.
- **Promote** existing assets that are still stuck in the initial procurement state to `in_use` on the next sync.

To use a different state key (e.g. `deployed`), update `discovered_state` in
the connector's config via **Integrations → \<connector\> → Edit** or directly
in the `integration.connectors` table.

---

## Triggering a sync

### Automatic

Each connector syncs on its configured schedule (default every 60 minutes).
The pull loop runs inside the API process and checks due connectors every minute.

### Manual

In **Integrations → \<connector\>**, click **Run now**. The sync runs
synchronously and the result (created / updated / skipped / errors) is shown
immediately.

### Reprocess

If you update a mapping (add new fields, fix a transform) and want to apply it
to already-received data without hitting Graph again:

**Integrations → \<connector\> → Reprocess**

This replays every raw record in `integration.raw_records` through the current
mapping. Useful after mapping changes or after a first-time setup to
retrospectively fill in newly mapped fields.

---

## Purging and re-ingesting

To start fresh (e.g. after fixing a mapping issue):

```sql
-- Remove assets created by the Intune connector
-- (replace 'intune_devices' with your connector key)
DELETE FROM core.assets
WHERE id IN (
  SELECT asset_id FROM core.asset_identities WHERE source = 'intune_devices'
);
DELETE FROM core.asset_identities WHERE source = 'intune_devices';
DELETE FROM integration.raw_records
WHERE connector_id = (SELECT id FROM integration.connectors WHERE key = 'intune_devices');
```

Then trigger a **Run now** to re-ingest from Graph.

---

## Monitoring sync health

**Integrations → \<connector\>** shows the sync history with:

- **Created / Updated / Skipped / Errors** counts per run.
- **Status**: `success`, `partial` (some errors), or `error`.
- Error samples for the first 20 failed records.

Errors typically indicate:
- Unknown `asset_type` — update the mapping's type resolution or set a `default_asset_type`.
- No `external_id` — the identity field is blank for that record (e.g. a device with no Intune ID yet).
- Database constraint violations — usually duplicate serials from different sources; resolve by checking `core.asset_identities`.

---

## Credentials rotation

When the client secret expires:

1. Create a new secret in the Entra app registration.
2. In ITAM → **Integrations → \<connector\> → Edit** → update the **Client secret** field → save.

The new secret is used on the next sync (within one minute for the pull loop).
The old connector's cached OAuth token expires within its original TTL (usually 1 hour) and is replaced automatically.
