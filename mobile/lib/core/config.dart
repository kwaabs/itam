/// Connection settings entered on first launch and persisted locally.
class AppConfig {
  final String apiBaseUrl;

  const AppConfig({required this.apiBaseUrl});

  /// Sensible dev default. For the Android emulator use 10.0.2.2 instead of
  /// localhost; on a physical device use the host machine's LAN IP.
  static const defaults = AppConfig(apiBaseUrl: 'http://localhost:5607');

  AppConfig copyWith({String? apiBaseUrl}) => AppConfig(apiBaseUrl: apiBaseUrl ?? this.apiBaseUrl);

  bool get isComplete => apiBaseUrl.trim().isNotEmpty;
}
