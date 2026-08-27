import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'pickers.dart';
import 'scan_screen.dart';

/// Streamlined custody flow: scan/find an asset, then check it out (assign to a
/// person) or check it in (return) without digging through the detail page.
class CheckoutScreen extends StatefulWidget {
  const CheckoutScreen({super.key});

  @override
  State<CheckoutScreen> createState() => _CheckoutScreenState();
}

class _CheckoutScreenState extends State<CheckoutScreen> {
  late final Repository _repo;
  late final Me? _me;
  Asset? _asset;
  List<AssignmentRow> _custody = [];
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    final app = context.read<AppController>();
    _repo = Repository(app.dio);
    _me = app.me;
  }

  Future<void> _loadAsset(Asset a) async {
    setState(() => _asset = a);
    try {
      final custody = await _repo.custodyHistory(a.id);
      if (mounted) setState(() => _custody = custody);
    } catch (_) {
      if (mounted) setState(() => _custody = []);
    }
  }

  Future<void> _scan() async {
    final a = await Navigator.push<Asset>(
      context,
      MaterialPageRoute(builder: (_) => const ScanScreen(returnResult: true)),
    );
    if (a != null) await _loadAsset(a);
  }

  Future<void> _checkOut() async {
    List<Person> people;
    List<LocationNode> locs;
    try {
      people = await _repo.people();
      locs = await _repo.locations();
    } on ApiException catch (e) {
      _snack(e.message);
      return;
    }
    if (!mounted) return;
    final person = await pickPerson(context, people);
    if (person == null) return;
    if (!mounted) return;
    final hint = await assignDefaultLocationHint(_repo, person);
    final toLocationId = await pickAssignLocation(context, locs, defaultHint: hint);
    await _run(
      () => _repo.assign(_asset!.id, person.id, toLocationId: toLocationId),
      'Checked out to ${person.fullName.isEmpty ? person.email : person.fullName}',
    );
  }

  Future<void> _checkIn() async {
    await _run(() => _repo.returnAsset(_asset!.id), 'Checked in');
  }

  Future<void> _acknowledge() async {
    await _run(() => _repo.acknowledge(_asset!.id), 'Receipt acknowledged');
  }

  Future<void> _run(Future<Asset> Function() action, String ok) async {
    setState(() => _busy = true);
    try {
      final updated = await action();
      List<AssignmentRow> custody = _custody;
      try {
        custody = await _repo.custodyHistory(updated.id);
      } catch (_) {}
      setState(() {
        _asset = updated;
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

  @override
  Widget build(BuildContext context) {
    final a = _asset;
    final canAssign = _me?.can('asset.assign') ?? false;
    final needsAck = custodyNeedsAcknowledge(_custody);
    return Scaffold(
      appBar: AppBar(title: const Text('Check in / out')),
      body: a == null
          ? Center(
              child: Column(mainAxisSize: MainAxisSize.min, children: [
                const Icon(Icons.swap_horiz, size: 56, color: Colors.white24),
                const SizedBox(height: 16),
                const Text('Scan or find an asset to check in or out'),
                const SizedBox(height: 16),
                FilledButton.icon(onPressed: _scan, icon: const Icon(Icons.qr_code_scanner), label: const Text('Scan / find asset')),
              ]),
            )
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                Text(a.assetTag, style: const TextStyle(fontSize: 22, fontWeight: FontWeight.bold)),
                Text(a.name, style: const TextStyle(color: Colors.white70)),
                if (a.ingestMatchLabel.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: Chip(
                      label: Text(a.ingestMatchLabel, style: const TextStyle(fontSize: 12)),
                      backgroundColor: const Color(0xFF312e81),
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                _row('Status', a.assignedToName.isEmpty ? 'Available' : 'Checked out'),
                _row('Assigned to', a.assignedToName.isEmpty ? '—' : a.assignedToName),
                _row('Location', a.locationName.isEmpty ? '—' : a.locationName),
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
                          const Expanded(child: Text('Awaiting custody acknowledge', style: TextStyle(fontSize: 13))),
                        ],
                      ),
                    ),
                  ),
                ],
                const SizedBox(height: 24),
                if (needsAck && canAssign)
                  FilledButton.icon(
                    onPressed: _busy ? null : _acknowledge,
                    icon: const Icon(Icons.check_circle_outline),
                    label: const Text('Acknowledge receipt'),
                  ),
                if (needsAck && canAssign) const SizedBox(height: 10),
                if (a.assignedPersonId == null)
                  FilledButton.icon(
                    onPressed: _busy ? null : _checkOut,
                    icon: const Icon(Icons.person_add),
                    label: const Text('Check out (assign)'),
                  )
                else
                  FilledButton.icon(
                    onPressed: _busy ? null : _checkIn,
                    icon: const Icon(Icons.assignment_return),
                    label: const Text('Check in (return)'),
                  ),
                const SizedBox(height: 10),
                OutlinedButton.icon(onPressed: _busy ? null : _scan, icon: const Icon(Icons.qr_code_scanner), label: const Text('Scan another')),
                if (_busy) const Padding(padding: EdgeInsets.only(top: 20), child: LinearProgressIndicator()),
              ],
            ),
    );
  }

  Widget _row(String k, String v) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 6),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 110, child: Text(k, style: const TextStyle(color: Colors.white54))),
          Expanded(child: Text(v)),
        ]),
      );
}
