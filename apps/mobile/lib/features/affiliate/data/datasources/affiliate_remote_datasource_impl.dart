import 'package:dio/dio.dart';
import 'package:injectable/injectable.dart';

import '../../../../core/error/exceptions.dart';
import '../models/affiliate_models.dart';
import 'affiliate_remote_datasource.dart';

/// Dio implementation hitting the affiliate-service REST API.
/// Auth token is attached by AuthInterceptor; user identity comes from JWT.
@LazySingleton(as: AffiliateRemoteDataSource)
class AffiliateRemoteDataSourceImpl implements AffiliateRemoteDataSource {
  final Dio _dio;

  AffiliateRemoteDataSourceImpl({required Dio dio}) : _dio = dio;

  @override
  Future<AffiliateDashboardModel> getDashboard() async {
    try {
      final response = await _dio.get('/api/v1/affiliate/dashboard');
      if (response.statusCode == 200) {
        return AffiliateDashboardModel.fromJson(
          response.data as Map<String, dynamic>,
        );
      }
      throw _handleError(response);
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<List<AffiliateLinkModel>> getLinks() async {
    try {
      final response = await _dio.get('/api/v1/affiliate/links');
      if (response.statusCode == 200) {
        final body = response.data as Map<String, dynamic>;
        final items = (body['links'] ?? body['data'] ?? []) as List;
        return items
            .map((e) => AffiliateLinkModel.fromJson(e as Map<String, dynamic>))
            .toList();
      }
      throw _handleError(response);
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<List<AffiliatePayoutModel>> getPayouts() async {
    try {
      final response = await _dio.get('/api/v1/affiliate/payouts');
      if (response.statusCode == 200) {
        final body = response.data as Map<String, dynamic>;
        final items = (body['data'] ?? body['payouts'] ?? []) as List;
        return items
            .map((e) => AffiliatePayoutModel.fromJson(e as Map<String, dynamic>))
            .toList();
      }
      throw _handleError(response);
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<void> enroll({String? reason}) async {
    try {
      final response = await _dio.post(
        '/api/v1/affiliate/enroll',
        data: {'reason': reason ?? ''},
      );
      if (response.statusCode == 201 || response.statusCode == 200) return;
      throw _handleError(response);
    } on DioException catch (e) {
      throw _handleDioException(e);
    }
  }

  @override
  Future<void> requestPayout({
    required String methodId,
    required String amount,
    required String idempotencyKey,
  }) async {
    try {
      final response = await _dio.post(
        '/api/v1/affiliate/payouts/request',
        data: {
          'method_id': methodId,
          'amount': amount,
          'idempotency_key': idempotencyKey,
        },
      );
      if (response.statusCode == 201 || response.statusCode == 200) return;
      throw _handleError(response);
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
        return ServerException.fromResponse(
          e.response?.statusCode ?? 500,
          (e.response?.data as Map<String, dynamic>?) ?? {},
        );
      default:
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
