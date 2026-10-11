import '../../domain/entities/affiliate_dashboard.dart';

String _moneyAmount(dynamic v) {
  if (v == null) return '0';
  if (v is String) return v;
  if (v is Map) return (v['amount'] ?? '0').toString();
  return v.toString();
}

String _moneyCurrency(dynamic v, String fallback) {
  if (v is Map && v['currency'] != null) {
    return v['currency'].toString();
  }
  return fallback;
}

/// Parses affiliate-service dashboard payloads.
/// Accepts both `GET /api/v1/affiliate/dashboard` shapes used by web client.
class AffiliateDashboardModel extends AffiliateDashboard {
  const AffiliateDashboardModel({
    required super.earningsToday,
    required super.earningsThisMonth,
    required super.pendingAmount,
    required super.availableAmount,
    required super.paidAmount,
    required super.currency,
    required super.clicks,
    required super.registrations,
    required super.ftdCount,
    required super.activePlayers,
    required super.ggrAmount,
    required super.ngrAmount,
    required super.commissionAmount,
  });

  factory AffiliateDashboardModel.fromJson(Map<String, dynamic> json) {
    final data = json['data'] is Map<String, dynamic> ? json['data'] : json;
    int asInt(dynamic v) => v is int ? v : int.tryParse('$v') ?? 0;
    return AffiliateDashboardModel(
      earningsToday: _moneyAmount(data['earnings_today']),
      earningsThisMonth: _moneyAmount(data['earnings_this_month']),
      pendingAmount: _moneyAmount(data['pending_amount']),
      availableAmount: _moneyAmount(data['available_amount']),
      paidAmount: _moneyAmount(data['paid_amount']),
      currency: _moneyCurrency(data['pending_amount'], 'USD'),
      clicks: asInt(data['clicks']),
      registrations: asInt(data['registrations']),
      ftdCount: asInt(data['ftd_count']),
      activePlayers: asInt(data['active_players']),
      ggrAmount: _moneyAmount(data['ggr_amount']),
      ngrAmount: _moneyAmount(data['ngr_amount']),
      commissionAmount: _moneyAmount(data['commission_amount']),
    );
  }
}

class AffiliateLinkModel extends AffiliateLink {
  const AffiliateLinkModel({
    required super.id,
    required super.campaignName,
    required super.referralCode,
    required super.referralUrl,
  });

  factory AffiliateLinkModel.fromJson(Map<String, dynamic> json) {
    return AffiliateLinkModel(
      id: '${json['id'] ?? ''}',
      campaignName: '${json['campaign_name'] ?? ''}',
      referralCode: '${json['referral_code'] ?? ''}',
      referralUrl: '${json['referral_url'] ?? ''}',
    );
  }
}

class AffiliatePayoutModel extends AffiliatePayout {
  const AffiliatePayoutModel({
    required super.id,
    required super.amount,
    required super.currency,
    required super.status,
  });

  factory AffiliatePayoutModel.fromJson(Map<String, dynamic> json) {
    final amount = json['amount'];
    return AffiliatePayoutModel(
      id: '${json['id'] ?? ''}',
      amount: _moneyAmount(amount),
      currency: _moneyCurrency(amount, '${json['currency'] ?? 'USD'}'),
      status: '${json['status'] ?? 'requested'}',
    );
  }
}
