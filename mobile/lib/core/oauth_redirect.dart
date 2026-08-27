export 'oauth_common.dart';

export 'oauth_redirect_stub.dart'
    if (dart.library.html) 'oauth_redirect_web.dart'
    if (dart.library.io) 'oauth_redirect_io.dart';
