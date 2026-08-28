import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_web_auth_2/flutter_web_auth_2.dart';

import 'oauth_common.dart';

String? takeHashAccessToken() => null;

String? currentAppOrigin() => azureNativeCallbackUrl;

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

void startAzureLogin(String apiBaseUrl, String returnTo) {
  throw UnsupportedError('Use completeAzureLogin() on native platforms');
}

Future<AzureLoginOutcome> completeAzureLogin(String apiBaseUrl) async {
  final base = apiBaseUrl.replaceAll(RegExp(r'/+$'), '');
  final loginUrl = Uri.parse('$base/auth/azure/start').replace(queryParameters: {
    'redirect_to': azureNativeCallbackUrl,
  }).toString();
  try {
    final result = await FlutterWebAuth2.authenticate(
      url: loginUrl,
      callbackUrlScheme: 'itam',
      options: const FlutterWebAuth2Options(useWebview: false),
    );
    return _parseCallback(result);
  } on PlatformException catch (e) {
    if (e.code == 'CANCELED') return AzureLoginOutcome.cancelled();
    return AzureLoginOutcome.error(e.message ?? 'Microsoft sign-in failed');
  } catch (e) {
    return AzureLoginOutcome.error(e.toString());
  }
}

AzureLoginOutcome _parseCallback(String url) {
  final uri = Uri.parse(url);
  if (uri.fragment.isNotEmpty) {
    final params = Uri.splitQueryString(uri.fragment);
    final token = params['access_token'];
    if (token != null && token.isNotEmpty) {
      return AzureLoginOutcome.success(
        token,
        refreshToken: params['refresh_token'],
      );
    }
    final err = params['error_description'] ?? params['error'];
    if (err != null && err.isNotEmpty) {
      return AzureLoginOutcome.error(err);
    }
  }
  final err = uri.queryParameters['sso_error'] ?? uri.queryParameters['error_description'];
  if (err != null && err.isNotEmpty) {
    return AzureLoginOutcome.error(err);
  }
  return AzureLoginOutcome.error('No token returned from Microsoft sign-in');
}
