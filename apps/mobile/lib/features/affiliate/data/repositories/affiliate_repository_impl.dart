import 'package:dartz/dartz.dart';
import 'package:injectable/injectable.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/error/failures.dart';
import '../../domain/entities/affiliate_dashboard.dart';
import '../../domain/repositories/affiliate_repository.dart';
import '../datasources/affiliate_remote_datasource.dart';

/// Repository implementation mapping exceptions to failures.
@LazySingleton(as: AffiliateRepository)
class AffiliateRepositoryImpl implements AffiliateRepository {
  final AffiliateRemoteDataSource _remote;

  AffiliateRepositoryImpl({required AffiliateRemoteDataSource remote})
      : _remote = remote;

  @override
  Future<Either<Failure, AffiliateDashboard>> getDashboard() async {
    try {
      return Right(await _remote.getDashboard());
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, List<AffiliateLink>>> getLinks() async {
    try {
      return Right(await _remote.getLinks());
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, List<AffiliatePayout>>> getPayouts() async {
    try {
      return Right(await _remote.getPayouts());
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }
}
