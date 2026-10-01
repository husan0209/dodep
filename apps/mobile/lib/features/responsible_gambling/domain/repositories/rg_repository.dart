import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../domain/entities/rg_limits.dart';

/// Repository contract for the RG cabinet.
abstract class RGRepository {
  Future<Either<Failure, RGStatus>> loadStatus();
  Future<Either<Failure, RGLimits>> setLimits(Map<String, String?> money,
      {int? sessionMinutes, int? realityCheckMinutes});
  Future<Either<Failure, RGExclusion>> selfExclude(String period);
  Future<Either<Failure, Unit>> revokeSelfExclusion();
  Future<Either<Failure, Unit>> startTimeout(String period);
  Future<Either<Failure, bool>> canPlay({required String channel, String? amount});
}