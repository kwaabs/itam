# Azure Arc (Hybrid Compute) ingestion

ITAM pulls **Azure Arc–enabled servers** from Azure Resource Manager and
reconciles them as **server** assets (by default).

## API

```
GET https://management.azure.com/subscriptions/{subscriptionId}/providers/Microsoft.HybridCompute/machines?api-version=2024-07-10
```

Paged results use ARM `nextLink` (not Graph `@odata.nextLink`). The pull engine
follows both.

## Authentication

Use the same Entra **app registration** pattern as Microsoft Graph connectors:

- **Grant type:** client credentials
- **Token endpoint:** `https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token`
- **Scope:** `https://management.azure.com/.default`

### RBAC

Assign the service principal **Reader** (or a custom role with
`Microsoft.HybridCompute/machines/read`) on the subscription or resource group
that contains Arc machines.

## UI setup

1. **Integrations → + Azure Arc**
2. Enter tenant ID, client ID, client secret, and **subscription ID**
3. Default asset type: **server**
4. **Create & map**, then **Test connection** and **Run now**

Identity for matching is the ARM resource `id`. Fallbacks: `properties.vmId`,
`properties.vmUuid`, machine `name`, and `properties.machineFqdn`.

Mapped attributes include OS SKU, Arc agent status, FQDN, IP/MAC (first NIC),
hardware detected properties, and license status.

## Multiple subscriptions

Create one connector per subscription (each with its own subscription ID in the
object URL).
