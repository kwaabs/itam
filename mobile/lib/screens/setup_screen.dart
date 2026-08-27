import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/config.dart';

/// First-run connection setup: where the API and GoTrue live.
class SetupScreen extends StatefulWidget {
  const SetupScreen({super.key});

  @override
  State<SetupScreen> createState() => _SetupScreenState();
}

class _SetupScreenState extends State<SetupScreen> {
  late final TextEditingController _api;
  late final TextEditingController _gotrue;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    final cfg = context.read<AppController>().config;
    _api = TextEditingController(text: cfg.apiBaseUrl);
    _gotrue = TextEditingController(text: cfg.gotrueUrl);
  }

  @override
  void dispose() {
    _api.dispose();
    _gotrue.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    await context.read<AppController>().saveConfig(
          AppConfig(apiBaseUrl: _api.text.trim(), gotrueUrl: _gotrue.text.trim()),
        );
    if (mounted) setState(() => _saving = false);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Connect to ITAM')),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 460),
          child: ListView(
            shrinkWrap: true,
            padding: const EdgeInsets.all(20),
            children: [
              const Text('Server connection', style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold)),
              const SizedBox(height: 6),
              const Text(
                'Enter the ITAM API and auth (GoTrue) addresses. On an Android emulator '
                'use 10.0.2.2; on a phone use the host PC\'s LAN IP.',
                style: TextStyle(color: Colors.white60),
              ),
              const SizedBox(height: 20),
              TextField(
                controller: _api,
                decoration: const InputDecoration(labelText: 'API base URL', hintText: 'http://localhost:5607'),
                keyboardType: TextInputType.url,
              ),
              const SizedBox(height: 14),
              TextField(
                controller: _gotrue,
                decoration: const InputDecoration(labelText: 'GoTrue (auth) URL', hintText: 'http://localhost:5606'),
                keyboardType: TextInputType.url,
              ),
              const SizedBox(height: 22),
              FilledButton(
                onPressed: _saving ? null : _save,
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  child: _saving
                      ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Continue'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
