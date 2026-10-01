import 'package:equatable/equatable.dart';

/// Affiliate dashboard aggregates (revshare from NGR model).
/// Money is transported as decimal strings, never double.
class AffiliateDashboard extends Equatable {
  final String earningsToday;
  final String earningsThisMonth;
  final String pendingAmount;
  final String availableAmount;
  final String paidAmount;
  final String currency;
  final int clicks;
  final int registrations;
  final int ftdCount;
  final int activePlayers;
  final String ggrAmount;
  final String ngrAmount;
  final String commissionAmount;

  const AffiliateDashboard({
    required this.earningsToday,
    required this.earningsThisMonth,
    required this.pendingAmount,
    required this.availableAmount,
    required this.paidAmount,
    required this.currency,
    required this.clicks,
    required this.registrations,
    required this.ftdCount,
    required this.activePlayers,
    required this.ggrAmount,
    required this.ngrAmount,
    required this.commissionAmount,
  });

  double get conversionRate =>
      clicks == 0 ? 0 : registrations / clicks;

  @override
  List<Object?> get props => [
        earningsToday,
        earningsThisMonth,
        pendingAmount,
        availableAmount,
        paidAmount,
        currency,
        clicks,
        registrations,
        ftdCount,
        activePlayers,
        ggrAmount,
        ngrAmount,
        commissionAmount,
      ];
}

/// Affiliate referral link entity.
class AffiliateLink extends Equatable {
  final String id;
  final String campaignName;
  final String referralCode;
  final String referralUrl;

  const AffiliateLink({
    required this.id,
    required this.campaignName,
    required this.referralCode,
    required this.referralUrl,
  });

  @override
  List<Object?> get props => [id, campaignName, referralCode, referralUrl];
}

/// Affiliate payout entity (status timeline surfaced in UI).
class AffiliatePayout extends Equatable {
  final String id;
  final String amount;
  final String currency;
  final String status;

  const AffiliatePayout({
    required this.id,
    required this.amount,
    required this.currency,
    required this.status,
  });

  @override
  List<Object?> get props => [id, amount, currency, status];
}
