import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

/// A lightweight record of an asset the user opened, kept on-device so the
/// "Recent assets" list survives restarts without any server round-trip.
class RecentAsset {
  final String id;
  final String tag;
  final String name;
  RecentAsset(this.id, this.tag, this.name);

  Map<String, dynamic> toJson() => {'id': id, 'tag': tag, 'name': name};
  factory RecentAsset.fromJson(Map<String, dynamic> j) =>
      RecentAsset(j['id']?.toString() ?? '', j['tag']?.toString() ?? '', j['name']?.toString() ?? '');
}

class Recent {
  static const _key = 'recent_assets';
  static const _max = 25;

  static Future<List<RecentAsset>> list() async {
    final p = await SharedPreferences.getInstance();
    final raw = p.getStringList(_key) ?? [];
    return raw.map((s) {
      try {
        return RecentAsset.fromJson(jsonDecode(s) as Map<String, dynamic>);
      } catch (_) {
        return RecentAsset('', '', '');
      }
    }).where((e) => e.id.isNotEmpty).toList();
  }

  static Future<void> add(String id, String tag, String name) async {
    if (id.isEmpty) return;
    final p = await SharedPreferences.getInstance();
    final raw = p.getStringList(_key) ?? [];
    raw.removeWhere((s) {
      try {
        return (jsonDecode(s) as Map)['id'] == id;
      } catch (_) {
        return false;
      }
    });
    raw.insert(0, jsonEncode({'id': id, 'tag': tag, 'name': name}));
    if (raw.length > _max) raw.removeRange(_max, raw.length);
    await p.setStringList(_key, raw);
  }
}
