# ITAM Field — User Guide

A quick, task-by-task guide for using the field app day to day.

## Getting started

1. Open the app. First time only, enter the **API URL** and **Auth URL** your IT
   team gave you, then **Save**.
2. **Sign in** with your work email and password.
3. You land on the home screen: **"What do you want to do?"** Pick a tile.
   Greyed-out tiles mean your account doesn't have permission for that action.

The menu (⋮, top-right) lets you see who you're signed in as, change the
connection settings, or sign out.

---

## Look up an asset

**By scanning** — tap **Scan asset**, point the camera at the QR code or barcode.
- No camera (or on web)? Type the asset tag in the box at the bottom and tap **Find**.

**By searching** — tap **Find asset**, type any part of the tag, name, or serial,
and tap a result.

**Recently viewed** — tap **Recent** to jump back to assets you opened recently.

Either way you land on the **asset detail** page showing tag, name, type, state,
location, who it's assigned to, vendor, warranty, notes, and type-specific attributes.

---

## Manage peripherals

Tap **Peripherals** to browse monitors, keyboards, mice, printers, and other
end-user gear. Pull down to refresh. Tap **Add** to create a new peripheral (the
type picker is limited to peripheral types and shows fields like screen size or
connection type). Use the search icon to find a specific item within peripherals.

---

## Quick-update an asset

On the asset detail page, use **Quick actions**:

- **Edit** — change name, serial, vendor, notes, location, and type-specific fields.
- **Delete** — permanently remove the asset (requires delete permission).
- **Change state** — move it through its lifecycle (e.g. In stock → In use). Add an
  optional note.
- **Assign** — hand it to a person (search and pick them).
- **Return** — take it back from the current holder.
- **Move location** — search and pick a new location.

Changes save immediately and the page refreshes.

---

## Check something in or out

Tap **Check in / out** for the fastest custody flow:

1. **Scan / find asset**.
2. If it's available → **Check out (assign)** → pick the person.
3. If it's already out → **Check in (return)**.
4. Tap **Scan another** to keep going.

---

## Find what's around you ("Near me")

Tap **Near me**. The app reads your GPS, finds the **nearest store or site that
has coordinates** (within the radius your admin set), and lists every device
there — including sub-locations like rooms and departments.

- Pull down or tap **Refresh** to re-locate.
- "No location within X km" means you're not close enough to a mapped site, or no
  nearby location has coordinates yet.

---

## Do a stocktake / audit

Tap **Audit / stocktake**:

1. **Choose location** (search and pick) **or** tap **Use my location** to audit
   the nearest mapped site automatically.
2. The app loads everything **expected** there.
3. Tap **Scan** and scan each item you physically see.
   - Items expected here flip to **Confirmed present** (green).
   - Items that belong elsewhere show as **Misplaced** (amber) — tap **Move here**
     to relocate them on the spot.
   - You can also tap an item under **Not yet seen** to confirm it manually.
4. The header keeps a running tally: confirmed / missing / misplaced.

---

## Receive stock against a PO

Tap **Stock receive**:

1. Pick an open **purchase order**.
2. For each line, enter the **quantity received** and the **asset tags**.
3. Submit to check the items into stock.

---

## Browse stores

Tap **Stores** to see your stock-holding locations, with a map of the ones that
have coordinates. Tap a store (in the list or a map pin) to see everything it
holds, including its departments.

---

## Create an asset on the spot

Tap **New asset**, fill in tag, name, type, serial, vendor, location, and notes.
When the type has custom fields (monitors, printers, etc.) those appear below.
Save — you're taken straight to the new asset's detail page.

---

## Tips & troubleshooting

- **Camera/GPS prompts**: allow them when asked — scanning and "Near me" need them.
- **"No permission" tiles**: ask your IT admin to grant the relevant role.
- **Wrong server / can't sign in**: open the ⋮ menu → **Connection settings** and
  double-check the API and Auth URLs.
- **Nothing nearby**: a location only appears in "Near me"/the stores map once it
  has coordinates set (your admin can add these on stores and sites).
