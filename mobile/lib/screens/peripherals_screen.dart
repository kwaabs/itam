import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'create_asset_screen.dart';
import 'search_screen.dart';
import 'widgets.dart';

/// Browse peripherals (monitors, keyboards, mice, etc.) with quick add and search.
class PeripheralsScreen extends StatefulWidget {
  const PeripheralsScreen({super.key});

  @override
  State<PeripheralsScreen> createState() => _PeripheralsScreenState();
}

class _PeripheralsScreenState extends State<PeripheralsScreen> {
  late final Repository _repo;
  List<Asset> _items = [];
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
      final items = await _repo.listAssets(type: 'peripheral', pageSize: 100);
      setState(() {
        _items = items;
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  void _search() {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => const SearchScreen(typeKey: 'peripheral', title: 'Search peripherals')),
    );
  }

  void _add() {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => const CreateAssetScreen(peripheralsOnly: true)),
    ).then((_) => _load());
  }

  @override
  Widget build(BuildContext context) {
    final canWrite = context.watch<AppController>().me?.can('asset.write') ?? false;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Peripherals'),
        actions: [
          IconButton(icon: const Icon(Icons.search), tooltip: 'Search', onPressed: _search),
        ],
      ),
      floatingActionButton: canWrite
          ? FloatingActionButton.extended(
              onPressed: _add,
              icon: const Icon(Icons.add),
              label: const Text('Add'),
            )
          : null,
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: _items.isEmpty
                      ? ListView(
                          children: const [
                            SizedBox(height: 80),
                            Center(
                              child: Text('No peripherals yet.', style: TextStyle(color: Colors.white54)),
                            ),
                          ],
                        )
                      : ListView.separated(
                          itemCount: _items.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (_, i) => assetTile(context, _items[i], onAfter: _load),
                        ),
                ),
    );
  }
}
