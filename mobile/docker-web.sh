#!/usr/bin/env bash
# Serve the Flutter web app via the cirruslabs Flutter Docker image.
# Run from the mobile/ directory:  ./docker-web.sh
# Then open http://localhost:5609 in your browser.
#
# The app runs in YOUR browser, so it calls the ITAM API/GoTrue on localhost
# directly. Make sure the API allows http://localhost:5609 (default) and has
# been restarted since that CORS change.
set -e

docker rm -f itam-flutter >/dev/null 2>&1 || true

docker run --rm --name itam-flutter \
  -p 5609:5609 \
  -v "$(pwd):/app" \
  -w /app \
  ghcr.io/cirruslabs/flutter:stable \
  bash -lc "git config --global --add safe.directory '*'; flutter pub get && flutter run -d web-server --web-hostname 0.0.0.0 --web-port 5609"
