import 'package:dio/dio.dart';
import 'package:injectable/injectable.dart';

import '../../../../core/error/exceptions.dart';
import '../models/rg_limits_model.dart';
import 'rg_remote_datasource.dart';

/// Dio implementation of the RG cabinet against the Go RG service.
/// The AuthInterceptor attaches the JWT; the service derives the player
/// identity from it.
@LazySingleton(as: RGRemoteDataSource)
class RGRemoteDataSourceImpl implements RGRemoteDataSource {
  static const String _base = '/api/v1/rg';

  final Dio _dio;

  RGRemoteDataSourceImpl({required Dio dio}) : _dio = dio;

  @override
  Future<RGLimitsEnvelopeModel> getLimits() async {
    final res = await _get('$_base/limits');
    return RGLimitsEnvelopeModel.fromJson(res);
  }

  @override
  Future<RGLimitsEnvelopeModel> setLimits(SetRGLimitsBody body) async {
    final payload = body.toJson();
    if (payload == null) {
      throw const ValidationException('Нечего сохранять: все поля пустые');
    }
    final res = await _put('$_base/limits', payload);
    return RGLimitsEnvelopeModel.fromJson(res);
  }

  @override
  Future<RGStatusModel> getStatus() async {
    final res = await _get('$_base/status');
    return RGStatusModel.fromJson(res);
  }

  @override
  Future<RGExclusionModel> startSelfExclusion(SelfExclusionBody body) async {
    final res = await _post('$_base/self-exclusion', body.toJson(), expectCreated: true);
    return RGExclusionModel.fromJson(res);
  }

  @override
  Future<void> revokeSelfExclusion(RevokeSelfExclusionBody body) async {
    await _post('$_base/self-exclusion/revoke', body.toJson());
  }

  @override
  Future<void> startTimeout(String period) async {
    await _post('$_base/timeout', {'period': period}, expectCreated: true);
  }

  /// Returns true when the gate allows the action. HTTP 403 is a valid
  /// answer (not an error): the gate decided the player may not play.
  @override
  Future<bool> checkAllowed({required String channel, String? amount}) async {
    try {
      final response = await _dio.post(
        '$_base/check',
        data: {
          'channel': channel,
          if (amount != null && amount.isNotEmpty) 'amount': amount,
        },
      );
      final status = response.statusCode ?? 0;
      if (status == 200 || status == 403) {
        final body = response.data;
        final data = body is Map<String, dynamic> && body['data'] is Map<String, dynamic>
            ? body['data'] as Map<String, dynamic>
            : const <String, dynamic>{};
        return data['allowed'] == true;
      }
      throw _handleError(response);
    } on DioException catch (e) {
      if (e.response?.statusCode == 403) {
        final body = e.response?.data;
        final data = body is Map<String, dynamic> && body['data'] is Map<String, dynamic>
            ? body['data'] as Map<String, dynamic>
            : const <String, dynamic>{};
        return data['allowed'] == true;
      }
      throw _handleDioException(e);
    }
  }

  // ── transport ────────────────────────────────────────────────────────────

  Future<Map<String, dynamic>> _get(String path) async {
    try {
      final res = await _dio.get<Map<String, dynamic>>(path);
      return res.data ?? const {};
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  Future<Map<String, dynamic>> _put(String path, Map<String, dynamic> body) async {
    try {
      final res = await _dio.put<Map<String, dynamic>>(path, data: body);
      return res.data ?? const {};
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  Future<Map<String, dynamic>> _post(
    String path,
    Map<String, dynamic> body, {
    bool expectCreated = false,
  }) async {
    try {
      final res = await _dio.post<Map<String, dynamic>>(path, data: body);
      final status = res.statusCode ?? 0;
      if (expectCreated ? status != 201 : status != 200) {
        throw ServerException('unexpected status $status', statusCode: status);
      }
      return res.data ?? const {};
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  Exception _handleDioException(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return const TimeoutException();
      case DioExceptionType.connectionError:
        return const NetworkException();
      case DioExceptionType.badResponse:
      case DioExceptionType.badCertificate:
      case DioExceptionType.cancel:
        return ServerException.fromResponse(
          e.response?.statusCode ?? 500,
          (e.response?.data as Map<String, dynamic>?) ?? {},
        );
      case DioExceptionType.unknown:
        return ServerException(e.message ?? 'Unknown error');
    }
  }

  Exception _handleError(Response response) {
    return ServerException.fromResponse(
      response.statusCode ?? 500,
      (response.data as Map<String, dynamic>?) ?? {},
    );
  }
}