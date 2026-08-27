import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'config.dart';
import 'models.dart';
import 'oauth_redirect.dart' as oauth;

enum AppStatus { loading, needsSetup, needsLogin, ready }

/// Central app store: connection config, GoTrue tokens, the authenticated Dio
/// client, and the current user's permissions. Screens read this via provider.
class AppController extends ChangeNotifier {
  AppController();

  final _secure = const FlutterSecureStorage();

  AppConfig _config = AppConfig.defaults;
  AppConfig get config => _config;

  AppStatus _status = AppStatus.loading;
  AppStatus get status => _status;

  String? _accessToken;
  String? _refreshToken;
  Me? _me;
  Me? get me => _me;

  late Dio _dio;
  Dio get dio => _dio;

  bool _refreshing = false;

  // ---- bootstrap -----------------------------------------------------------

  Future<void> init() async {
    final prefs = await SharedPreferences.getInstance();
    final api = prefs.getString('api_base_url');
    final gotrue = prefs.getString('gotrue_url');
    final configured = prefs.getBool('configured') ?? false;
    if (configured && api != null && gotrue != null) {
      _config = AppConfig(apiBaseUrl: api, gotrueUrl: gotrue);
    }
    _buildDio();

    _accessToken = await _secure.read(key: 'access_token');
    _refreshToken = await _secure.read(key: 'refresh_token');

    // Azure AD SSO on Flutter web returns tokens in the URL hash.
    final ssoToken = oauth.takeHashAccessToken();
    if (ssoToken != null && ssoToken.isNotEmpty) {
      _accessToken = ssoToken;
      await _secure.write(key: 'access_token', value: ssoToken);
    }

    if (!configured) {
      _set(AppStatus.needsSetup);
      return;
    }
    if (_accessToken == null) {
      _set(AppStatus.needsLogin);
      return;
    }
    // Validate the saved token by loading /me; refresh once if needed.
    final ok = await _loadMe();
    _set(ok ? AppStatus.ready : AppStatus.needsLogin);
  }

  void _buildDio() {
    _dio = Dio(BaseOptions(
      baseUrl: _config.apiBaseUrl,
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 30),
      // Don't throw on 4xx so callers can read structured error bodies.
      validateStatus: (s) => s != null && s < 500,
    ));
    // validateStatus lets 4xx through as normal responses so callers can read
    // structured error bodies; so we detect 401 here in onResponse (not onError),
    // refresh the token once, and replay the original request.
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) {
        if (_accessToken != null) {
          options.headers['Authorization'] = 'Bearer $_accessToken';
        }
        handler.next(options);
      },
      onResponse: (response, handler) async {
        final req = response.requestOptions;
        if (response.statusCode == 401 &&
            _refreshToken != null &&
            !_refreshing &&
            req.extra['retried'] != true) {
          final ok = await _refresh();
          if (ok) {
            req.headers['Authorization'] = 'Bearer $_accessToken';
            req.extra['retried'] = true;
            try {
              final clone = await _dio.fetch(req);
              return handler.resolve(clone);
            } catch (_) {/* fall through to original 401 */}
          }
        }
        handler.next(response);
      },
    ));
  }

  // ---- config --------------------------------------------------------------

  Future<void> saveConfig(AppConfig cfg) async {
    _config = cfg;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('api_base_url', cfg.apiBaseUrl.trim());
    await prefs.setString('gotrue_url', cfg.gotrueUrl.trim());
    await prefs.setBool('configured', true);
    _buildDio();
    _set(_accessToken == null ? AppStatus.needsLogin : AppStatus.ready);
  }

  // ---- auth ----------------------------------------------------------------

  Future<String?> login(String email, String password) async {
    final auth = Dio(BaseOptions(validateStatus: (s) => s != null && s < 500));
    try {
      final url = '${_config.apiBaseUrl.replaceAll(RegExp(r'/+$'), '')}/auth/login';
      final resp = await auth.post(
        url,
        data: {'email': email, 'password': password},
        options: Options(headers: {'Content-Type': 'application/json'}),
      );
      if (resp.statusCode != 200 || resp.data is! Map) {
        return _authError(resp.data) ?? 'Login failed (${resp.statusCode})';
      }
      final data = resp.data as Map;
      _accessToken = data['access_token']?.toString();
      _refreshToken = data['refresh_token']?.toString();
      if (_accessToken == null) return 'No token returned';
      await _persistTokens();
      final ok = await _loadMe();
      if (!ok) return 'Signed in, but could not reach the API. Check the API URL.';
      _set(AppStatus.ready);
      return null;
    } on DioException catch (e) {
      return 'Cannot reach auth server: ${e.message}';
    }
  }

  Future<bool> azureLoginEnabled() => oauth.azureLoginEnabled(_config.apiBaseUrl);

  void startAzureLogin() {
    final origin = oauth.currentAppOrigin();
    if (origin == null || origin.isEmpty) {
      throw UnsupportedError('Azure AD sign-in requires a valid return URL');
    }
    oauth.startAzureLogin(_config.gotrueUrl, origin);
  }

  /// Microsoft SSO through GoTrue. On web the browser navigates away; on native
  /// the system browser returns a GoTrue token via the itam:// deep link.
  Future<String?> loginWithMicrosoft() async {
    if (kIsWeb) {
      try {
        startAzureLogin();
      } catch (e) {
        return e.toString();
      }
      return null;
    }
    final outcome = await oauth.completeAzureLogin(_config.gotrueUrl);
    if (outcome.cancelled) return null;
    if (outcome.error != null) return outcome.error;
    final token = outcome.token;
    if (token == null || token.isEmpty) return 'No token returned';
    _accessToken = token;
    _refreshToken = outcome.refreshToken;
    await _persistTokens();
    final ok = await _loadMe();
    if (!ok) return 'Signed in, but could not reach the API. Check the API URL.';
    _set(AppStatus.ready);
    return null;
  }

  Future<bool> _refresh() async {
    if (_refreshToken == null) return false;
    _refreshing = true;
    final auth = Dio(BaseOptions(validateStatus: (s) => s != null && s < 500));
    try {
      final url = '${_config.apiBaseUrl.replaceAll(RegExp(r'/+$'), '')}/auth/refresh';
      final resp = await auth.post(url, data: {'refresh_token': _refreshToken});
      if (resp.statusCode == 200 && resp.data is Map) {
        final data = resp.data as Map;
        _accessToken = data['access_token']?.toString();
        _refreshToken = data['refresh_token']?.toString() ?? _refreshToken;
        await _persistTokens();
        return true;
      }
      return false;
    } catch (_) {
      return false;
    } finally {
      _refreshing = false;
    }
  }

  Future<bool> _loadMe() async {
    try {
      final resp = await _dio.get('/api/me');
      if (resp.statusCode == 200 && resp.data is Map) {
        _me = Me.fromJson((resp.data as Map).cast<String, dynamic>());
        return true;
      }
      return false;
    } catch (_) {
      return false;
    }
  }

  /// Return to the connection-settings screen (without wiping tokens).
  void backToSetup() => _set(AppStatus.needsSetup);

  Future<void> logout() async {
    if (_refreshToken != null) {
      // Best-effort server-side revocation; don't block clearing local state.
      final token = _refreshToken;
      final base = _config.apiBaseUrl.replaceAll(RegExp(r'/+$'), '');
      unawaited(Future(() async {
        try {
          await Dio().post('$base/auth/logout', data: {'refresh_token': token});
        } catch (_) {/* best-effort */}
      }));
    }
    _accessToken = null;
    _refreshToken = null;
    _me = null;
    await _secure.delete(key: 'access_token');
    await _secure.delete(key: 'refresh_token');
    _set(AppStatus.needsLogin);
  }

  Future<void> _persistTokens() async {
    await _secure.write(key: 'access_token', value: _accessToken);
    if (_refreshToken != null) {
      await _secure.write(key: 'refresh_token', value: _refreshToken);
    }
  }

  String? _authError(dynamic body) {
    if (body is Map) {
      return body['error_description']?.toString() ??
          body['msg']?.toString() ??
          body['error']?.toString();
    }
    return null;
  }

  void _set(AppStatus s) {
    _status = s;
    notifyListeners();
  }
}
