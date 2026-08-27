import 'package:flutter/material.dart';

import '../core/models.dart';
import 'asset_detail_screen.dart';

Color? _hex(String color) {
  if (color.startsWith('#') && color.length == 7) {
    return Color(int.parse('FF${color.substring(1)}', radix: 16));
  }
  return null;
}

/// A consistent row for an asset that opens its detail page on tap.
/// [onAfter] fires when returning from the detail page (e.g. to refresh a list).
Widget assetTile(BuildContext context, Asset a, {VoidCallback? onAfter}) {
  final subtitle = [a.name, if (a.locationName.isNotEmpty) a.locationName].join(' · ');
  final stateColor = _hex(a.stateColor);
  return ListTile(
    leading: const Icon(Icons.devices_other),
    title: Text(a.assetTag),
    subtitle: Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis),
    trailing: a.stateName.isEmpty
        ? null
        : Text(a.stateName, style: TextStyle(fontSize: 12, color: stateColor ?? Colors.white54)),
    onTap: () async {
      await Navigator.push(context, MaterialPageRoute(builder: (_) => AssetDetailScreen(assetId: a.id)));
      onAfter?.call();
    },
  );
}
