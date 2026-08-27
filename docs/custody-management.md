# Custody management

Track who has an asset, where it moved, and audit history across web and mobile.

## Workflows

### Assign / return (web)

**Overview → Custody** card on the asset page:

1. Pick a person → **Assign**
2. Optionally pick **Also move to location** (updates physical location in one step)
3. **Return to stock** when the device comes back

### Assign / return (mobile)

**Check in / out** or asset detail → **Assign**:

1. Pick the person
2. Prompt: keep current location or choose a new one

### History

| View | What it shows |
|------|----------------|
| **Custody tab** (web) | Table of assign/return/transfer rows with dates, locations, reasons |
| **Timeline tab** (web) | Full event ledger (edits, costs, state changes, etc.) |
| **History** (mobile) | Expandable custody + timeline on asset detail |

### People → assets held

**People** page → **N held** button on each person → list of assigned assets (click through to detail).

### Reports

**Reports → Custody overview**:

- Checked out vs available counts
- Top holders (who has the most gear)
- **Needs attention** — returned but still not in stock (lifecycle mismatch)

## API

```
POST /api/assets/{id}/assign
  { "holder_person_id": "...", "to_location_id": "..." }  // location optional

GET /api/assets/{id}/history     // assignments with person/location names
GET /api/assets/{id}/timeline    // unified event ledger
GET /api/assets?assigned_person_id={uuid}
GET /api/reports/custody
```

## Tips

- Use **location on assign** when handing equipment to someone at a different site/desk.
- Use **org unit** (owner) for budget/region roll-ups; use **assignee** for custody.
- Run **Reports → Custody** periodically to catch assets returned but still marked in-use.
