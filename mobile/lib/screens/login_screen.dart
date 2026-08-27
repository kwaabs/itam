import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _email = TextEditingController();
  final _password = TextEditingController();
  bool _busy = false;
  bool _ssoEnabled = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadSso();
  }

  Future<void> _loadSso() async {
    final app = context.read<AppController>();
    final enabled = await app.azureLoginEnabled();
    if (mounted) setState(() => _ssoEnabled = enabled);
  }

  @override
  void dispose() {
    _email.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await context.read<AppController>().login(_email.text.trim(), _password.text);
    if (mounted) {
      setState(() {
        _busy = false;
        _error = err;
      });
    }
  }

  Future<void> _microsoftLogin() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await context.read<AppController>().loginWithMicrosoft();
    if (mounted) {
      setState(() {
        _busy = false;
        if (err != null) _error = err;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final cfg = context.watch<AppController>().config;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Sign in'),
        actions: [
          IconButton(
            tooltip: 'Connection settings',
            icon: const Icon(Icons.settings),
            onPressed: () => context.read<AppController>().backToSetup(),
          ),
        ],
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: ListView(
            shrinkWrap: true,
            padding: const EdgeInsets.all(20),
            children: [
              const Text('ITAM Field', style: TextStyle(fontSize: 26, fontWeight: FontWeight.bold)),
              Text(cfg.apiBaseUrl, style: const TextStyle(color: Colors.white38, fontSize: 12)),
              const SizedBox(height: 24),
              TextField(
                controller: _email,
                decoration: const InputDecoration(labelText: 'Email'),
                keyboardType: TextInputType.emailAddress,
                autofillHints: const [AutofillHints.username],
              ),
              const SizedBox(height: 14),
              TextField(
                controller: _password,
                decoration: const InputDecoration(labelText: 'Password'),
                obscureText: true,
                autofillHints: const [AutofillHints.password],
                onSubmitted: (_) => _login(),
              ),
              if (_error != null) ...[
                const SizedBox(height: 14),
                Text(_error!, style: const TextStyle(color: Color(0xFFef4444))),
              ],
              const SizedBox(height: 22),
              FilledButton(
                onPressed: _busy ? null : _login,
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  child: _busy
                      ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Sign in'),
                ),
              ),
              if (_ssoEnabled) ...[
                const SizedBox(height: 12),
                OutlinedButton(
                  onPressed: _busy ? null : _microsoftLogin,
                  child: const Padding(
                    padding: EdgeInsets.symmetric(vertical: 12),
                    child: Text('Sign in with Microsoft'),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
