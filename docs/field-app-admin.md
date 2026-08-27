# Field App — Admin & Ops Guide

How to configure and operate the ITAM mobile field app and the server-side
settings it depends on.

## 1. Network & CORS

The field app (Flutter web) runs in the browser and calls the API
directly, so its origin must be allowed by CORS.

- Default allowed origins (`backend/internal/config/config.go`):
  `http://localhost:5608` (web app) and `http://localhost:5609` (field app dev).
- Override with the `CORS_ORIGINS` env var (comma-separated) when serving the
  field app from another host/port (e.g. a phone hitting your LAN IP).

Service URL the app asks for on first launch:

| Setting | Local default | Android emulator |
|---------|---------------|------------------|
| API base URL | `http://localhost:5607` | `http://10.0.2.2:5607` |

## 2. Permissions field users need

Tiles are gated by the same RBAC permissions as the web app (resolved from
`/api/me`). Grant the relevant ones to your field role:

| Capability | Permission |
|------------|------------|
| View / scan / search / recent / near-me | `asset.read` |
| Browse stores & locations | `hierarchy.read` |
| Change lifecycle state | `asset.transition` |
| Assign / return / move (check in/out, audit relocate) | `asset.assign` |
| Create assets | `asset.write` |
| Stock receive | `procurement.manage` |

`/api/field-config` requires only that the user is authenticated (no special
permission), so the "Near me" radius is always readable.

## 3. "Near me" radius

Controls how far the app's GPS will reach before it gives up and says
"nothing nearby".

- **Setting key:** `field.nearby_radius_km` (number, kilometres).
- **Default:** 25 km (used when the setting is unset).
- **Where to change it:** web app → **Admin → Maps → Mobile field app →
  "Near me" radius (km)** (requires `settings.manage`).
- **How it's read:** the app calls `GET /api/field-config`, which returns
  `{ "nearby_radius_km": <number> }`. Changes apply on the next use — no app
  rebuild or redeploy.

## 4. Map basemap

The default basemap for the web Locations/Assets maps (the field app's store map
uses OpenStreetMap tiles directly).

- **Setting key:** `map.basemap`.
- **Where to change it:** **Admin → Maps → Default basemap**.

## 5. Location coordinates (what makes "Near me" and the maps work)

A location only appears in "Near me" and on the maps once it has **coordinates**.

- **Stores:** set the **Store location** (free text) and optional
  **Latitude/Longitude** when creating a store (web app → **Stores → New store**),
  or edit them later on the same page.
- **Sites/buildings:** set coordinates in **Locations** (web app), editing a
  site/building and entering latitude & longitude.

Coordinates are stored as a PostGIS point; the app finds the nearest one by
great-circle distance.

## 6. Subtree rollup

"Devices in a location" rolls up the whole subtree when requested:

- API: `GET /api/assets?location_id=<id>&subtree=true` returns the location's
  assets **plus** everything in its descendants (floors, rooms, departments…).
- The field app uses this for **Near me**, **Stores → devices**, and **Audit**, so
  auditing a store also catches its departments' stock.

## 7. Operating the dev web build

From `mobile/`, `./docker-web.ps1` (or `.sh`) serves the app at
`http://localhost:5609` using the `ghcr.io/cirruslabs/flutter:stable` image. It
runs `flutter pub get` then `flutter run -d web-server`. Stop it with
`docker rm -f itam-flutter`.

After changing Flutter source, restart that container to recompile (the headless
`web-server` device doesn't hot-reload without a TTY).
