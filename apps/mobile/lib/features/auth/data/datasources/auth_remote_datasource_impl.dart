import 'package:dio/dio.dart';
import 'package:injectable/injectable.dart';

import '../../../core/error/exceptions.dart';
import '../models/auth_tokens.dart';
import '../models/user_model.dart';
import 'auth_remote_datasource.dart';

/// Implementation of AuthRemoteDataSource
@LazySingleton(as: AuthRemoteDataSource)
class AuthRemoteDataSourceImpl implements AuthRemoteDataSource {
  final Dio _dio;

  AuthRemoteDataSourceImpl({required Dio dio}) : _dio = dio;

  @override
  Future<AuthTokens> login({
    required String email,
    required String password,
  }) async {
    try {
      final response = await _dio.post(
        '/api/v1/auth/login',
        data: {
          'email': email,
          'password': password,
        },
      );

      if (response.statusCode == 200) {
        final data = response.data['data'] as Map<String, dynamic>;
        return AuthTokens.fromJson(data);
      } else {
        throw _handleError(response);
      }
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<AuthTokens> register({
    required String email,
    required String password,
    required String username,
  }) async {
    try {
      final response = await _dio.post(
        '/api/v1/auth/register',
        data: {
          'email': email,
          'password': password,
          'username': username,
        },
      );

      if (response.statusCode == 201) {
        final data = response.data['data'] as Map<String, dynamic>;
        return AuthTokens.fromJson(data);
      } else {
        throw _handleError(response);
      }
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<void> logout() async {
    try {
      await _dio.post('/api/v1/auth/logout');
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<UserModel> getCurrentUser() async {
    try {
      final response = await _dio.get('/api/v1/auth/me');

      if (response.statusCode == 200) {
        final data = response.data['data'] as Map<String, dynamic>;
        return UserModel.fromJson(data);
      } else {
        throw _handleError(response);
      }
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<AuthTokens> refreshTokens({required String refreshToken}) async {
    try {
      final response = await _dio.post(
        '/api/v1/auth/refresh',
        data: {
          'refresh_token': refreshToken,
        },
      );

      if (response.statusCode == 200) {
        final data = response.data['data'] as Map<String, dynamic>;
        return AuthTokens.fromJson(data);
      } else {
        throw _handleError(response);
      }
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  /// Handle Dio exceptions
  Exception _handleDioException(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return const TimeoutException();
      case DioExceptionType.connectionError:
        return const NetworkException();
      case DioExceptionType.badResponse:
        return ServerException.fromResponse(
          e.response!.statusCode!,
          e.response!.data as Map<String, dynamic>,
        );
      default:
        return ServerException(e.message ?? 'Unknown error');
    }
  }

  /// Handle error response
  Exception _handleError(Response response) {
    return ServerException.fromResponse(
      response.statusCode!,
      response.data as Map<String, dynamic>,
    );
  }
}
