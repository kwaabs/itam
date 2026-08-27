// Lightweight, tolerant models mirroring the ITAM API JSON. We only model the
// fields the field app needs and keep the raw map around for anything extra.

String _s(dynamic v) => v == null ? '' : v.toString();
int? _i(dynamic v) => v == null ? null : (v is int ? v : int.tryParse(v.toString()));
double? _d(dynamic v) => v == null ? null : (v is num ? v.toDouble() : double.tryParse(v.toString()));

class Me {
  final String userId;
  final String email;
  final bool isSuperuser;
  final List<String> permissions;

  Me({required this.userId, required this.email, required this.isSuperuser, required this.permissions});

  factory Me.fromJson(Map<String, dynamic> j) => Me(
        userId: _s(j['user_id']),
        email: _s(j['email']),
        isSuperuser: j['is_superuser'] == true,
        permissions: ((j['permissions'] as List?) ?? const []).map((e) => e.toString()).toList(),
      );

  bool can(String perm) => isSuperuser || permissions.contains(perm);
}

class AssetType {
  final int id;
  final String key;
  final String name;
  final String path;
  final bool isAbstract;
  final int? lifecycleId;

  AssetType({
    required this.id,
    required this.key,
    required this.name,
    required this.path,
    required this.isAbstract,
    this.lifecycleId,
  });

  factory AssetType.fromJson(Map<String, dynamic> j) => AssetType(
        id: _i(j['id']) ?? 0,
        key: _s(j['key']),
        name: _s(j['name']),
        path: _s(j['path']),
        isAbstract: j['is_abstract'] == true,
        lifecycleId: _i(j['lifecycle_id']),
      );

  /// Concrete types under hardware.peripheral.* (monitors, keyboards, etc.).
  bool get isPeripheral => path.startsWith('hardware.peripheral.');
}

class DataType {
  final int id;
  final String key;
  final String label;
  final String valueKind;

  DataType({required this.id, required this.key, required this.label, required this.valueKind});

  factory DataType.fromJson(Map<String, dynamic> j) => DataType(
        id: _i(j['id']) ?? 0,
        key: _s(j['key']),
        label: _s(j['label']),
        valueKind: _s(j['value_kind']),
      );
}

class Unit {
  final int id;
  final String key;
  final String label;
  final String symbol;

  Unit({required this.id, required this.key, required this.label, required this.symbol});

  factory Unit.fromJson(Map<String, dynamic> j) => Unit(
        id: _i(j['id']) ?? 0,
        key: _s(j['key']),
        label: _s(j['label']),
        symbol: _s(j['symbol']),
      );
}

class FieldDefinition {
  final int id;
  final String key;
  final String label;
  final bool required;
  final String helpText;
  final List<dynamic> enumOptions;
  final DataType? dataType;
  final Unit? unit;

  FieldDefinition({
    required this.id,
    required this.key,
    required this.label,
    required this.required,
    required this.helpText,
    required this.enumOptions,
    this.dataType,
    this.unit,
  });

  factory FieldDefinition.fromJson(Map<String, dynamic> j) => FieldDefinition(
        id: _i(j['id']) ?? 0,
        key: _s(j['key']),
        label: _s(j['label']),
        required: j['required'] == true,
        helpText: _s(j['help_text']),
        enumOptions: (j['enum_options'] as List?) ?? const [],
        dataType: j['data_type'] is Map ? DataType.fromJson((j['data_type'] as Map).cast<String, dynamic>()) : null,
        unit: j['unit'] is Map ? Unit.fromJson((j['unit'] as Map).cast<String, dynamic>()) : null,
      );

  String get valueKind => dataType?.valueKind.isNotEmpty == true ? dataType!.valueKind : 'text';
}

class LocationNode {
  final String id;
  final String name;
  final String kind;
  final String path;
  final double? latitude;
  final double? longitude;

  LocationNode({
    required this.id,
    required this.name,
    required this.kind,
    required this.path,
    this.latitude,
    this.longitude,
  });

  factory LocationNode.fromJson(Map<String, dynamic> j) => LocationNode(
        id: _s(j['id']),
        name: _s(j['name']),
        kind: _s(j['kind']),
        path: _s(j['path']),
        latitude: _d(j['latitude']),
        longitude: _d(j['longitude']),
      );

  bool get hasCoords => latitude != null && longitude != null;

  /// Depth from the materialized ltree-style path, for indented display.
  int get depth {
    final p = path;
    if (p.isEmpty) return 0;
    return p.split('.').length - 1;
  }
}

class OrgUnit {
  final String id;
  final String name;
  final String kind;
  final String path;
  final String? defaultLocationId;

  OrgUnit({required this.id, required this.name, required this.kind, required this.path, this.defaultLocationId});

  factory OrgUnit.fromJson(Map<String, dynamic> j) => OrgUnit(
        id: _s(j['id']),
        name: _s(j['name']),
        kind: _s(j['kind']),
        path: _s(j['path']),
        defaultLocationId: j['default_location_id']?.toString(),
      );

  int get depth => path.isEmpty ? 0 : path.split('.').length - 1;
}

class Person {
  final String id;
  final String firstName;
  final String lastName;
  final String email;
  final String employeeNo;
  final String? orgUnitId;

  Person({required this.id, required this.firstName, required this.lastName, required this.email, required this.employeeNo, this.orgUnitId});

  factory Person.fromJson(Map<String, dynamic> j) => Person(
        id: _s(j['id']),
        firstName: _s(j['first_name']),
        lastName: _s(j['last_name']),
        email: _s(j['email']),
        employeeNo: _s(j['employee_no']),
        orgUnitId: j['org_unit_id']?.toString(),
      );

  String get fullName => '$firstName $lastName'.trim();
}

class Transition {
  final String key;
  final String label;
  final bool allowed;

  Transition({required this.key, required this.label, required this.allowed});

  factory Transition.fromJson(Map<String, dynamic> j) => Transition(
        key: _s(j['key']),
        label: _s(j['label']),
        allowed: j['allowed'] != false,
      );
}

class AssetEvent {
  final String kind;
  final String summary;
  final String occurredAt;

  AssetEvent({required this.kind, required this.summary, required this.occurredAt});

  factory AssetEvent.fromJson(Map<String, dynamic> j) => AssetEvent(
        kind: _s(j['kind']),
        summary: _s(j['summary']),
        occurredAt: _s(j['occurred_at']),
      );
}

class AssignmentRow {
  final String kind;
  final String assignedAt;
  final String? returnedAt;
  final String? acknowledgedAt;
  final String reason;
  final String holderName;
  final String locationLabel;

  AssignmentRow({
    required this.kind,
    required this.assignedAt,
    this.returnedAt,
    this.acknowledgedAt,
    required this.reason,
    required this.holderName,
    required this.locationLabel,
  });

  factory AssignmentRow.fromJson(Map<String, dynamic> j) {
    final hp = j['holder_person'];
    final ho = j['holder_org_unit'];
    String holder = '';
    if (hp is Map) {
      holder = '${hp['first_name'] ?? ''} ${hp['last_name'] ?? ''}'.trim();
    } else if (ho is Map) {
      holder = _s(ho['name']);
    }
    final from = j['from_location'] is Map ? _s((j['from_location'] as Map)['name']) : '';
    final to = j['to_location'] is Map ? _s((j['to_location'] as Map)['name']) : '';
    final loc = _s(j['kind']) == 'transfer' ? '$from → $to' : (to.isNotEmpty ? to : from);
    return AssignmentRow(
      kind: _s(j['kind']),
      assignedAt: _s(j['assigned_at']),
      returnedAt: j['returned_at']?.toString(),
      acknowledgedAt: j['acknowledged_at']?.toString(),
      reason: _s(j['reason']),
      holderName: holder,
      locationLabel: loc,
    );
  }

  bool get isOpenAssign => kind == 'assign' && (returnedAt == null || returnedAt!.isEmpty);

  bool get needsAcknowledge => isOpenAssign && (acknowledgedAt == null || acknowledgedAt!.isEmpty);

  String get ackLabel {
    if (kind != 'assign') return '';
    if (acknowledgedAt != null && acknowledgedAt!.isNotEmpty) {
      return acknowledgedAt!.split('T').first;
    }
    if (isOpenAssign) return 'Pending';
    return '';
  }

  String get eventLabel {
    switch (kind) {
      case 'assign':
        return holderName.isEmpty ? 'Assigned' : 'Assigned to $holderName';
      case 'return':
        return 'Returned';
      case 'transfer':
        return locationLabel.isEmpty ? 'Moved' : 'Moved $locationLabel';
      default:
        return kind;
    }
  }
}

class CustodyAnomalyRow {
  final String id;
  final String assetTag;
  final String name;
  final String stateName;

  CustodyAnomalyRow({
    required this.id,
    required this.assetTag,
    required this.name,
    required this.stateName,
  });

  factory CustodyAnomalyRow.fromJson(Map<String, dynamic> j) => CustodyAnomalyRow(
        id: _s(j['id']),
        assetTag: _s(j['asset_tag']),
        name: _s(j['name']),
        stateName: _s(j['state_name']),
      );
}

class CustodyReport {
  final int assigned;
  final int available;
  final List<CustodyAnomalyRow> unacknowledged;
  final List<CustodyAnomalyRow> assignedInStock;
  final List<CustodyAnomalyRow> notInStockAfterReturn;

  CustodyReport({
    required this.assigned,
    required this.available,
    required this.unacknowledged,
    required this.assignedInStock,
    required this.notInStockAfterReturn,
  });

  factory CustodyReport.fromJson(Map<String, dynamic> j) => CustodyReport(
        assigned: _i(j['assigned']) ?? 0,
        available: _i(j['available']) ?? 0,
        unacknowledged: ((j['unacknowledged'] as List?) ?? const [])
            .map((e) => CustodyAnomalyRow.fromJson((e as Map).cast<String, dynamic>()))
            .toList(),
        assignedInStock: ((j['assigned_in_stock'] as List?) ?? const [])
            .map((e) => CustodyAnomalyRow.fromJson((e as Map).cast<String, dynamic>()))
            .toList(),
        notInStockAfterReturn: ((j['not_in_stock_after_return'] as List?) ?? const [])
            .map((e) => CustodyAnomalyRow.fromJson((e as Map).cast<String, dynamic>()))
            .toList(),
      );

  int get exceptionCount => unacknowledged.length + assignedInStock.length + notInStockAfterReturn.length;
}

bool custodyNeedsAcknowledge(List<AssignmentRow> rows) => rows.any((r) => r.needsAcknowledge);

class Asset {
  final String id;
  final String assetTag;
  final String name;
  final String serial;
  final String vendor;
  final String notes;
  final int assetTypeId;
  final String? locationId;
  final String? ownerOrgUnitId;
  final String? assignedPersonId;
  final double? purchaseCost;
  final String? warrantyExpiry;
  final Map<String, dynamic> attributes;
  final Map<String, dynamic> raw;

  Asset({
    required this.id,
    required this.assetTag,
    required this.name,
    required this.serial,
    required this.vendor,
    required this.notes,
    required this.assetTypeId,
    required this.locationId,
    this.ownerOrgUnitId,
    required this.assignedPersonId,
    required this.purchaseCost,
    required this.warrantyExpiry,
    required this.attributes,
    required this.raw,
  });

  factory Asset.fromJson(Map<String, dynamic> j) => Asset(
        id: _s(j['id']),
        assetTag: _s(j['asset_tag']),
        name: _s(j['name']),
        serial: _s(j['serial']),
        vendor: _s(j['vendor']),
        notes: _s(j['notes']),
        assetTypeId: _i(j['asset_type_id']) ?? 0,
        locationId: j['location_id']?.toString(),
        ownerOrgUnitId: j['owner_org_unit_id']?.toString(),
        assignedPersonId: j['assigned_person_id']?.toString(),
        purchaseCost: _d(j['purchase_cost']),
        warrantyExpiry: j['warranty_expiry']?.toString(),
        attributes: (j['attributes'] as Map?)?.cast<String, dynamic>() ?? {},
        raw: j,
      );

  String get typeName {
    final t = raw['asset_type'];
    if (t is Map && t['name'] != null) return t['name'].toString();
    return '';
  }

  String get stateName {
    final s = raw['current_state'];
    if (s is Map && s['label'] != null) return s['label'].toString();
    return '';
  }

  String get stateColor {
    final s = raw['current_state'];
    if (s is Map && s['color'] != null) return s['color'].toString();
    return '';
  }

  String get locationName {
    final l = raw['location'];
    if (l is Map && l['name'] != null) return l['name'].toString();
    return '';
  }

  String get ownerOrgUnitName {
    final o = raw['owner_org_unit'];
    if (o is Map && o['name'] != null) return o['name'].toString();
    return '';
  }

  String get assignedToName {
    final a = raw['assigned_to'];
    if (a is Map) {
      final n = '${a['first_name'] ?? ''} ${a['last_name'] ?? ''}'.trim();
      if (n.isNotEmpty) return n;
    }
    return '';
  }

  String get ingestMatchLabel {
    final m = attributes['ingest_match_method']?.toString();
    final c = attributes['ingest_match_confidence']?.toString();
    if (m == null || m.isEmpty) return '';
    return 'Discovery: $m${c != null && c.isNotEmpty ? ' ($c)' : ''}';
  }
}

class Store {
  final String id;
  final String name;
  final String address;
  final String path;
  final double? latitude;
  final double? longitude;
  final int inStockCount;
  final int departmentCount;

  Store({
    required this.id,
    required this.name,
    required this.address,
    required this.path,
    required this.latitude,
    required this.longitude,
    required this.inStockCount,
    required this.departmentCount,
  });

  factory Store.fromJson(Map<String, dynamic> j) => Store(
        id: _s(j['id']),
        name: _s(j['name']),
        address: _s(j['address']),
        path: _s(j['path']),
        latitude: _d(j['latitude']),
        longitude: _d(j['longitude']),
        inStockCount: _i(j['in_stock_count']) ?? 0,
        departmentCount: _i(j['department_count']) ?? 0,
      );

  bool get hasCoords => latitude != null && longitude != null;

  /// As a LocationNode, for reuse with pickers / nearest helpers.
  LocationNode get asNode =>
      LocationNode(id: id, name: name, kind: 'store', path: path, latitude: latitude, longitude: longitude);
}

class PurchaseOrder {
  final String id;
  final String poNumber;
  final String status;
  final Map<String, dynamic> raw;

  PurchaseOrder({required this.id, required this.poNumber, required this.status, required this.raw});

  factory PurchaseOrder.fromJson(Map<String, dynamic> j) => PurchaseOrder(
        id: _s(j['id']),
        poNumber: _s(j['po_number']),
        status: _s(j['status']),
        raw: j,
      );
}
