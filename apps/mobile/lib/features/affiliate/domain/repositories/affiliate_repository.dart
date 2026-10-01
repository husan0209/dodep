import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../domain/entities/affiliate_dashboard.dart';

/// Repository contract for the affiliate cabinet.
abstract class AffiliateRepository {
  Future<Either<Failure, AffiliateDashboard>> getDashboard();
  Future<Either<Failure, List<AffiliateLink>>> getLinks();
  Future<Either<Failure, List<AffiliatePayout>>> getPayouts();
}
