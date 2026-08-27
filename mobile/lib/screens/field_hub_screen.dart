import 'package:flutter/material.dart';

/// One destination inside a field-app hub menu.
class HubEntry {
  const HubEntry({
    required this.title,
    this.subtitle,
    required this.icon,
    required this.enabled,
    required this.screen,
  });

  final String title;
  final String? subtitle;
  final IconData icon;
  final bool enabled;
  final Widget screen;
}

/// Simple sub-menu for grouped field workflows.
class FieldHubScreen extends StatelessWidget {
  const FieldHubScreen({
    super.key,
    required this.title,
    this.subtitle,
    required this.entries,
  });

  final String title;
  final String? subtitle;
  final List<HubEntry> entries;

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
        children: [
          if (subtitle != null)
            Padding(
              padding: const EdgeInsets.fromLTRB(4, 0, 4, 16),
              child: Text(subtitle!, style: TextStyle(color: Colors.white.withValues(alpha: 0.55), fontSize: 14)),
            ),
          ...entries.map((e) => _HubTile(entry: e, colorScheme: c)),
        ],
      ),
    );
  }
}

class _HubTile extends StatelessWidget {
  const _HubTile({required this.entry, required this.colorScheme});

  final HubEntry entry;
  final ColorScheme colorScheme;

  @override
  Widget build(BuildContext context) {
    return Opacity(
      opacity: entry.enabled ? 1 : 0.45,
      child: Card(
        margin: const EdgeInsets.only(bottom: 10),
        color: colorScheme.surfaceContainerHighest,
        child: ListTile(
          leading: Icon(entry.icon, color: colorScheme.primary),
          title: Text(entry.title, style: const TextStyle(fontWeight: FontWeight.w600)),
          subtitle: entry.subtitle != null
              ? Text(entry.subtitle!, style: const TextStyle(fontSize: 13, color: Colors.white54))
              : null,
          trailing: entry.enabled ? const Icon(Icons.chevron_right, color: Colors.white38) : const Text('No permission', style: TextStyle(fontSize: 11, color: Colors.white38)),
          onTap: entry.enabled
              ? () => Navigator.push(context, MaterialPageRoute(builder: (_) => entry.screen))
              : null,
        ),
      ),
    );
  }
}
