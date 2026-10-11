import 'package:dartz/dartz.dart';
import 'package:injectable/injectable.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/error/failures.dart';
import '../../domain/entities/rg_limits.dart';
import '../../domain/repositories/rg_repository.dart';
import '../datasources/rg_remote_datasource.dart';
import '../models/rg_limits_model.dart';

/// Repository implementation mapping exceptions to failures.
@LazySingleton(as: RGRepository)
class RGRepositoryImpl implements RGRepository {
  final RGRemoteDataSource _remote;

  RGRepositoryImpl({required RGRemoteDataSource remote}) : _remote = remote;

  @override
  Future<Either<Failure, RGStatus>> loadStatus() async {
    try {
      return Right(await _remote.getStatus());
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, RGLimits>> setLimits(
    Map<String, String?> money, {
    int? sessionMinutes,
    int? realityCheckMinutes,
  }) async {
    try {
      final model = await _remote.setLimits(SetRGLimitsBody(
        money: money,
        sessionMinutes: sessionMinutes,
        realityCheckMinutes: realityCheckMinutes,
      ));
      return Right(model);
    } on ValidationException catch (e) {
      return Left(ValidationFailure(e.message));
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, RGExclusion>> selfExclude(String period) async {
    try {
      final model = await _remote.startSelfExclusion(SelfExclusionBody(period));
      return Right(model);
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, Unit>> revokeSelfExclusion() async {
    try {
      await _remote.revokeSelfExclusion(const RevokeSelfExclusionBody());
      return const Right(unit);
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, Unit>> startTimeout(String period) async {
    try {
      await _remote.startTimeout(period);
      return const Right(unit);
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, bool>> canPlay({
    required String channel,
    String? amount,
  }) async {
    try {
      return Right(
          await _remote.checkAllowed(channel: channel, amount: amount));
    } on NetworkException {
      return const Left(NetworkFailure());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }
}
