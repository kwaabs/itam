import 'package:flutter/material.dart';

import '../core/models.dart';
import '../data/repository.dart';

Future<String?> assignDefaultLocationHint(Repository repo, Person person) async {
  final orgId = person.orgUnitId;
  if (orgId == null || orgId.isEmpty) return null;
  final ous = await repo.orgUnits();
  OrgUnit? ou;
  for (final u in ous) {
    if (u.id == orgId) {
      ou = u;
      break;
    }
  }
  final locId = ou?.defaultLocationId;
  if (ou == null || locId == null || locId.isEmpty) return null;
  final locs = await repo.locations();
  for (final l in locs) {
    if (l.id == locId) return "${ou.name}: ${l.name}";
  }
  return "${ou.name}'s default location";
}

Future<Person?> pickPerson(BuildContext context, List<Person> people) {
  return _pick<Person>(
    context,
    title: 'Assign to',
    items: people,
    label: (p) => p.fullName.isEmpty ? p.email : p.fullName,
    sublabel: (p) => [p.employeeNo, p.email].where((s) => s.isNotEmpty).join(' · '),
    match: (p, q) => '${p.fullName} ${p.email} ${p.employeeNo}'.toLowerCase().contains(q),
  );
}

Future<LocationNode?> pickLocation(BuildContext context, List<LocationNode> locs) {
  return _pick<LocationNode>(
    context,
    title: 'Move to location',
    items: locs,
    label: (l) => '${'   ' * l.depth}${l.name}',
    sublabel: (l) => l.kind,
    match: (l, q) => '${l.name} ${l.kind}'.toLowerCase().contains(q),
  );
}

Future<OrgUnit?> pickOrgUnit(BuildContext context, List<OrgUnit> units) {
  return _pick<OrgUnit>(
    context,
    title: 'Owner org unit',
    items: units,
    label: (u) => '${'   ' * u.depth}${u.name}',
    sublabel: (u) => u.kind.isEmpty ? 'org unit' : u.kind,
    match: (u, q) => '${u.name} ${u.kind}'.toLowerCase().contains(q),
  );
}

/// After picking a person for assign, optionally pick a new location too.
/// [defaultHint] describes org default location when user keeps current location.
Future<String?> pickAssignLocation(BuildContext context, List<LocationNode> locs, {String? defaultHint}) async {
  final update = await showDialog<bool>(
    context: context,
    builder: (_) => AlertDialog(
      title: const Text('Update location?'),
      content: Text(
        defaultHint != null && defaultHint.isNotEmpty
            ? 'Also move this asset to a new location?\n\nIf you keep current, assign will use: $defaultHint'
            : 'Also move this asset to a new location?',
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context, false),
          child: Text(defaultHint != null && defaultHint.isNotEmpty ? 'Use org default' : 'Keep current'),
        ),
        FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Choose location')),
      ],
    ),
  );
  if (update != true || !context.mounted) return null;
  final loc = await pickLocation(context, locs);
  return loc?.id;
}

Future<T?> _pick<T>(
  BuildContext context, {
  required String title,
  required List<T> items,
  required String Function(T) label,
  required String Function(T) sublabel,
  required bool Function(T, String) match,
}) {
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) {
      String query = '';
      return StatefulBuilder(
        builder: (ctx, setState) {
          final filtered = query.isEmpty ? items : items.where((e) => match(e, query)).toList();
          return Padding(
            padding: EdgeInsets.only(bottom: MediaQuery.of(ctx).viewInsets.bottom),
            child: SizedBox(
              height: MediaQuery.of(ctx).size.height * 0.7,
              child: Column(
                children: [
                  Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      children: [
                        Text(title, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                        const SizedBox(height: 10),
                        TextField(
                          autofocus: true,
                          decoration: const InputDecoration(prefixIcon: Icon(Icons.search), hintText: 'Search', isDense: true),
                          onChanged: (v) => setState(() => query = v.toLowerCase()),
                        ),
                      ],
                    ),
                  ),
                  Expanded(
                    child: ListView.builder(
                      itemCount: filtered.length,
                      itemBuilder: (_, i) {
                        final e = filtered[i];
                        final sub = sublabel(e);
                        return ListTile(
                          title: Text(label(e)),
                          subtitle: sub.isEmpty ? null : Text(sub),
                          onTap: () => Navigator.pop(ctx, e),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      );
    },
  );
}
