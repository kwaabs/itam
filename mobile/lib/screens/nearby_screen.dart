import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/geo.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'widgets.dart';

/// Uses the device GPS to find the nearest location with coordinates, then lists
/// every device there (subtree rollup).
class NearbyScreen extends StatefulWidget {
  const NearbyScreen({super.key});

  @override
  State<NearbyScreen> createState() => _NearbyScreenState();
}

class _NearbyScreenState extends State<NearbyScreen> {
  late final Repository _repo;
  NearestLocation? _near;
  List<Asset> _assets = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
    _locate();
  }

  Future<void> _locate() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final pos = await Geo.currentPosition();
      final radiusKm = await _repo.nearbyRadiusKm();
      final locs = await _repo.locations();
      final near = Geo.nearest(locs, pos.latitude, pos.longitude, maxMeters: radiusKm * 1000);
      if (near == null) {
        setState(() {
          _error = 'No location within ${radiusKm.round()} km. Move closer, or add coordinates to a store/site.';
          _loading = false;
        });
        return;
      }
      final assets = await _repo.assetsByLocation(near.location.id);
      setState(() {
        _near = near;
        _assets = assets;
        _loading = false;
      });
    } on GeoException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Near me')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _loading ? null : _locate,
        icon: const Icon(Icons.my_location),
        label: const Text('Refresh'),
      ),
      body: _loading
          ? const Center(child: Column(mainAxisSize: MainAxisSize.min, children: [CircularProgressIndicator(), SizedBox(height: 16), Text('Finding your location…')]))
          : _error != null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(mainAxisSize: MainAxisSize.min, children: [
                      const Icon(Icons.location_off, size: 48, color: Colors.white24),
                      const SizedBox(height: 12),
                      Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: Colors.white70)),
                      const SizedBox(height: 16),
                      FilledButton.icon(onPressed: _locate, icon: const Icon(Icons.refresh), label: const Text('Try again')),
                    ]),
                  ),
                )
              : Column(
                  children: [
                    Container(
                      width: double.infinity,
                      padding: const EdgeInsets.all(16),
                      color: Theme.of(context).colorScheme.surfaceContainerHighest,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(_near!.location.name, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                          const SizedBox(height: 4),
                          Text('${_near!.location.kind} · ${_near!.distanceLabel} · ${_assets.length} device${_assets.length == 1 ? '' : 's'}',
                              style: const TextStyle(color: Colors.white60)),
                        ],
                      ),
                    ),
                    Expanded(
                      child: _assets.isEmpty
                          ? const Center(child: Text('No devices recorded here.', style: TextStyle(color: Colors.white54)))
                          : ListView.separated(
                              itemCount: _assets.length,
                              separatorBuilder: (_, __) => const Divider(height: 1),
                              itemBuilder: (_, i) => assetTile(context, _assets[i], onAfter: _locate),
                            ),
                    ),
                  ],
                ),
    );
  }
}
