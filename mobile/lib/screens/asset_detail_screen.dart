import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../core/recent.dart';
import '../data/repository.dart';
import 'edit_asset_screen.dart';
import 'pickers.dart';
import 'widgets/dynamic_fields.dart';

class AssetDetailScreen extends StatefulWidget {
  final String assetId;
  const AssetDetailScreen({super.key, required this.assetId});

  @override
  State<AssetDetailScreen> createState() => _AssetDetailScreenState();
}

class _AssetDetailScreenState extends State<AssetDetailScreen> {
  late final Repository _repo;
  late final Me? _me;

  Asset? _asset;
  List<Transition> _transitions = [];
  List<FieldDefinition> _fields = [];
  List<AssetEvent> _timeline = [];
  List<AssignmentRow> _custody = [];
  bool _showHistory = false;
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final app = context.read<AppController>();
    _repo = Repository(app.dio);
    _me = app.me;
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final a = await _repo.getAsset(widget.assetId);
      List<Transition> tr = [];
      List<FieldDefinition> fields = [];
      if (_me?.can('asset.transition') ?? false) {
        try {
          tr = await _repo.transitions(a.id);
        } catch (_) {}
      }
      try {
        fields = await _repo.typeFields(a.assetTypeId);
      } catch (_) {}
      List<AssetEvent> timeline = [];
      List<AssignmentRow> custody = [];
      if (_me?.can('asset.read') ?? false) {
        try {
          timeline = await _repo.timeline(a.id);
        } catch (_) {}
        try {
          custody = await _repo.custodyHistory(a.id);
        } catch (_) {}
      }
      setState(() {
        _asset = a;
        _transitions = tr;
        _fields = fields;
        _timeline = timeline;
        _custody = custody;
        _loading = false;
      });
      Recent.add(a.id, a.assetTag, a.name);
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  Future<void> _run(Future<Asset> Function() action, {String ok = 'Saved'}) async {
    setState(() => _busy = true);
    try {
      final updated = await action();
      List<Transition> tr = _transitions;
      List<AssignmentRow> custody = _custody;
      if (_me?.can('asset.transition') ?? false) {
        try {
          tr = await _repo.transitions(updated.id);
        } catch (_) {}
      }
      if (_me?.can('asset.read') ?? false) {
        try {
          custody = await _repo.custodyHistory(updated.id);
        } catch (_) {}
      }
      setState(() {
        _asset = updated;
        _transitions = tr;
        _custody = custody;
      });
      _snack(ok);
    } on ApiException catch (e) {
      _snack(e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _snack(String m) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  // ---- actions -------------------------------------------------------------

  Future<void> _changeState() async {
    final allowed = _transitions.where((t) => t.allowed).toList();
    if (allowed.isEmpty) {
      _snack('No state changes available');
      return;
    }
    final t = await showModalBottomSheet<Transition>(
      context: context,
      builder: (_) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: [
            const ListTile(title: Text('Change state', style: TextStyle(fontWeight: FontWeight.bold))),
            for (final tr in allowed)
              ListTile(leading: const Icon(Icons.arrow_forward), title: Text(tr.label), onTap: () => Navigator.pop(context, tr)),
          ],
        ),
      ),
    );
    if (t == null) return;
    final note = await _askText('Note (optional)');
    if (note == null) return; // cancelled
    await _run(() => _repo.transition(_asset!.id, t.key, note: note));
  }

  Future<void> _assign() async {
    final people = await _loadPeople();
    if (people == null) return;
    if (!mounted) return;
    final person = await pickPerson(context, people);
    if (person == null) return;
    String? toLocationId;
    final locs = await _loadLocations();
    if (locs != null && mounted) {
      final hint = await assignDefaultLocationHint(_repo, person);
      toLocationId = await pickAssignLocation(context, locs, defaultHint: hint);
    }
    await _run(() => _repo.assign(_asset!.id, person.id, toLocationId: toLocationId));
  }

  Future<void> _return() async {
    await _run(() => _repo.returnAsset(_asset!.id));
  }

  Future<void> _acknowledge() async {
    await _run(() => _repo.acknowledge(_asset!.id), ok: 'Receipt acknowledged');
  }

  Future<void> _move() async {
    final locs = await _loadLocations();
    if (locs == null) return;
    if (!mounted) return;
    final loc = await pickLocation(context, locs);
    if (loc == null) return;
    await _run(() => _repo.transfer(_asset!.id, loc.id));
  }

  Future<void> _edit() async {
    final updated = await Navigator.push<Asset>(
      context,
      MaterialPageRoute(builder: (_) => EditAssetScreen(assetId: widget.assetId)),
    );
    if (updated != null) {
      setState(() => _asset = updated);
      try {
        _fields = await _repo.typeFields(updated.assetTypeId);
      } catch (_) {}
    } else {
      await _load();
    }
  }

  Future<void> _delete() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Delete asset?'),
        content: Text('Permanently delete ${_asset!.assetTag}? This cannot be undone.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: const Color(0xFFef4444)),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    setState(() => _busy = true);
    try {
      await _repo.deleteAsset(_asset!.id);
      if (!mounted) return;
      Navigator.pop(context);
    } on ApiException catch (e) {
      _snack(e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<List<Person>?> _loadPeople() async {
    try {
      return await _repo.people();
    } on ApiException catch (e) {
      _snack(e.message);
      return null;
    }
  }

  Future<List<LocationNode>?> _loadLocations() async {
    try {
      return await _repo.locations();
    } on ApiException catch (e) {
      _snack(e.message);
      return null;
    }
  }

  Future<String?> _askText(String label) async {
    final c = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (_) => AlertDialog(
        title: Text(label),
        content: TextField(controller: c, autofocus: true),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(context, c.text.trim()), child: const Text('OK')),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final a = _asset;
    final canAssign = _me?.can('asset.assign') ?? false;
    final canWrite = _me?.can('asset.write') ?? false;
    final canDelete = _me?.can('asset.delete') ?? false;
    final needsAck = custodyNeedsAcknowledge(_custody);
    return Scaffold(
      appBar: AppBar(title: Text(a?.assetTag ?? 'Asset')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView(
                    padding: const EdgeInsets.all(16),
                    children: [
                      Text(a!.name, style: const TextStyle(fontSize: 22, fontWeight: FontWeight.bold)),
                      const SizedBox(height: 4),
                      Wrap(spacing: 8, runSpacing: 8, children: [
                        if (a.typeName.isNotEmpty) _chip(a.typeName),
                        if (a.stateName.isNotEmpty) _chip(a.stateName, color: a.stateColor),
                        if (a.ingestMatchLabel.isNotEmpty) _chip(a.ingestMatchLabel, color: '#6366f1'),
                      ]),
                      if (needsAck && canAssign) ...[
                        const SizedBox(height: 12),
                        Material(
                          color: const Color(0xFF422006),
                          borderRadius: BorderRadius.circular(12),
                          child: Padding(
                            padding: const EdgeInsets.all(12),
                            child: Row(
                              children: [
                                const Icon(Icons.pending_actions, color: Color(0xFFfbbf24), size: 20),
                                const SizedBox(width: 10),
                                const Expanded(
                                  child: Text(
                                    'Awaiting custody acknowledge — confirm receipt of this asset.',
                                    style: TextStyle(fontSize: 13),
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ],
                      const SizedBox(height: 16),
                      _row('Tag', a.assetTag),
                      _row('Serial', a.serial),
                      _row('Location', a.locationName.isEmpty ? '—' : a.locationName),
                      _row('Org unit', a.ownerOrgUnitName.isEmpty ? '—' : a.ownerOrgUnitName),
                      _row('Assigned to', a.assignedToName.isEmpty ? '—' : a.assignedToName),
                      _row('Vendor', a.vendor.isEmpty ? '—' : a.vendor),
                      if (a.notes.isNotEmpty) _row('Notes', a.notes),
                      if (a.warrantyExpiry != null && a.warrantyExpiry!.isNotEmpty)
                        _row('Warranty', a.warrantyExpiry!.split('T').first),
                      const SizedBox(height: 8),
                      AttributeRows(attributes: a.attributes, fields: _fields),
                      if (_custody.isNotEmpty || _timeline.isNotEmpty) ...[
                        const Divider(height: 32),
                        InkWell(
                          onTap: () => setState(() => _showHistory = !_showHistory),
                          child: Row(
                            children: [
                              Text('History', style: Theme.of(context).textTheme.titleMedium),
                              const Spacer(),
                              Icon(_showHistory ? Icons.expand_less : Icons.expand_more),
                            ],
                          ),
                        ),
                        if (_showHistory) ...[
                          const SizedBox(height: 8),
                          if (_custody.isNotEmpty) ...[
                            const Text('Custody', style: TextStyle(fontWeight: FontWeight.w600)),
                            const SizedBox(height: 6),
                            for (final c in _custody.take(15))
                              Padding(
                                padding: const EdgeInsets.symmetric(vertical: 6),
                                child: Row(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    SizedBox(
                                      width: 110,
                                      child: Text(
                                        c.assignedAt.split('T').first,
                                        style: const TextStyle(color: Colors.white54, fontSize: 12),
                                      ),
                                    ),
                                    Expanded(child: Text(c.eventLabel)),
                                    if (c.kind == 'assign')
                                      SizedBox(
                                        width: 72,
                                        child: Text(
                                          c.ackLabel,
                                          textAlign: TextAlign.end,
                                          style: TextStyle(
                                            fontSize: 12,
                                            color: c.needsAcknowledge ? const Color(0xFFfbbf24) : Colors.white54,
                                          ),
                                        ),
                                      ),
                                  ],
                                ),
                              ),
                          ],
                          if (_timeline.isNotEmpty) ...[
                            const SizedBox(height: 12),
                            const Text('Timeline', style: TextStyle(fontWeight: FontWeight.w600)),
                            const SizedBox(height: 6),
                            for (final ev in _timeline.take(20))
                              Padding(
                                padding: const EdgeInsets.symmetric(vertical: 6),
                                child: Row(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    SizedBox(
                                      width: 110,
                                      child: Text(
                                        ev.occurredAt.split('T').first,
                                        style: const TextStyle(color: Colors.white54, fontSize: 12),
                                      ),
                                    ),
                                    Expanded(child: Text(ev.summary.isEmpty ? ev.kind : ev.summary)),
                                  ],
                                ),
                              ),
                          ],
                        ],
                      ],
                      const Divider(height: 32),
                      Text('Quick actions', style: Theme.of(context).textTheme.titleMedium),
                      const SizedBox(height: 12),
                      Wrap(
                        spacing: 10,
                        runSpacing: 10,
                        children: [
                          if (canWrite) _actionBtn(Icons.edit, 'Edit', _busy ? null : _edit),
                          if (canDelete) _actionBtn(Icons.delete_outline, 'Delete', _busy ? null : _delete),
                          if (_me?.can('asset.transition') ?? false)
                            _actionBtn(Icons.swap_horiz, 'Change state', _busy ? null : _changeState),
                          if (canAssign && a.assignedPersonId == null)
                            _actionBtn(Icons.person_add, 'Assign', _busy ? null : _assign),
                          if (canAssign && a.assignedPersonId != null)
                            _actionBtn(Icons.assignment_return, 'Return', _busy ? null : _return),
                          if (canAssign && needsAck)
                            _actionBtn(Icons.check_circle_outline, 'Acknowledge', _busy ? null : _acknowledge),
                          if (canAssign) _actionBtn(Icons.move_up, 'Move location', _busy ? null : _move),
                        ],
                      ),
                      if (_busy) const Padding(padding: EdgeInsets.only(top: 20), child: LinearProgressIndicator()),
                    ],
                  ),
                ),
    );
  }

  Widget _row(String k, String v) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 6),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(width: 110, child: Text(k, style: const TextStyle(color: Colors.white54))),
            Expanded(child: Text(v)),
          ],
        ),
      );

  Widget _chip(String label, {String color = ''}) {
    Color? c;
    if (color.startsWith('#') && (color.length == 7)) {
      c = Color(int.parse('FF${color.substring(1)}', radix: 16));
    }
    return Chip(
      label: Text(label),
      side: c != null ? BorderSide(color: c) : null,
      backgroundColor: c == null ? null : c.withOpacity(0.15),
    );
  }

  Widget _actionBtn(IconData icon, String label, VoidCallback? onTap) =>
      FilledButton.tonalIcon(onPressed: onTap, icon: Icon(icon), label: Text(label));
}
