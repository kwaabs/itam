# Serve the Flutter web app via the cirruslabs Flutter Docker image.
# Run from the mobile/ directory:  ./docker-web.ps1
# Then open http://localhost:5609 in your browser.
#
# The app runs in YOUR browser, so it calls the ITAM API/GoTrue on localhost
# directly. Make sure the API allows http://localhost:5609 (it does by default)
# and that the API has been restarted since that CORS change.

docker rm -f itam-flutter 2>$null | Out-Null

docker run --rm --name itam-flutter `
  -p 5609:5609 `
  -v "${PWD}:/app" `
  -w /app `
  ghcr.io/cirruslabs/flutter:stable `
  bash -lc "git config --global --add safe.directory '*'; flutter pub get && flutter run -d web-server --web-hostname 0.0.0.0 --web-port 5609"
