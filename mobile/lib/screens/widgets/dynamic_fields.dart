import 'package:flutter/material.dart';

import '../../core/models.dart';

/// Renders type-specific attribute fields from metadata field definitions.
class DynamicFieldsForm extends StatelessWidget {
  final List<FieldDefinition> fields;
  final Map<String, dynamic> values;
  final Map<String, String> errors;
  final ValueChanged<Map<String, dynamic>> onChanged;

  const DynamicFieldsForm({
    super.key,
    required this.fields,
    required this.values,
    required this.onChanged,
    this.errors = const {},
  });

  void _set(String key, dynamic v) {
    final next = Map<String, dynamic>.from(values);
    if (v == null || (v is String && v.isEmpty)) {
      next.remove(key);
    } else {
      next[key] = v;
    }
    onChanged(next);
  }

  static String _optValue(dynamic o) {
    if (o is Map && o['value'] != null) return o['value'].toString();
    return o.toString();
  }

  static String _optLabel(dynamic o) {
    if (o is Map && o['label'] != null) return o['label'].toString();
    return o.toString();
  }

  @override
  Widget build(BuildContext context) {
    if (fields.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Type-specific fields', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 12),
        for (final f in fields) ...[
          _field(context, f),
          const SizedBox(height: 14),
        ],
      ],
    );
  }

  Widget _field(BuildContext context, FieldDefinition f) {
    final unit = f.unit?.symbol;
    final label = unit == null || unit.isEmpty ? f.label : '${f.label} ($unit)';
    final err = errors[f.key];

    Widget input;
    switch (f.valueKind) {
      case 'number':
        input = TextFormField(
          initialValue: values[f.key]?.toString() ?? '',
          keyboardType: const TextInputType.numberWithOptions(decimal: true),
          decoration: InputDecoration(labelText: f.required ? '$label *' : label, errorText: err),
          onChanged: (v) => _set(f.key, v.isEmpty ? null : num.tryParse(v)),
        );
      case 'bool':
        final cur = values[f.key];
        final boolVal = cur == true || cur == 'true';
        input = InputDecorator(
          decoration: InputDecoration(labelText: f.required ? '$label *' : label, errorText: err),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<bool>(
              isExpanded: true,
              value: boolVal,
              items: const [
                DropdownMenuItem(value: true, child: Text('Yes')),
                DropdownMenuItem(value: false, child: Text('No')),
              ],
              onChanged: (v) => _set(f.key, v),
            ),
          ),
        );
      case 'date':
        input = TextFormField(
          initialValue: _dateStr(values[f.key]),
          decoration: InputDecoration(
            labelText: f.required ? '$label *' : label,
            errorText: err,
            suffixIcon: const Icon(Icons.calendar_today, size: 18),
          ),
          readOnly: true,
          onTap: () async {
            final initial = _parseDate(values[f.key]) ?? DateTime.now();
            final picked = await showDatePicker(
              context: context,
              initialDate: initial,
              firstDate: DateTime(1990),
              lastDate: DateTime(2100),
            );
            if (picked != null) {
              _set(f.key, picked.toIso8601String().split('T').first);
            }
          },
        );
      case 'enum':
        input = DropdownButtonFormField<String>(
          value: values[f.key]?.toString().isNotEmpty == true ? values[f.key].toString() : null,
          decoration: InputDecoration(labelText: f.required ? '$label *' : label, errorText: err),
          items: [
            if (!f.required) const DropdownMenuItem(value: null, child: Text('—')),
            for (final o in f.enumOptions)
              DropdownMenuItem(value: _optValue(o), child: Text(_optLabel(o))),
          ],
          onChanged: (v) => _set(f.key, v),
        );
      default:
        input = TextFormField(
          initialValue: values[f.key]?.toString() ?? '',
          decoration: InputDecoration(labelText: f.required ? '$label *' : label, errorText: err),
          onChanged: (v) => _set(f.key, v),
        );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        input,
        if (f.helpText.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: Text(f.helpText, style: const TextStyle(fontSize: 12, color: Colors.white54)),
          ),
      ],
    );
  }

  static String _dateStr(dynamic v) {
    if (v == null) return '';
    final s = v.toString();
    return s.contains('T') ? s.split('T').first : s;
  }

  static DateTime? _parseDate(dynamic v) {
    if (v == null) return null;
    final s = _dateStr(v);
    return DateTime.tryParse(s);
  }
}

/// Read-only display of attribute key/values using field labels when available.
class AttributeRows extends StatelessWidget {
  final Map<String, dynamic> attributes;
  final List<FieldDefinition> fields;

  const AttributeRows({super.key, required this.attributes, this.fields = const []});

  @override
  Widget build(BuildContext context) {
    if (attributes.isEmpty) return const SizedBox.shrink();
    final labels = {for (final f in fields) f.key: f.label};
    const hidden = {'ingest_match_method', 'ingest_match_confidence'};
    final keys = attributes.keys.where((k) => !hidden.contains(k)).toList()..sort();
    if (keys.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Attributes', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        for (final k in keys)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 6),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(width: 110, child: Text(labels[k] ?? k, style: const TextStyle(color: Colors.white54))),
                Expanded(child: Text(_fmt(attributes[k]))),
              ],
            ),
          ),
      ],
    );
  }

  static String _fmt(dynamic v) {
    if (v == null) return '—';
    if (v is bool) return v ? 'Yes' : 'No';
    return v.toString();
  }
}
