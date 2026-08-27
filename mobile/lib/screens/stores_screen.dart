import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:latlong2/latlong.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'asset_list_screen.dart';

/// Lists stock-holding stores with a map of the ones that have coordinates.
/// Tapping a store opens its full device list (subtree rollup).
class StoresScreen extends StatefulWidget {
  const StoresScreen({super.key});

  @override
  State<StoresScreen> createState() => _StoresScreenState();
}

class _StoresScreenState extends State<StoresScreen> {
  late final Repository _repo;
  List<Store> _stores = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final s = await _repo.stores();
      setState(() {
        _stores = s;
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  void _openStore(Store s) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => AssetListScreen(
          title: s.name,
          subtitle: s.address.isEmpty ? 'Stock in this store' : s.address,
          loader: () => _repo.assetsByLocation(s.id),
          emptyText: 'No stock held here.',
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final located = _stores.where((s) => s.hasCoords).toList();
    return Scaffold(
      appBar: AppBar(title: const Text('Stores')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView(
                    children: [
                      if (located.isNotEmpty)
                        SizedBox(
                          height: 220,
                          child: _StoresMap(stores: located, onTap: _openStore),
                        ),
                      if (_stores.isEmpty)
                        const Padding(padding: EdgeInsets.all(32), child: Center(child: Text('No stores yet.', style: TextStyle(color: Colors.white54)))),
                      for (final s in _stores)
                        ListTile(
                          leading: const Icon(Icons.warehouse),
                          title: Text(s.name),
                          subtitle: Text([
                            if (s.address.isNotEmpty) s.address,
                            '${s.inStockCount} in stock',
                            if (s.departmentCount > 0) '${s.departmentCount} dept${s.departmentCount == 1 ? '' : 's'}',
                          ].join(' · ')),
                          trailing: const Icon(Icons.chevron_right),
                          onTap: () => _openStore(s),
                        ),
                    ],
                  ),
                ),
    );
  }
}

class _StoresMap extends StatelessWidget {
  final List<Store> stores;
  final void Function(Store) onTap;
  const _StoresMap({required this.stores, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final pts = stores.map((s) => LatLng(s.latitude!, s.longitude!)).toList();
    final center = LatLng(
      pts.map((p) => p.latitude).reduce((a, b) => a + b) / pts.length,
      pts.map((p) => p.longitude).reduce((a, b) => a + b) / pts.length,
    );
    return FlutterMap(
      options: MapOptions(initialCenter: center, initialZoom: pts.length == 1 ? 12 : 4),
      children: [
        TileLayer(
          urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
          userAgentPackageName: 'com.itam.field',
        ),
        MarkerLayer(
          markers: [
            for (final s in stores)
              Marker(
                point: LatLng(s.latitude!, s.longitude!),
                width: 40,
                height: 40,
                child: GestureDetector(
                  onTap: () => onTap(s),
                  child: const Icon(Icons.location_on, color: Color(0xFF6aa6ff), size: 36),
                ),
              ),
          ],
        ),
      ],
    );
  }
}
