import 'package:dartz/dartz.dart';
import 'package:injectable/injectable.dart';

import '../../../core/error/exceptions.dart';
import '../../../core/error/failures.dart';
import '../../domain/entities/user.dart';
import '../../domain/repositories/auth_repository.dart';
import '../datasources/auth_local_datasource.dart';
import '../datasources/auth_remote_datasource.dart';
import '../models/user_model.dart';

/// Implementation of AuthRepository
@LazySingleton(as: AuthRepository)
class AuthRepositoryImpl implements AuthRepository {
  final AuthRemoteDataSource _remoteDataSource;
  final AuthLocalDataSource _localDataSource;

  AuthRepositoryImpl({
    required AuthRemoteDataSource remoteDataSource,
    required AuthLocalDataSource localDataSource,
  })  : _remoteDataSource = remoteDataSource,
        _localDataSource = localDataSource;

  @override
  Future<Either<Failure, User>> login({
    required String email,
    required String password,
  }) async {
    try {
      final tokens = await _remoteDataSource.login(
        email: email,
        password: password,
      );

      await _localDataSource.saveTokens(tokens);

      final user = await _remoteDataSource.getCurrentUser();
      await _localDataSource.cacheUser(user);

      return Right(user);
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } on NetworkException {
      return const Left(NetworkFailure());
    } on AuthException catch (e) {
      return Left(AuthFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, User>> register({
    required String email,
    required String password,
    required String username,
  }) async {
    try {
      final tokens = await _remoteDataSource.register(
        email: email,
        password: password,
        username: username,
      );

      await _localDataSource.saveTokens(tokens);

      final user = await _remoteDataSource.getCurrentUser();
      await _localDataSource.cacheUser(user);

      return Right(user);
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } on NetworkException {
      return const Left(NetworkFailure());
    } on AuthException catch (e) {
      return Left(AuthFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, void>> logout() async {
    try {
      await _remoteDataSource.logout();
      await _localDataSource.clearTokens();
      await _localDataSource.clearCachedUser();
      return const Right(null);
    } on NetworkException {
      // Even if network fails, clear local data
      await _localDataSource.clearTokens();
      await _localDataSource.clearCachedUser();
      return const Right(null);
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, User>> getCurrentUser() async {
    try {
      final cachedUser = _localDataSource.getCachedUser();
      if (cachedUser != null) {
        return Right(cachedUser);
      }

      final user = await _remoteDataSource.getCurrentUser();
      await _localDataSource.cacheUser(user);

      return Right(user);
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } on NetworkException {
      return const Left(NetworkFailure());
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<Either<Failure, void>> refreshTokens() async {
    try {
      final refreshToken = _localDataSource.getRefreshToken();
      if (refreshToken == null) {
        return const Left(AuthFailure('Refresh token not found', code: 'NO_REFRESH_TOKEN'));
      }

      final tokens = await _remoteDataSource.refreshTokens(refreshToken: refreshToken);
      await _localDataSource.saveTokens(tokens);

      return const Right(null);
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message, code: e.code));
    } on NetworkException {
      return const Left(NetworkFailure());
    } on AuthException catch (e) {
      return Left(AuthFailure(e.message, code: e.code));
    } catch (e) {
      return Left(UnknownFailure(e.toString()));
    }
  }
}
