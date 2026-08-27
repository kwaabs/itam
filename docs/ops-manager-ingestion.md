# ManageEngine OpManager Ingestion

ITAM can pull monitored network devices from **ManageEngine OpManager**
(also referred to as "Ops Manager" in some deployments) via its REST JSON API
and reconcile them into the asset register alongside Intune, Defender, and
other sources.

---

## API overview

OpManager exposes device inventory through a JSON endpoint:

```
GET https://<host>:8061/api/json/device/listDevices?apiKey=<key>
```

The response is a **bare JSON array** of device objects (not OData-wrapped).
Each device includes monitoring metadata such as IP address, category,
vendor, probe, and status.

Example record:

```json
{
  "id": "20000000022",
  "displayName": "10.170.2.55",
  "deviceName": "10.170.2.55.20000000001",
  "ipaddress": "10.170.2.55",
  "type": "Windows 10",
  "vendorName": "Microsoft",
  "category": "Desktop",
  "statusStr": "UnManaged",
  "probeName": "ECGProbe3",
  "mapName": "Desktops_Map.netmap",
  "addedTime": "18 Sep 2024 06:26:58 PM UTC",
  "moid": 20000000022,
  "isSNMP": false,
  "interfaceCount": 0
}
```

---

## Authentication

OpManager uses a static **API key** passed as a query parameter (`apiKey`).
ITAM stores the key in the connector's encrypted `pull_secret` field and
appends it to the URL at fetch time — **do not** include the key in the
configured base URL.

Connector auth config:

```json
{
  "auth": {
    "type": "api_key",
    "in": "query",
    "param": "apiKey"
  }
}
```

For sources that use a header instead, set `"in": "header"` and `"param":
"X-API-Key"` (or the header name your API expects).

---

## Setting up the connector

### Option A — ITAM web UI

1. Go to **Integrations → + OpManager**.
2. Fill in:
   - **Device list URL** — base URL without the API key, e.g.
     `https://opcentral.example.com:8061/api/json/device/listDevices`
   - **API key** — your OpManager REST API key
   - **Default asset type** — fallback when category is unmapped (default: `server`)
   - **Poll every (minutes)** — sync interval (default 60)
3. Click **Create & map**, then **Test connection** and **Run now**.

### Option B — PowerShell script

```powershell
cd scripts

.\Setup-OpsManagerConnector.ps1 `
  -BaseUrl "https://opcentraldddafsvrg.gaedecggh.com:8061/api/json/device/listDevices" `
  -ApiKey  "193asddfbc21e48b5cf9768bdadfa21fdfee085a"
```

The script is idempotent: it replaces any existing connector with the same key,
creates the mapping, tests the connection, and runs a full sync.

---

## Field mapping

| OpManager field | Maps to | Notes |
|---|---|---|
| `id` | identity (match key) | OpManager device ID |
| `displayName` | asset name | Usually the IP or hostname |
| `ipaddress` | `attr.ip` | Also used as match fallback |
| `deviceName` | `attr.opmanager_device_name` | Internal OpManager name |
| `type` | `attr.device_type` | e.g. `Windows 10`, `Unknown` |
| `vendorName` | `attr.manufacturer` | e.g. `Microsoft`, `Dell Inc.` |
| `category` | `attr.category` + **asset type resolution** | See below |
| `statusStr` | `attr.monitor_status` | Clean text: `Clear`, `UnManaged`, etc. |
| `statusNum` | `attr.status_code` | Numeric status code |
| `probeName` | `attr.probe` | Monitoring probe |
| `probeDisplayName` | `attr.probe_display` | |
| `mapName` | `attr.network_map` | OpManager network map file |
| `addedTime` | `attr.added_at` | Parsed to ISO-8601 |
| `isSNMP` | `attr.snmp` | |
| `interfaceCount` | `attr.interface_count` | |
| `moid` | `attr.moid` | Managed object ID |

Match fallbacks (in order): `ipaddress`, `deviceName`, `moid`.

---

## Asset type resolution

OpManager's `category` field drives asset type selection:

| Category | ITAM asset type |
|---|---|
| Desktop | `desktop` |
| Laptop | `laptop` |
| Server | `server` |
| Switch | `switch` |
| Router | `router` |
| *(anything else)* | `server` (default) |

Override the default by choosing a different **Default asset type** when
creating the connector, or edit the mapping's `type_resolution` block.

---

## Discovered asset state

Like other pull sources, OpManager devices enter the `in_use` lifecycle state
(not "procured") because they are live monitored assets.

---

## Reconciling with other sources

OpManager devices are matched independently by their OpManager `id` stored in
`core.asset_identities` under source key `ops_manager_devices`. If the same
physical device also appears in Intune, it will exist as **two assets** unless
you configure shared match keys (e.g. match Intune by serial and OpManager by
IP, then merge manually) or add a custom mapping filter.

A common pattern is to treat OpManager as the **network/monitoring layer**
(IP, status, probe, map) and Intune as the **endpoint management layer**
(user, compliance, encryption). Both can coexist on the same asset if you
align match fallbacks (e.g. Intune `deviceName` ↔ OpManager `displayName`).

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Test returns fetch error / TLS failure | Self-signed certificate on OpManager | Ensure the API host is reachable from the ITAM server; consider a reverse proxy with a valid cert |
| Test returns HTTP 401/403 | Invalid or expired API key | Regenerate the key in OpManager → Settings → REST API |
| Devices created but wrong type | Category not in the type map | Edit mapping `type_resolution.map` or set a better default |
| Duplicate assets vs Intune | Separate identity namespaces | Expected; merge manually or align naming |
| `addedTime` not parsed | Unusual date format | Check raw record in field discovery; add a transform if needed |

---

## Rotating the API key

1. Generate a new API key in OpManager.
2. In ITAM → **Integrations → \<connector\> → Edit** → update **Client secret / pull secret** → save.

The new key is used on the next scheduled sync.
