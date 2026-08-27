import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'pickers.dart';

/// Lists open purchase orders to receive stock against.
class StockScreen extends StatefulWidget {
  const StockScreen({super.key});

  @override
  State<StockScreen> createState() => _StockScreenState();
}

class _StockScreenState extends State<StockScreen> {
  late final Repository _repo;
  List<PurchaseOrder> _pos = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final pos = await _repo.purchaseOrders();
      setState(() {
        _pos = pos.where((p) => p.status != 'cancelled' && p.status != 'closed').toList();
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
      appBar: AppBar(title: const Text('Stock receive')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : _pos.isEmpty
                  ? const Center(child: Text('No open purchase orders'))
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView.separated(
                        itemCount: _pos.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (_, i) {
                          final po = _pos[i];
                          return ListTile(
                            leading: const Icon(Icons.receipt_long),
                            title: Text(po.poNumber),
                            subtitle: Text('Status: ${po.status}'),
                            trailing: const Icon(Icons.chevron_right),
                            onTap: () async {
                              final received = await Navigator.push<bool>(
                                context,
                                MaterialPageRoute(builder: (_) => ReceivePOScreen(poId: po.id, poNumber: po.poNumber)),
                              );
                              if (received == true) _load();
                            },
                          );
                        },
                      ),
                    ),
    );
  }
}

/// Receive against a single PO: enter quantities + asset tags per line.
class ReceivePOScreen extends StatefulWidget {
  final String poId;
  final String poNumber;
  const ReceivePOScreen({super.key, required this.poId, required this.poNumber});

  @override
  State<ReceivePOScreen> createState() => _ReceivePOScreenState();
}

class _LineForm {
  final String id;
  final String description;
  final int ordered;
  final int received;
  final qtyCtrl = TextEditingController();
  final tagsCtrl = TextEditingController();
  _LineForm(this.id, this.description, this.ordered, this.received);
  int get remaining => (ordered - received).clamp(0, ordered);
}

class _ReceivePOScreenState extends State<ReceivePOScreen> {
  late final Repository _repo;
  List<_LineForm> _lines = [];
  LocationNode? _location;
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
    _load();
  }

  Future<void> _load() async {
    try {
      final po = await _repo.purchaseOrder(widget.poId);
      final lines = (po['lines'] as List?) ?? [];
      setState(() {
        _lines = lines.map((e) {
          final m = (e as Map).cast<String, dynamic>();
          return _LineForm(
            m['id'].toString(),
            (m['description'] ?? '').toString(),
            (m['quantity'] is num) ? (m['quantity'] as num).toInt() : 0,
            (m['received_qty'] is num) ? (m['received_qty'] as num).toInt() : 0,
          );
        }).toList();
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

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

  Future<void> _submit() async {
    final payloadLines = <Map<String, dynamic>>[];
    for (final l in _lines) {
      final qty = int.tryParse(l.qtyCtrl.text.trim()) ?? 0;
      if (qty <= 0) continue;
      final tags = l.tagsCtrl.text
          .split(RegExp(r'[,\n]'))
          .map((s) => s.trim())
          .where((s) => s.isNotEmpty)
          .toList();
      payloadLines.add({
        'po_line_id': l.id,
        'quantity': qty,
        'asset_tags': tags,
        'serials': <String>[],
      });
    }
    if (payloadLines.isEmpty) {
      _snack('Enter a quantity on at least one line');
      return;
    }
    setState(() => _busy = true);
    try {
      await _repo.receivePO(widget.poId, {
        'location_id': _location?.id,
        'notes': '',
        'lines': payloadLines,
      });
      if (!mounted) return;
      _snack('Received');
      Navigator.pop(context, true);
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
    return Scaffold(
      appBar: AppBar(title: Text('Receive ${widget.poNumber}')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
              : ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      shape: RoundedRectangleBorder(
                        side: const BorderSide(color: Colors.white24),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      leading: const Icon(Icons.place),
                      title: Text(_location?.name ?? 'Destination location (optional)'),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: _pickLoc,
                    ),
                    const SizedBox(height: 16),
                    for (final l in _lines) _lineCard(l),
                    const SizedBox(height: 8),
                    FilledButton(
                      onPressed: _busy ? null : _submit,
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 12),
                        child: _busy
                            ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Text('Receive'),
                      ),
                    ),
                  ],
                ),
    );
  }

  Widget _lineCard(_LineForm l) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l.description.isEmpty ? '(no description)' : l.description,
                style: const TextStyle(fontWeight: FontWeight.w600)),
            Text('Ordered ${l.ordered} · received ${l.received} · remaining ${l.remaining}',
                style: const TextStyle(color: Colors.white54, fontSize: 12)),
            const SizedBox(height: 10),
            Row(
              children: [
                SizedBox(
                  width: 90,
                  child: TextField(
                    controller: l.qtyCtrl,
                    decoration: const InputDecoration(labelText: 'Qty', isDense: true),
                    keyboardType: TextInputType.number,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: TextField(
                    controller: l.tagsCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Asset tags (comma/newline)',
                      isDense: true,
                    ),
                    minLines: 1,
                    maxLines: 3,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
