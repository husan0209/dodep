import '../models/affiliate_models.dart';

/// Remote source for the affiliate cabinet (Go affiliate-service).
abstract class AffiliateRemoteDataSource {
  Future<AffiliateDashboardModel> getDashboard();
  Future<List<AffiliateLinkModel>> getLinks();
  Future<List<AffiliatePayoutModel>> getPayouts();
  Future<void> enroll({String? reason});
  Future<void> requestPayout({
    required String methodId,
    required String amount,
    required String idempotencyKey,
  });
}
