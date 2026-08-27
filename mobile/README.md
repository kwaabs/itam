# ITAM Field (Flutter)

A lightweight companion app for **manual field work** against the ITAM backend:
scan an asset, view it, make quick changes — plus search, "near me", check
in/out, stocktake, stock receiving, stores, and creating assets on the spot.

It talks to the **existing** ITAM API and authenticates against GoTrue with
email/password. The only backend additions it relies on are the `subtree`
asset filter and the `/api/field-config` endpoint (both already shipped).

## Features

| Home tile | What it does |
|-----------|--------------|
| **Scan asset** | Scan a QR (deep link → asset id) or Code128 barcode (asset tag) → open the asset. A manual "enter tag" box is always available (web / no camera). |
| **Find asset** | Free-text search by tag / name / serial. |
| **Peripherals** | Browse monitors, keyboards, mice, etc.; search within peripherals; quick add. |
| **Near me** | Uses GPS to find the nearest store/site **within the configured radius**, then lists every device there (subtree rollup). |
| **Check in / out** | Scan/find an asset → one-tap assign (out) or return (in). |
| **Audit / stocktake** | Pick a location (or "Use my location"), scan everything present, see what's missing/misplaced, relocate with one tap. |
| **Stores** | List of stock-holding stores + a map of the located ones; tap a store → its stock. |
| **Stock receive** | Receive quantities + asset tags against an open purchase order. |
| **New asset** | Create an asset: tag, name, type, serial, vendor, location, notes, and type-specific fields (e.g. screen size for monitors). |
| **Recent** | Recently opened assets, kept on-device (no scan needed). |

From an asset's detail screen you can also **edit** (name, serial, vendor, notes, location, attributes) and **delete** when your role allows it.

All actions respect the same RBAC permissions as the web app (read from `/api/me`);
tiles you lack permission for are dimmed.

## Run it (web) via Docker — recommended

No local Flutter SDK needed. From the `mobile/` folder:

```powershell
./docker-web.ps1      # Windows PowerShell
```
```bash
./docker-web.sh       # macOS / Linux
```

This uses `ghcr.io/cirruslabs/flutter:stable` to run `flutter pub get` and serve
the app headlessly (`-d web-server`) on **http://localhost:5609**. The app runs
in *your* browser, so it calls the ITAM API/GoTrue on `localhost` directly. The
API already allows `http://localhost:5609` for CORS.

> The container only compiles + serves the bundle; the camera, GPS and map all
> run in your browser (allowed on `localhost`).

### Run it (web) with a local SDK instead

```bash
cd mobile
flutter pub get
flutter run -d chrome --web-port 5609
```

## First launch

The setup screen asks for:

- **API base URL** — default `http://localhost:5607`
- **GoTrue (auth) URL** — default `http://localhost:5606`

Then sign in with your ITAM credentials (e.g. the bootstrap admin).

## Running on Android

The Android platform is generated under `android/`. Build/run with:

```bash
cd mobile
flutter pub get
flutter run -d <device-or-emulator>
```

Host addresses:
- **Emulator**: use `http://10.0.2.2:5607` / `:5606` (maps to your PC's localhost).
- **Physical phone**: use your PC's LAN IP; ensure the phone can reach it.

The manifest already requests the permissions the app needs
(`INTERNET`, `CAMERA`, `ACCESS_FINE_LOCATION`, `ACCESS_COARSE_LOCATION`).

## Dependencies

`provider` (state), `dio` (HTTP + auth interceptor), `mobile_scanner` (camera),
`flutter_secure_storage` (tokens), `shared_preferences` (config + recent),
`geolocator` (GPS), `flutter_map` + `latlong2` (stores map), `intl`.

## Configuration knobs (server-side)

- **"Near me" radius** — metadata setting `field.nearby_radius_km` (default 25),
  editable in the web app under **Admin → Maps → Mobile field app**. The app reads
  it from `GET /api/field-config`.

## Project layout

```
lib/
  core/      config, app state (auth + Dio), models, geo (GPS+nearest), recent
  data/      repository (all API calls)
  screens/   setup, login, home, scan, asset detail, search, peripherals, nearby, checkout,
             audit, stores, stock, create/edit asset form, recent (+ shared widgets/list)
docker-web.ps1 / docker-web.sh   one-command web serve via Docker
```

See [`USER_GUIDE.md`](USER_GUIDE.md) for step-by-step field instructions.
