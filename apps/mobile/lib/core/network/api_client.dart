import 'package:dio/dio.dart';
import 'package:injectable/injectable.dart';

import '../config/app_config.dart';
import 'interceptors.dart';

/// API Client based on Dio
@singleton
class ApiClient {
  late final Dio _dio;

  ApiClient() {
    _dio = Dio(
      BaseOptions(
        baseUrl: AppConfig.apiBaseUrl,
        connectTimeout: const Duration(seconds: 30),
        receiveTimeout: const Duration(seconds: 30),
        headers: {
          'Content-Type': 'application/json',
        },
      ),
    );

    _dio.interceptors.addAll([
      AuthInterceptor(),
      LoggingInterceptor(),
      RetryInterceptor(dio: _dio, retries: 2),
    ]);
  }

  /// GET request
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? params,
  }) async {
    return await _dio.get<T>(path, queryParameters: params);
  }

  /// POST request
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
  }) async {
    return await _dio.post<T>(path, data: data);
  }

  /// PUT request
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
  }) async {
    return await _dio.put<T>(path, data: data);
  }

  /// PATCH request
  Future<Response<T>> patch<T>(
    String path, {
    dynamic data,
  }) async {
    return await _dio.patch<T>(path, data: data);
  }

  /// DELETE request
  Future<Response<T>> delete<T>(String path) async {
    return await _dio.delete<T>(path);
  }

  /// Download file
  Future<Response> download(String path, String savePath) async {
    return await _dio.download(path, savePath);
  }

  Dio get dio => _dio;
}
