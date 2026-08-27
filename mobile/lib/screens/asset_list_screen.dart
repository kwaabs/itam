import 'package:flutter/material.dart';

import '../core/models.dart';
import '../data/repository.dart';
import 'widgets.dart';

/// Generic "list of assets" page driven by a loader. Reused by Near me,
/// Stores → devices, etc.
class AssetListScreen extends StatefulWidget {
  final String title;
  final String? subtitle;
  final Future<List<Asset>> Function() loader;
  final String emptyText;

  const AssetListScreen({
    super.key,
    required this.title,
    required this.loader,
    this.subtitle,
    this.emptyText = 'No devices here.',
  });

  @override
  State<AssetListScreen> createState() => _AssetListScreenState();
}

class _AssetListScreenState extends State<AssetListScreen> {
  List<Asset> _assets = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final a = await widget.loader();
      setState(() {
        _assets = a;
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
      appBar: AppBar(
        title: Text(widget.title),
        bottom: widget.subtitle == null
            ? null
            : PreferredSize(
                preferredSize: const Size.fromHeight(24),
                child: Padding(
                  padding: const EdgeInsets.only(left: 16, bottom: 8),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Text(widget.subtitle!, style: const TextStyle(fontSize: 12, color: Colors.white60)),
                  ),
                ),
              ),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: _assets.isEmpty
                      ? ListView(children: [Padding(padding: const EdgeInsets.all(32), child: Center(child: Text(widget.emptyText, style: const TextStyle(color: Colors.white54))))])
                      : ListView.separated(
                          itemCount: _assets.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (_, i) => assetTile(context, _assets[i], onAfter: _load),
                        ),
                ),
    );
  }
}
