// Unit tests for the GPS "nearest location" logic that powers Near me / audit.

import 'package:flutter_test/flutter_test.dart';
import 'package:itam_field/core/geo.dart';
import 'package:itam_field/core/models.dart';

LocationNode _loc(String name, double? lat, double? lng) =>
    LocationNode(id: name, name: name, kind: 'site', path: name, latitude: lat, longitude: lng);

void main() {
  final london = _loc('London', 51.5072, -0.1276);
  final frankfurt = _loc('Frankfurt', 50.1109, 8.6821);
  final noCoords = _loc('NoCoords', null, null);

  group('Geo.nearest', () {
    test('picks the closest located node', () {
      final n = Geo.nearest([frankfurt, london, noCoords], 51.50, -0.12);
      expect(n, isNotNull);
      expect(n!.location.name, 'London');
    });

    test('ignores nodes without coordinates', () {
      final n = Geo.nearest([noCoords], 51.5, -0.12);
      expect(n, isNull);
    });

    test('returns null when nothing falls within the radius', () {
      // London→Frankfurt is ~650 km; a 100 km cap excludes it.
      final n = Geo.nearest([frankfurt], 51.50, -0.12, maxMeters: 100000);
      expect(n, isNull);
    });

    test('includes a node within the radius', () {
      final n = Geo.nearest([london], 51.50, -0.12, maxMeters: 100000);
      expect(n, isNotNull);
      expect(n!.distanceMeters, lessThan(100000));
    });
  });
}
