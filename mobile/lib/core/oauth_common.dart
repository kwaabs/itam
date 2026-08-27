/// Result of a GoTrue Azure AD sign-in attempt on native platforms.
class AzureLoginOutcome {
  final String? token;
  final String? refreshToken;
  final String? error;
  final bool cancelled;

  const AzureLoginOutcome._({this.token, this.refreshToken, this.error, this.cancelled = false});

  factory AzureLoginOutcome.success(String token, {String? refreshToken}) =>
      AzureLoginOutcome._(token: token, refreshToken: refreshToken);

  factory AzureLoginOutcome.error(String message) =>
      AzureLoginOutcome._(error: message);

  factory AzureLoginOutcome.cancelled() => const AzureLoginOutcome._(cancelled: true);
}

/// Deep link the ITAM API redirects to after Microsoft SSO on native apps.
const String azureNativeCallbackUrl = 'itam://sso-callback';
