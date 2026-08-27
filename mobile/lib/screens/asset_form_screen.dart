import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'asset_detail_screen.dart';
import 'pickers.dart';
import 'widgets/dynamic_fields.dart';

/// Shared create / edit form for assets (including peripheral type-specific fields).
class AssetFormScreen extends StatefulWidget {
  /// When set, the form edits an existing asset; otherwise it creates one.
  final String? assetId;
  final bool peripheralsOnly;
  final String? initialTypeKey;

  const AssetFormScreen({
    super.key,
    this.assetId,
    this.peripheralsOnly = false,
    this.initialTypeKey,
  });

  bool get isEdit => assetId != null;

  @override
  State<AssetFormScreen> createState() => _AssetFormScreenState();
}

class _AssetFormScreenState extends State<AssetFormScreen> {
  late final Repository _repo;
  final _tag = TextEditingController();
  final _name = TextEditingController();
  final _serial = TextEditingController();
  final _vendor = TextEditingController();
  final _notes = TextEditingController();

  List<AssetType> _types = [];
  List<FieldDefinition> _fields = [];
  AssetType? _type;
  LocationNode? _location;
  OrgUnit? _ownerOrgUnit;
  Map<String, dynamic> _attributes = {};
  Map<String, String> _fieldErrors = {};
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
    _init();
  }

  Future<void> _init() async {
    try {
      final types = await _repo.assetTypes();
      final filtered = widget.peripheralsOnly ? types.where((t) => t.isPeripheral).toList() : types;
      setState(() => _types = filtered);

      if (widget.isEdit) {
        final a = await _repo.getAsset(widget.assetId!);
        _tag.text = a.assetTag;
        _name.text = a.name;
        _serial.text = a.serial;
        _vendor.text = a.vendor;
        _notes.text = a.notes;
        _attributes = Map<String, dynamic>.from(a.attributes);
        _type = types.where((t) => t.id == a.assetTypeId).cast<AssetType?>().firstWhere(
              (t) => t != null,
              orElse: () => null,
            );
        if (_type != null) {
          _fields = await _repo.typeFields(_type!.id);
        }
        if (a.locationId != null) {
          final locs = await _repo.locations();
          _location = locs.cast<LocationNode?>().firstWhere((l) => l?.id == a.locationId, orElse: () => null);
        }
        if (a.ownerOrgUnitId != null) {
          final ous = await _repo.orgUnits();
          _ownerOrgUnit = ous.cast<OrgUnit?>().firstWhere((o) => o?.id == a.ownerOrgUnitId, orElse: () => null);
        }
      } else {
        final pre = widget.initialTypeKey;
        if (pre != null) {
          _type = filtered.where((t) => t.key == pre).cast<AssetType?>().firstWhere((t) => t != null, orElse: () => null);
          if (_type != null) _fields = await _repo.typeFields(_type!.id);
        }
      }
      setState(() => _loading = false);
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  Future<void> _onTypeChanged(AssetType? t) async {
    setState(() {
      _type = t;
      _fields = [];
      _attributes = {};
      _fieldErrors = {};
    });
    if (t == null) return;
    try {
      final fields = await _repo.typeFields(t.id);
      if (mounted) setState(() => _fields = fields);
    } on ApiException catch (e) {
      _snack(e.message);
    }
  }

  @override
  void dispose() {
    _tag.dispose();
    _name.dispose();
    _serial.dispose();
    _vendor.dispose();
    _notes.dispose();
    super.dispose();
  }

  Future<void> _pickOrgUnit() async {
    try {
      final ous = await _repo.orgUnits();
      if (!mounted) return;
      final ou = await pickOrgUnit(context, ous);
      if (ou != null) setState(() => _ownerOrgUnit = ou);
    } on ApiException catch (e) {
      _snack(e.message);
    }
  }

  void _clearOrgUnit() => setState(() => _ownerOrgUnit = null);

  Future<void> _pickLoc() async {
    try {
      final locs = await _repo.locations();
      if (!mounted) return;
      final loc = await pickLocation(context, locs);
      if (loc != null) setState(() => _location = loc);
    } on ApiException catch (e) {
      _snack(e.message);
    }
  }

  Future<void> _save() async {
    if (!widget.isEdit && (_tag.text.trim().isEmpty || _type == null)) {
      _snack('Tag and type are required');
      return;
    }
    if (_name.text.trim().isEmpty) {
      _snack('Name is required');
      return;
    }
    setState(() {
      _busy = true;
      _fieldErrors = {};
      _error = null;
    });
    try {
      if (widget.isEdit) {
        final a = await _repo.updateAsset(widget.assetId!, {
          'name': _name.text.trim(),
          'serial': _serial.text.trim(),
          'vendor': _vendor.text.trim(),
          'notes': _notes.text.trim(),
          'location_id': _location?.id,
          'owner_org_unit_id': _ownerOrgUnit?.id,
          'attributes': _attributes,
        });
        if (!mounted) return;
        Navigator.pop(context, a);
      } else {
        final a = await _repo.createAsset({
          'asset_tag': _tag.text.trim(),
          'name': _name.text.trim(),
          'asset_type_id': _type!.id,
          'serial': _serial.text.trim(),
          'vendor': _vendor.text.trim(),
          'notes': _notes.text.trim(),
          'location_id': _location?.id,
          'owner_org_unit_id': _ownerOrgUnit?.id,
          'attributes': _attributes,
        });
        if (!mounted) return;
        Navigator.pushReplacement(context, MaterialPageRoute(builder: (_) => AssetDetailScreen(assetId: a.id)));
      }
    } on ApiException catch (e) {
      setState(() {
        _fieldErrors = e.fieldErrors;
        _error = e.message;
      });
      if (e.fieldErrors.isEmpty) _snack(e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _snack(String m) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  @override
  Widget build(BuildContext context) {
    final title = widget.isEdit ? 'Edit asset' : (widget.peripheralsOnly ? 'New peripheral' : 'New asset');
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null && !widget.isEdit && _types.isEmpty
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    if (_error != null && _fieldErrors.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 12),
                        child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))),
                      ),
                    if (widget.isEdit)
                      TextField(
                        controller: _tag,
                        readOnly: true,
                        decoration: const InputDecoration(labelText: 'Asset tag'),
                      )
                    else
                      TextField(controller: _tag, decoration: const InputDecoration(labelText: 'Asset tag *')),
                    const SizedBox(height: 14),
                    TextField(controller: _name, decoration: const InputDecoration(labelText: 'Name *')),
                    const SizedBox(height: 14),
                    if (widget.isEdit)
                      TextFormField(
                        initialValue: _type?.name ?? '',
                        readOnly: true,
                        decoration: const InputDecoration(labelText: 'Type'),
                      )
                    else
                      DropdownButtonFormField<AssetType>(
                        value: _type,
                        decoration: const InputDecoration(labelText: 'Type *'),
                        items: _types.map((t) => DropdownMenuItem(value: t, child: Text(t.name))).toList(),
                        onChanged: _busy ? null : _onTypeChanged,
                      ),
                    const SizedBox(height: 14),
                    TextField(controller: _serial, decoration: const InputDecoration(labelText: 'Serial')),
                    const SizedBox(height: 14),
                    TextField(controller: _vendor, decoration: const InputDecoration(labelText: 'Vendor')),
                    const SizedBox(height: 14),
                    TextField(
                      controller: _notes,
                      decoration: const InputDecoration(labelText: 'Notes'),
                      maxLines: 2,
                    ),
                    const SizedBox(height: 14),
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      shape: RoundedRectangleBorder(
                        side: const BorderSide(color: Colors.white24),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      title: Text(_location?.name ?? 'Location (optional)'),
                      leading: const Icon(Icons.place),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: _busy ? null : _pickLoc,
                    ),
                    const SizedBox(height: 14),
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      shape: RoundedRectangleBorder(
                        side: const BorderSide(color: Colors.white24),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      title: Text(_ownerOrgUnit?.name ?? 'Org unit (optional)'),
                      subtitle: _ownerOrgUnit?.kind.isNotEmpty == true ? Text(_ownerOrgUnit!.kind) : null,
                      leading: const Icon(Icons.account_tree),
                      trailing: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (_ownerOrgUnit != null)
                            IconButton(icon: const Icon(Icons.clear, size: 20), onPressed: _busy ? null : _clearOrgUnit),
                          const Icon(Icons.chevron_right),
                        ],
                      ),
                      onTap: _busy ? null : _pickOrgUnit,
                    ),
                    const SizedBox(height: 20),
                    DynamicFieldsForm(
                      fields: _fields,
                      values: _attributes,
                      errors: _fieldErrors,
                      onChanged: (v) => setState(() => _attributes = v),
                    ),
                    const SizedBox(height: 24),
                    FilledButton(
                      onPressed: _busy ? null : _save,
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 12),
                        child: _busy
                            ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                            : Text(widget.isEdit ? 'Save changes' : 'Create asset'),
                      ),
                    ),
                  ],
                ),
    );
  }
}
