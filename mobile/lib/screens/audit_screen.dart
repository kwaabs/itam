import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/geo.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'pickers.dart';
import 'scan_screen.dart';

/// Stocktake: pick a location, scan everything physically present, and see
/// which expected assets are missing and which scanned items are misplaced.
class AuditScreen extends StatefulWidget {
  const AuditScreen({super.key});

  @override
  State<AuditScreen> createState() => _AuditScreenState();
}

class _AuditScreenState extends State<AuditScreen> {
  late final Repository _repo;
  late final bool _canMove;

  LocationNode? _location;
  List<Asset> _expected = [];
  final Map<String, Asset> _extras = {}; // scanned here but belong elsewhere
  final Set<String> _seen = {};
  bool _loading = false;
  bool _locating = false;

  @override
  void initState() {
    super.initState();
    final app = context.read<AppController>();
    _repo = Repository(app.dio);
    _canMove = app.me?.can('asset.assign') ?? false;
  }

  Future<void> _chooseLocation() async {
    List<LocationNode> locs;
    try {
      locs = await _repo.locations();
    } on ApiException catch (e) {
      _snack(e.message);
      return;
    }
    if (!mounted) return;
    final loc = await pickLocation(context, locs);
    if (loc == null) return;
    await _startAudit(loc);
  }

  /// Resolve the device's GPS position to the nearest location that has
  /// coordinates (stores / sites), then audit it.
  Future<void> _useMyLocation() async {
    setState(() => _locating = true);
    try {
      final pos = await Geo.currentPosition();
      final radiusKm = await _repo.nearbyRadiusKm();
      final locs = await _repo.locations();
      final near = Geo.nearest(locs, pos.latitude, pos.longitude, maxMeters: radiusKm * 1000);
      if (near == null) {
        _snack('No location within ${radiusKm.round()} km of you.');
        return;
      }
      _snack('Nearest: ${near.location.name} · ${near.distanceLabel}');
      await _startAudit(near.location);
    } on GeoException catch (e) {
      _snack(e.message);
    } on ApiException catch (e) {
      _snack(e.message);
    } finally {
      if (mounted) setState(() => _locating = false);
    }
  }

  Future<void> _startAudit(LocationNode loc) async {
    setState(() {
      _location = loc;
      _loading = true;
      _expected = [];
      _seen.clear();
      _extras.clear();
    });
    try {
      final list = await _repo.assetsByLocation(loc.id);
      setState(() {
        _expected = list;
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() => _loading = false);
      _snack(e.message);
    }
  }

  Future<void> _scan() async {
    final asset = await Navigator.push<Asset>(
      context,
      MaterialPageRoute(builder: (_) => const ScanScreen(returnResult: true)),
    );
    if (asset == null) return;
    _record(asset);
  }

  void _record(Asset a) {
    final belongsHere = _expected.any((e) => e.id == a.id);
    setState(() {
      _seen.add(a.id);
      if (!belongsHere) _extras[a.id] = a;
    });
    _snack(belongsHere ? 'Confirmed ${a.assetTag}' : 'Misplaced: ${a.assetTag}');
  }

  Future<void> _moveHere(Asset a) async {
    if (_location == null) return;
    try {
      await _repo.transfer(a.id, _location!.id, reason: 'Stocktake relocate');
      setState(() {
        _extras.remove(a.id);
        if (!_expected.any((e) => e.id == a.id)) _expected = [..._expected, a];
      });
      _snack('Moved ${a.assetTag} here');
    } on ApiException catch (e) {
      _snack(e.message);
    }
  }

  void _snack(String m) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m), duration: const Duration(seconds: 1)));
  }

  @override
  Widget build(BuildContext context) {
    final missing = _expected.where((e) => !_seen.contains(e.id)).toList();
    final present = _expected.where((e) => _seen.contains(e.id)).toList();

    return Scaffold(
      appBar: AppBar(title: const Text('Audit / stocktake')),
      floatingActionButton: _location == null
          ? null
          : FloatingActionButton.extended(onPressed: _scan, icon: const Icon(Icons.qr_code_scanner), label: const Text('Scan')),
      body: _location == null
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.fact_check, size: 56, color: Colors.white24),
                    const SizedBox(height: 16),
                    const Text('Pick a location to audit', textAlign: TextAlign.center),
                    const SizedBox(height: 16),
                    FilledButton.icon(onPressed: _chooseLocation, icon: const Icon(Icons.place), label: const Text('Choose location')),
                    const SizedBox(height: 10),
                    OutlinedButton.icon(
                      onPressed: _locating ? null : _useMyLocation,
                      icon: _locating
                          ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                          : const Icon(Icons.my_location),
                      label: Text(_locating ? 'Locating…' : 'Use my location'),
                    ),
                  ],
                ),
              ),
            )
          : _loading
              ? const Center(child: CircularProgressIndicator())
              : Column(
                  children: [
                    _header(present.length, missing.length),
                    Expanded(
                      child: ListView(
                        children: [
                          if (_extras.isNotEmpty) ...[
                            _sectionTitle('Misplaced here (${_extras.length})', const Color(0xFFf59e0b)),
                            for (final a in _extras.values)
                              ListTile(
                                leading: const Icon(Icons.warning_amber, color: Color(0xFFf59e0b)),
                                title: Text(a.assetTag),
                                subtitle: Text('${a.name} · belongs: ${a.locationName.isEmpty ? "—" : a.locationName}'),
                                trailing: _canMove
                                    ? TextButton(onPressed: () => _moveHere(a), child: const Text('Move here'))
                                    : null,
                              ),
                          ],
                          _sectionTitle('Confirmed present (${present.length})', const Color(0xFF22c55e)),
                          for (final a in present)
                            ListTile(
                              leading: const Icon(Icons.check_circle, color: Color(0xFF22c55e)),
                              title: Text(a.assetTag),
                              subtitle: Text(a.name),
                            ),
                          _sectionTitle('Not yet seen (${missing.length})', Colors.white38),
                          for (final a in missing)
                            ListTile(
                              leading: const Icon(Icons.radio_button_unchecked, color: Colors.white38),
                              title: Text(a.assetTag),
                              subtitle: Text(a.name),
                              onTap: () => _record(a), // tap to confirm manually
                            ),
                        ],
                      ),
                    ),
                  ],
                ),
    );
  }

  Widget _header(int present, int missing) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(child: Text(_location!.name, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold))),
              TextButton(onPressed: _chooseLocation, child: const Text('Change')),
            ],
          ),
          const SizedBox(height: 4),
          Text('$present of ${_expected.length} confirmed · $missing missing · ${_extras.length} misplaced',
              style: const TextStyle(color: Colors.white60)),
        ],
      ),
    );
  }

  Widget _sectionTitle(String t, Color c) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
        child: Text(t, style: TextStyle(color: c, fontWeight: FontWeight.bold)),
      );
}
