# Organizational grouping (region, district, division, unit)

Assets and peripherals can be grouped for analysis using the **org unit hierarchy** — not separate columns on each asset.

## How it works

1. **Define your tree** under **Org Units** (`/org-units`):
   - **Region** → **District** → **Division** → **Unit** (kinds are configurable metadata)
2. **Assign each asset** to the owning unit via **Owner (org unit)** on create/edit (web, mobile, CSV import).
3. **Roll-up queries** use the org unit's ltree path: filtering by a region includes every district, division, and unit beneath it.

This is optional — assets without an owner org unit still work; they simply won't appear in org-scoped reports.

## Example hierarchy (demo seed)

After migration `00040_org_unit_kinds.sql`:

```
Organization (region)
└── North Region (region)
    └── North Central District (district)
        └── IT Operations (division)
            └── Helpdesk Unit (unit)
```

Assign a keyboard to **Helpdesk Unit** → it counts toward IT Operations, North Central District, North Region, and Organization in reports.

## Analysis

### Reports page

**Inventory by organization** — pick any org unit (e.g. a region), optionally filter by type (e.g. `keyboard`). Shows total count and breakdown by asset type in that subtree.

### API

```
GET /api/assets?owner_org_unit_id={uuid}&subtree=true&type=keyboard
GET /api/reports/org-inventory?org_unit_id={uuid}&subtree=true&type=keyboard
```

### List filters

- **Assets** and **Peripherals** pages have an **All org units** dropdown (subtree roll-up).

## Import

CSV column `owner_org_unit` uses the org unit **key** (slug), same as locations use `location`.

## RBAC

Org unit scoping in IAM already uses subtree paths — grants on a region apply to assets owned by units under that region.

## Default location per unit

Set **Default location** on an org unit so assign/checkout can infer where assets go when no explicit location is chosen. See [Linkage and data quality](./linkage-improvements.md).

## Mobile

Create/edit asset → **Org unit (optional)** picker. Asset detail shows the assigned unit name.
