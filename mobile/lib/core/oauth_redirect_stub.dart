import 'oauth_common.dart';

/// Fallback when neither web nor IO OAuth is available.
String? takeHashAccessToken() => null;

String? currentAppOrigin() => null;

void startAzureLogin(String apiBaseUrl, String returnTo) {
  throw UnsupportedError('Azure AD sign-in is not supported on this platform');
}

Future<bool> azureLoginEnabled(String apiBaseUrl) async => false;

Future<AzureLoginOutcome> completeAzureLogin(String apiBaseUrl) async {
  return AzureLoginOutcome.error('Azure AD sign-in is not supported on this platform');
}
