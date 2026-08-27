# Linkage and data quality (pragmatic improvements)

These changes reduce common gaps between discovery sources, org structure, and physical location — without trying to be perfect.

## Org unit default location

Each org unit can have a **default location** (Org Units page). When you assign an asset to a person in that unit and leave the location blank, the API sets the asset's location to the unit's default.

Use this for branch desks, helpdesk floors, or warehouse stock rooms tied to a team.

## Location aliases

External systems often use different site names than ITAM location keys. **Location aliases** (Locations page) map those labels to a real location:

- OpManager map or probe names
- Legacy site codes from CSV imports
- Any string that appears in `location_key`, `network_map`, or `probe` fields during ingest

Aliases are matched case-insensitively. Add them before or after the first sync — the next ingest run will resolve them.

## Connector sync behavior

Pull connectors (OpManager, Intune/Graph) ship with **`update_location_on_sync: true`** by default. On each sync, if the source provides a resolvable location, the asset's `location_id` is updated.

Turn this off per connector in Integrations if you prefer manual location control.

## Cross-source deduplication

When ingesting a device, ITAM tries to match an existing asset in order:

1. **Serial number** (exact)
2. **IP address** in asset attributes (when serial is missing)
3. **Unique name** match (only when exactly one asset shares that name)

This helps OpManager and Intune land on the same row without manual merges. It is best-effort — duplicate serials still need cleanup (see data quality report).

## Assignee → org unit

If an ingested asset has an assignee but no owner org unit, sync can set **owner org unit** from the person's org when that field is empty.

## Data quality report

**Reports → Data quality** summarizes:

- Assets missing location, org unit, or serial
- Duplicate serial numbers (top 20)
- A clickable sample of assets to fix

Use this as a weekly triage list rather than a hard gate.

## Related docs

- [Org grouping](./org-grouping.md) — region / district / division / unit roll-ups
- [Custody management](./custody-management.md) — assign, return, location on checkout
- [OpManager ingestion](./ops-manager-ingestion.md) — map/probe fields and location keys
