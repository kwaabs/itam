import 'package:dio/dio.dart';

import '../core/models.dart';

class ApiException implements Exception {
  final String message;
  final int? status;
  final Map<String, String> fieldErrors;
  ApiException(this.message, [this.status, this.fieldErrors = const {}]);
  @override
  String toString() => message;
}

/// Thin wrapper over the ITAM REST API for the actions the field app performs.
class Repository {
  final Dio dio;
  Repository(this.dio);

  Never _fail(Response resp) {
    final data = resp.data;
    final fieldErrors = <String, String>{};
    if (data is Map && data['fields'] is Map) {
      for (final e in (data['fields'] as Map).entries) {
        fieldErrors[e.key.toString()] = e.value.toString();
      }
    }
    final msg = (data is Map && data['error'] != null)
        ? data['error'].toString()
        : 'Request failed (${resp.statusCode})';
    throw ApiException(msg, resp.statusCode, fieldErrors);
  }

  bool _isOk(Response r) => r.statusCode != null && r.statusCode! >= 200 && r.statusCode! < 300;

  // ---- assets --------------------------------------------------------------

  Future<Asset> getAsset(String id) async {
    final r = await dio.get('/api/assets/$id');
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<List<Asset>> searchAssets(String q, {String? type, int pageSize = 25}) async {
    return listAssets(q: q, type: type, pageSize: pageSize);
  }

  Future<List<Asset>> listAssets({String? q, String? type, String? ownerOrgUnitId, int pageSize = 50}) async {
    final params = <String, dynamic>{'page_size': pageSize};
    if (q != null && q.isNotEmpty) params['q'] = q;
    if (type != null && type.isNotEmpty) params['type'] = type;
    if (ownerOrgUnitId != null && ownerOrgUnitId.isNotEmpty) {
      params['owner_org_unit_id'] = ownerOrgUnitId;
      params['subtree'] = 'true';
    }
    final r = await dio.get('/api/assets', queryParameters: params);
    if (!_isOk(r)) _fail(r);
    final items = (r.data is Map ? (r.data as Map)['items'] : r.data) as List? ?? [];
    return items.map((e) => Asset.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  /// Assets in a location. With [subtree] (default) the API rolls up the whole
  /// location subtree — the node itself plus floors/rooms/departments beneath it.
  Future<List<Asset>> assetsByLocation(String locationId, {bool subtree = true}) async {
    final r = await dio.get('/api/assets', queryParameters: {
      'location_id': locationId,
      if (subtree) 'subtree': 'true',
      'page_size': 500,
    });
    if (!_isOk(r)) _fail(r);
    final items = (r.data is Map ? (r.data as Map)['items'] : r.data) as List? ?? [];
    return items.map((e) => Asset.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<Asset> createAsset(Map<String, dynamic> body) async {
    final r = await dio.post('/api/assets', data: body);
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<Asset> updateAsset(String id, Map<String, dynamic> body) async {
    final r = await dio.put('/api/assets/$id', data: body);
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<void> deleteAsset(String id) async {
    final r = await dio.delete('/api/assets/$id');
    if (!_isOk(r)) _fail(r);
  }

  // ---- lifecycle / custody / movement --------------------------------------

  Future<List<Transition>> transitions(String assetId) async {
    final r = await dio.get('/api/assets/$assetId/transitions');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => Transition.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<Asset> transition(String assetId, String key, {String note = ''}) async {
    final r = await dio.post('/api/assets/$assetId/transition', data: {'transition_key': key, 'note': note});
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<Asset> assign(String assetId, String personId, {String reason = '', String? toLocationId}) async {
    final data = <String, dynamic>{'holder_person_id': personId, 'reason': reason};
    if (toLocationId != null && toLocationId.isNotEmpty) data['to_location_id'] = toLocationId;
    final r = await dio.post('/api/assets/$assetId/assign', data: data);
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<Asset> returnAsset(String assetId, {String reason = ''}) async {
    final r = await dio.post('/api/assets/$assetId/return', data: {'reason': reason});
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<Asset> transfer(String assetId, String toLocationId, {String reason = ''}) async {
    final r = await dio.post('/api/assets/$assetId/transfer', data: {'to_location_id': toLocationId, 'reason': reason});
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<Asset> acknowledge(String assetId) async {
    final r = await dio.post('/api/assets/$assetId/acknowledge', data: const {});
    if (!_isOk(r)) _fail(r);
    return Asset.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<CustodyReport> custodyReport() async {
    final r = await dio.get('/api/reports/custody');
    if (!_isOk(r)) _fail(r);
    return CustodyReport.fromJson((r.data as Map).cast<String, dynamic>());
  }

  // ---- lookups -------------------------------------------------------------

  Future<List<LocationNode>> locations() async {
    final r = await dio.get('/api/locations');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => LocationNode.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  /// Mobile field-app runtime config (metadata-controlled on the server).
  /// Falls back to a sane default if the endpoint is unavailable.
  Future<double> nearbyRadiusKm() async {
    try {
      final r = await dio.get('/api/field-config');
      if (_isOk(r) && r.data is Map) {
        final v = (r.data as Map)['nearby_radius_km'];
        if (v is num) return v.toDouble();
      }
    } catch (_) {}
    return 25.0;
  }

  Future<List<Store>> stores() async {
    final r = await dio.get('/api/stores');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => Store.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<List<Person>> people() async {
    final r = await dio.get('/api/people');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => Person.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<List<OrgUnit>> orgUnits() async {
    final r = await dio.get('/api/org-units');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => OrgUnit.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<List<AssetType>> assetTypes() async {
    final r = await dio.get('/api/metadata/asset-types');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list
        .map((e) => AssetType.fromJson((e as Map).cast<String, dynamic>()))
        .where((t) => !t.isAbstract)
        .toList();
  }

  Future<List<FieldDefinition>> typeFields(int typeId) async {
    final r = await dio.get('/api/metadata/asset-types/$typeId/fields');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => FieldDefinition.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<List<AssetEvent>> timeline(String assetId, {int limit = 50}) async {
    final r = await dio.get('/api/assets/$assetId/timeline', queryParameters: {'limit': limit});
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => AssetEvent.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<List<AssignmentRow>> custodyHistory(String assetId) async {
    final r = await dio.get('/api/assets/$assetId/history');
    if (!_isOk(r)) _fail(r);
    final data = r.data as Map;
    final list = (data['assignments'] as List?) ?? [];
    return list.map((e) => AssignmentRow.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  // ---- procurement (stock receiving) ---------------------------------------

  Future<List<PurchaseOrder>> purchaseOrders() async {
    final r = await dio.get('/api/procurement/purchase-orders');
    if (!_isOk(r)) _fail(r);
    final list = (r.data as List?) ?? [];
    return list.map((e) => PurchaseOrder.fromJson((e as Map).cast<String, dynamic>())).toList();
  }

  Future<Map<String, dynamic>> purchaseOrder(String id) async {
    final r = await dio.get('/api/procurement/purchase-orders/$id');
    if (!_isOk(r)) _fail(r);
    return (r.data as Map).cast<String, dynamic>();
  }

  Future<void> receivePO(String id, Map<String, dynamic> body) async {
    final r = await dio.post('/api/procurement/purchase-orders/$id/receive', data: body);
    if (!_isOk(r)) _fail(r);
  }
}
