import 'package:flutter/material.dart';

import 'asset_form_screen.dart';

class CreateAssetScreen extends StatelessWidget {
  final bool peripheralsOnly;
  final String? initialTypeKey;

  const CreateAssetScreen({super.key, this.peripheralsOnly = false, this.initialTypeKey});

  @override
  Widget build(BuildContext context) {
    return AssetFormScreen(peripheralsOnly: peripheralsOnly, initialTypeKey: initialTypeKey);
  }
}
