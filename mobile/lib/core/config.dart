/// Connection settings entered on first launch and persisted locally.
class AppConfig {
  final String apiBaseUrl;
  final String gotrueUrl;

  const AppConfig({required this.apiBaseUrl, required this.gotrueUrl});

  /// Sensible dev defaults. For the Android emulator use 10.0.2.2 instead of
  /// localhost; on a physical device use the host machine's LAN IP.
  static const defaults = AppConfig(
    apiBaseUrl: 'http://localhost:5607',
    gotrueUrl: 'http://localhost:5606',
  );

  AppConfig copyWith({String? apiBaseUrl, String? gotrueUrl}) => AppConfig(
        apiBaseUrl: apiBaseUrl ?? this.apiBaseUrl,
        gotrueUrl: gotrueUrl ?? this.gotrueUrl,
      );

  bool get isComplete => apiBaseUrl.trim().isNotEmpty && gotrueUrl.trim().isNotEmpty;
}
