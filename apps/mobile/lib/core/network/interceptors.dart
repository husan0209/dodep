import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:hive_flutter/hive_flutter.dart';

/// Attaches the stored bearer token to outgoing requests.
///
/// The token is read from the same Hive box that `AuthLocalDataSourceImpl`
/// writes it to, so it always tracks login/logout. Endpoints that mint
/// credentials are skipped via [anonymousPaths], and a request with no
/// stored token is passed through untouched.
class AuthInterceptor extends Interceptor {
  AuthInterceptor({Box<dynamic>? tokenBox}) : _tokenBox = tokenBox;

  /// Hive box that `AuthLocalDataSourceImpl` persists tokens into.
  static const String tokenBoxName = 'secure_storage';

  /// Key holding the access token inside [tokenBoxName].
  static const String accessTokenKey = 'access_token';

  /// Endpoints that must never receive an `Authorization` header.
  static const Set<String> anonymousPaths = <String>{
    '/api/v1/auth/login',
    '/api/v1/auth/register',
    '/api/v1/auth/refresh',
  };

  final Box<dynamic>? _tokenBox;

  /// Resolves the token box lazily, so the interceptor can be built
  /// before Hive is initialised (it is created in `ApiClient`'s
  /// constructor, which runs while the object graph is being wired).
  Box<dynamic>? _resolveBox() {
    final injected = _tokenBox;
    if (injected != null) return injected;
    if (!Hive.isBoxOpen(tokenBoxName)) return null;
    return Hive.box<dynamic>(tokenBoxName);
  }

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    if (anonymousPaths.contains(options.uri.path)) {
      handler.next(options);
      return;
    }

    final token = _resolveBox()?.get(accessTokenKey);
    if (token is! String || token.isEmpty) {
      handler.next(options);
      return;
    }

    options.headers['Authorization'] = 'Bearer $token';
    handler.next(options);
  }
}

/// Logging interceptor for debugging
class LoggingInterceptor extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    if (kDebugMode) {
      print('┌───────────────────────────────────────────────────────────────');
      print('│ 🌐 ${options.method} ${options.uri}');
      print('│ Headers: ${options.headers}');
      if (options.data != null) {
        print('│ Body: ${options.data}');
      }
      print('└───────────────────────────────────────────────────────────────');
    }
    handler.next(options);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    if (kDebugMode) {
      print('┌───────────────────────────────────────────────────────────────');
      print('│ ✅ ${response.statusCode} ${response.requestOptions.uri}');
      print('│ Response: ${response.data}');
      print('└───────────────────────────────────────────────────────────────');
    }
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    if (kDebugMode) {
      print('┌───────────────────────────────────────────────────────────────');
      print('│ ❌ ${err.requestOptions.uri}');
      print('│ Error: ${err.message}');
      print('└───────────────────────────────────────────────────────────────');
    }
    handler.next(err);
  }
}

/// Retry interceptor for failed requests
class RetryInterceptor extends Interceptor {
  final Dio dio;
  final int retries;

  RetryInterceptor({required this.dio, this.retries = 2});

  @override
  Future<void> onError(
      DioException err, ErrorInterceptorHandler handler) async {
    // Only retry on network errors or 5xx server errors
    final shouldRetry = err.type == DioExceptionType.connectionError ||
        err.type == DioExceptionType.connectionTimeout ||
        (err.response?.statusCode ?? 0) >= 500;

    if (!shouldRetry) {
      return handler.next(err);
    }

    var retryCount = 0;
    DioException? lastError;

    while (retryCount < retries) {
      try {
        retryCount++;
        if (kDebugMode) {
          print(
              '🔄 Retry attempt $retryCount/$retries for ${err.requestOptions.uri}');
        }

        // Wait with exponential backoff
        await Future.delayed(Duration(milliseconds: 1000 * retryCount));

        // Retry the request
        final response = await dio.fetch(err.requestOptions);
        return handler.resolve(response);
      } on DioException catch (e) {
        lastError = e;
      }
    }

    // All retries failed
    return handler.next(lastError ?? err);
  }
}
