import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'asset_detail_screen.dart';

/// Camera scanner with a manual-entry fallback. Resolves a scanned QR deep link
/// (carries the asset id) or a Code128 barcode (the asset tag) to an asset.
class ScanScreen extends StatefulWidget {
  /// When set, the scanned asset is returned via Navigator.pop instead of
  /// opening its detail page (used by the audit flow).
  final bool returnResult;
  const ScanScreen({super.key, this.returnResult = false});

  @override
  State<ScanScreen> createState() => _ScanScreenState();
}

class _ScanScreenState extends State<ScanScreen> {
  final _controller = MobileScannerController(formats: const [BarcodeFormat.qrCode, BarcodeFormat.code128]);
  final _manual = TextEditingController();
  bool _handling = false;
  bool _busy = false;

  late final Repository _repo;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
  }

  @override
  void dispose() {
    _controller.dispose();
    _manual.dispose();
    super.dispose();
  }

  void _onDetect(BarcodeCapture capture) {
    if (_handling) return;
    for (final b in capture.barcodes) {
      final raw = b.rawValue;
      if (raw != null && raw.trim().isNotEmpty) {
        _resolve(raw.trim());
        break;
      }
    }
  }

  Future<void> _resolve(String raw) async {
    setState(() {
      _handling = true;
      _busy = true;
    });
    try {
      String? id;
      final m = RegExp(r'/assets/([0-9a-fA-F-]{8,})').firstMatch(raw);
      if (m != null) {
        id = m.group(1);
      } else if (RegExp(r'^[0-9a-fA-F]{8}-[0-9a-fA-F-]+$').hasMatch(raw)) {
        id = raw;
      }

      if (id != null) {
        final a = await _repo.getAsset(id);
        _deliver(a);
        return;
      }
      final list = await _repo.searchAssets(raw);
      if (list.isEmpty) {
        _snack('No asset matches "$raw"');
        _resume();
      } else if (list.length == 1) {
        _deliver(list.first);
      } else {
        _pick(list, raw);
      }
    } on ApiException catch (e) {
      _snack(e.message);
      _resume();
    } catch (e) {
      _snack('Lookup failed: $e');
      _resume();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _deliver(Asset a) async {
    if (!mounted) return;
    if (widget.returnResult) {
      Navigator.pop(context, a);
      return;
    }
    await Navigator.push(context, MaterialPageRoute(builder: (_) => AssetDetailScreen(assetId: a.id)));
    _resume();
  }

  Future<void> _pick(List<Asset> list, String q) async {
    final chosen = await showModalBottomSheet<Asset>(
      context: context,
      builder: (_) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: [
            Padding(
              padding: const EdgeInsets.all(16),
              child: Text('${list.length} matches for "$q"',
                  style: const TextStyle(fontWeight: FontWeight.bold)),
            ),
            for (final a in list)
              ListTile(
                title: Text(a.assetTag),
                subtitle: Text(a.name),
                onTap: () => Navigator.pop(context, a),
              ),
          ],
        ),
      ),
    );
    if (chosen != null) {
      _deliver(chosen);
    } else {
      _resume();
    }
  }

  void _resume() {
    if (mounted) setState(() => _handling = false);
  }

  void _snack(String m) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Scan asset'),
        actions: [
          IconButton(icon: const Icon(Icons.cameraswitch), onPressed: () => _controller.switchCamera()),
          IconButton(icon: const Icon(Icons.flash_on), onPressed: () => _controller.toggleTorch()),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: Stack(
              alignment: Alignment.center,
              children: [
                MobileScanner(controller: _controller, onDetect: _onDetect),
                if (_busy) const CircularProgressIndicator(),
                IgnorePointer(
                  child: Container(
                    width: 240,
                    height: 240,
                    decoration: BoxDecoration(
                      border: Border.all(color: Colors.white70, width: 2),
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _manual,
                    decoration: const InputDecoration(
                      labelText: 'Enter asset tag manually',
                      isDense: true,
                    ),
                    onSubmitted: (v) {
                      if (v.trim().isNotEmpty) _resolve(v.trim());
                    },
                  ),
                ),
                const SizedBox(width: 10),
                FilledButton(
                  onPressed: _busy
                      ? null
                      : () {
                          final v = _manual.text.trim();
                          if (v.isNotEmpty) _resolve(v);
                        },
                  child: const Text('Find'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
