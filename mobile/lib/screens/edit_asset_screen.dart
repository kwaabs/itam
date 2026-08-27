import 'package:flutter/material.dart';

import 'asset_form_screen.dart';

class EditAssetScreen extends StatelessWidget {
  final String assetId;
  const EditAssetScreen({super.key, required this.assetId});

  @override
  Widget build(BuildContext context) {
    return AssetFormScreen(assetId: assetId);
  }
}
