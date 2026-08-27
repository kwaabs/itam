import 'dart:html' as html;

import 'package:dio/dio.dart';

import 'oauth_common.dart';

/// Reads `#access_token=…` from the current URL after GoTrue OAuth callback.
String? takeHashAccessToken() {
  final hash = html.window.location.hash;
  if (hash.isEmpty || !hash.startsWith('#')) return null;
  final params = Uri.splitQueryString(hash.substring(1));
  final token = params['access_token'];
  if (token == null || token.isEmpty) return null;
  final path = html.window.location.pathname ?? '/';
  html.window.history.replaceState(null, '', path);
  return token;
}

String? currentAppOrigin() => html.window.location.origin;

void startAzureLogin(String gotrueUrl, String returnTo) {
  final base = gotrueUrl.replaceAll(RegExp(r'/+$'), '');
  final q = Uri(queryParameters: {
    'provider': 'azure',
    'redirect_to': returnTo,
  }).query;
  html.window.location.assign('$base/authorize?$q');
}

Future<bool> azureLoginEnabled(String apiBaseUrl) async {
  try {
    final base = apiBaseUrl.replaceAll(RegExp(r'/+$'), '');
    final resp = await Dio(BaseOptions(validateStatus: (s) => s != null && s < 500))
        .get('$base/auth/sso/status');
    return resp.statusCode == 200 && resp.data is Map && resp.data['enabled'] == true;
  } catch (_) {
    return false;
  }
}

Future<AzureLoginOutcome> completeAzureLogin(String gotrueUrl) async {
  final origin = currentAppOrigin();
  if (origin == null || origin.isEmpty) {
    return AzureLoginOutcome.error('Could not determine app origin');
  }
  startAzureLogin(gotrueUrl, origin);
  return AzureLoginOutcome.cancelled();
}
