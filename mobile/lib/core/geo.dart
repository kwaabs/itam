import 'dart:math' as math;

import 'package:geolocator/geolocator.dart';

import 'models.dart';

/// Result of resolving the user's position to the closest known location.
class NearestLocation {
  final LocationNode location;
  final double distanceMeters;
  NearestLocation(this.location, this.distanceMeters);

  String get distanceLabel {
    if (distanceMeters < 1000) return '${distanceMeters.round()} m away';
    return '${(distanceMeters / 1000).toStringAsFixed(distanceMeters < 10000 ? 1 : 0)} km away';
  }
}

class GeoException implements Exception {
  final String message;
  GeoException(this.message);
  @override
  String toString() => message;
}

/// Thin wrapper around geolocator: checks service + permission state, then
/// returns the device position. Throws [GeoException] with a friendly message.
class Geo {
  static Future<Position> currentPosition() async {
    if (!await Geolocator.isLocationServiceEnabled()) {
      throw GeoException('Location services are turned off. Enable them and try again.');
    }
    var perm = await Geolocator.checkPermission();
    if (perm == LocationPermission.denied) {
      perm = await Geolocator.requestPermission();
    }
    if (perm == LocationPermission.denied) {
      throw GeoException('Location permission denied.');
    }
    if (perm == LocationPermission.deniedForever) {
      throw GeoException('Location permission permanently denied. Allow it in settings.');
    }
    return Geolocator.getCurrentPosition(
      locationSettings: const LocationSettings(accuracy: LocationAccuracy.high),
    );
  }

  /// Closest location (by great-circle distance) that actually has coordinates.
  /// When [maxMeters] is set, locations beyond that radius are ignored (returns
  /// null if nothing falls within range).
  static NearestLocation? nearest(List<LocationNode> locations, double lat, double lng, {double? maxMeters}) {
    NearestLocation? best;
    for (final l in locations) {
      if (!l.hasCoords) continue;
      final d = _haversine(lat, lng, l.latitude!, l.longitude!);
      if (best == null || d < best.distanceMeters) {
        best = NearestLocation(l, d);
      }
    }
    if (best != null && maxMeters != null && best.distanceMeters > maxMeters) {
      return null;
    }
    return best;
  }

  static double _haversine(double lat1, double lon1, double lat2, double lon2) {
    const earth = 6371000.0; // metres
    final dLat = _rad(lat2 - lat1);
    final dLon = _rad(lon2 - lon1);
    final a = math.sin(dLat / 2) * math.sin(dLat / 2) +
        math.cos(_rad(lat1)) * math.cos(_rad(lat2)) * math.sin(dLon / 2) * math.sin(dLon / 2);
    return earth * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a));
  }

  static double _rad(double deg) => deg * math.pi / 180.0;
}
