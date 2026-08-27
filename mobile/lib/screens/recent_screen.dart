import 'package:flutter/material.dart';

import '../core/recent.dart';
import 'asset_detail_screen.dart';

/// Recently opened assets, kept on-device. Handy for returning to something you
/// just looked at without scanning again.
class RecentScreen extends StatefulWidget {
  const RecentScreen({super.key});

  @override
  State<RecentScreen> createState() => _RecentScreenState();
}

class _RecentScreenState extends State<RecentScreen> {
  List<RecentAsset> _items = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final items = await Recent.list();
    if (!mounted) return;
    setState(() {
      _items = items;
      _loading = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Recent assets')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _items.isEmpty
              ? const Center(child: Text('Nothing viewed yet.', style: TextStyle(color: Colors.white54)))
              : ListView.separated(
                  itemCount: _items.length,
                  separatorBuilder: (_, __) => const Divider(height: 1),
                  itemBuilder: (_, i) {
                    final r = _items[i];
                    return ListTile(
                      leading: const Icon(Icons.history),
                      title: Text(r.tag),
                      subtitle: Text(r.name, maxLines: 1, overflow: TextOverflow.ellipsis),
                      onTap: () async {
                        await Navigator.push(context, MaterialPageRoute(builder: (_) => AssetDetailScreen(assetId: r.id)));
                        _load();
                      },
                    );
                  },
                ),
    );
  }
}
