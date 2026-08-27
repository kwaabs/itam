import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'asset_detail_screen.dart';

/// Field view of custody exceptions from the custody report — complements web 9.0.
class CustodyTasksScreen extends StatefulWidget {
  const CustodyTasksScreen({super.key});

  @override
  State<CustodyTasksScreen> createState() => _CustodyTasksScreenState();
}

class _CustodyTasksScreenState extends State<CustodyTasksScreen> {
  late final Repository _repo;
  CustodyReport? _report;
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
      final r = await _repo.custodyReport();
      setState(() {
        _report = r;
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  void _open(String assetId) {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => AssetDetailScreen(assetId: assetId)),
    ).then((_) => _load());
  }

  @override
  Widget build(BuildContext context) {
    final r = _report;
    return Scaffold(
      appBar: AppBar(title: const Text('Custody tasks')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView(
                    padding: const EdgeInsets.all(16),
                    children: [
                      if (r!.exceptionCount == 0)
                        const Padding(
                          padding: EdgeInsets.only(top: 48),
                          child: Center(child: Text('No custody exceptions — all clear.')),
                        )
                      else ...[
                        _section(
                          'Awaiting acknowledge',
                          r.unacknowledged,
                          icon: Icons.pending_actions,
                          color: const Color(0xFFfbbf24),
                        ),
                        _section(
                          'Assigned but in stock',
                          r.assignedInStock,
                          icon: Icons.inventory_2,
                          color: const Color(0xFFf97316),
                        ),
                        _section(
                          'Returned but not in stock',
                          r.notInStockAfterReturn,
                          icon: Icons.warning_amber,
                          color: const Color(0xFFef4444),
                        ),
                      ],
                    ],
                  ),
                ),
    );
  }

  Widget _section(String title, List<CustodyAnomalyRow> items, {required IconData icon, required Color color}) {
    if (items.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 8, top: 8),
          child: Row(
            children: [
              Icon(icon, size: 18, color: color),
              const SizedBox(width: 8),
              Text(title, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 15)),
              const SizedBox(width: 8),
              Text('(${items.length})', style: const TextStyle(color: Colors.white54)),
            ],
          ),
        ),
        for (final row in items)
          Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              title: Text(row.assetTag),
              subtitle: Text([row.name, row.stateName].where((s) => s.isNotEmpty).join(' · ')),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => _open(row.id),
            ),
          ),
        const SizedBox(height: 8),
      ],
    );
  }
}
