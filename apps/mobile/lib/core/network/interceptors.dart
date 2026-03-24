import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

import '../error/exceptions.dart';

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
  Future<void> onError(DioException err, ErrorInterceptorHandler handler) async {
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
          print('🔄 Retry attempt $retryCount/$retries for ${err.requestOptions.uri}');
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
