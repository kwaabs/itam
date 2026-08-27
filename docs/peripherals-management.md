# Peripherals Management

ITAM tracks keyboards, mice, monitors, projectors, printers, and other
end-user hardware as first-class assets — not as loose attributes on a laptop.
Each peripheral has its own asset tag, lifecycle state, location, assignee,
cost/depreciation, and type-specific fields.

---

## Supported peripheral types

| Type | Key fields |
|---|---|
| Monitor / Screen | Screen size, resolution, panel type, refresh rate, connectivity |
| Keyboard | Layout, connectivity (wired/wireless/bluetooth) |
| Mouse | Connectivity, sensor DPI |
| Printer | Type (laser/inkjet/…), color, speed, duplex, connectivity |
| Scanner | Scanner type, optical DPI, color |
| USB Flash Drive | Capacity, interface, encrypted |
| External Drive | Capacity, interface |
| Webcam | Resolution, built-in mic |
| Headset | Connectivity, microphone |
| Docking Station | Ports, power delivery (W) |
| Projector | Brightness (lumens), resolution |
| Speaker | — |

These types are defined in migration `00035_peripherals.sql`. Admins can add
more subtypes under **Metadata → Asset Types** if needed.

---

## Typical workflows

### 1. Register a new peripheral (direct)

1. Go to **Inventory → Peripherals → + Add peripheral** (or click **+ add** on a type card).
2. Pick the type (monitor, keyboard, etc.) — type-specific fields appear automatically.
3. Enter asset tag, name, serial (if any), location, vendor.
4. Save → asset is created in the lifecycle's initial state (usually **Procured** or **In Stock** depending on how you receive it).

Shortcut URL: `/assets/new?type=monitor` pre-selects the asset type.

### 2. Receive via procurement (recommended for bulk)

1. **Procurement → New PO** — add line items with peripheral asset types (e.g. 10× Keyboard).
2. Receive the PO → assets are generated with tags and land in **In Stock**.
3. View them on **Stock** or **Peripherals** (filter state = In stock).

### 3. Issue to a person

1. Open the asset on **Peripherals** or **Assets**.
2. **Assign** tab → select the person (holder).
3. Run lifecycle transition **Deploy** (from In Stock → Deployed/In Use) if still in stock.

Alternatively from **Stock**: click **Deploy** on the row.

### 4. Issue to a location (e.g. meeting room projector)

1. Set **Location** on the asset (e.g. `Conference Room A`).
2. Transition to **In Use** / **Deployed**.
3. Projectors and room-mounted monitors are often location-assigned rather than person-assigned.

### 5. Return / retire

- **Return** clears the assignee (asset stays at current location).
- Lifecycle transition **Retire** when damaged or end-of-life.

---

## Peripherals page

**Inventory → Peripherals** shows all assets whose type is under the
`hardware.peripheral.*` tree:

- Summary cards per type (monitor, keyboard, …) with counts
- Filter by type, lifecycle state, or search text
- Click a row to open the full asset detail (costs, history, labels, transitions)

---

## Stock room

**Inventory → Stock** lists everything in the **In Stock** lifecycle state,
including peripherals waiting to be issued. Use the type filter to focus on
keyboards, monitors, etc.

Demo data: run `backend/seeds/demo_peripherals.sql` to seed sample stock items
(`STK-MON-*`, `STK-KBD-*`, …).

---

## Mobile field app

Field staff with `asset.write` can register peripherals on-site via **Create asset**
— all non-abstract types including peripherals appear in the type picker.

For faster checkout/issue flows, use **Check in / Check out** on existing assets
after scanning their QR label.

---

## Relationship to computers

Peripherals are separate assets from laptops/desktops. To record that a monitor
is used with a specific laptop:

1. Open the peripheral asset → **Relationships** (if enabled) and link to the computer, or
2. Assign both to the same **person** and **location** (pragmatic approach most teams use).

A dedicated `attached_to` relationship type can be added in metadata if you need
formal parent/child links on the asset graph.

---

## What is NOT auto-discovered

Intune and OpManager ingest **computers and network devices**, not individual
USB keyboards or monitors. Peripherals are registered manually, via procurement
receive, or bulk import — unless you extend ingestion with a custom source.

---

## Tips

- Print QR/barcode labels from **Assets** list → **Print labels** after creating stock.
- Use consistent tag prefixes (`MON-`, `KBD-`, `PRJ-`) for easy searching.
- Set **useful life** per type in metadata for depreciation (already defaulted: 24–36 months).
- Filter **Peripherals** by state **In stock** before a hiring wave to see available kit.
