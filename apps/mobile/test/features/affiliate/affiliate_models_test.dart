import 'package:flutter_test/flutter_test.dart';

import 'package:dod_mobile/features/affiliate/data/models/affiliate_models.dart';

void main() {
  group('AffiliateDashboardModel.fromJson', () {
    test('parses dashboard envelope', () {
      final model = AffiliateDashboardModel.fromJson({
        'earnings_today': {'amount': '10.50', 'currency': 'USD'},
        'earnings_this_month': {'amount': '120.00', 'currency': 'USD'},
        'pending_amount': {'amount': '80.00', 'currency': 'USD'},
        'available_amount': {'amount': '40.00', 'currency': 'USD'},
        'paid_amount': {'amount': '200.00', 'currency': 'USD'},
        'clicks': 100,
        'registrations': 10,
        'ftd_count': 4,
        'active_players': 6,
        'ggr_amount': {'amount': '1000.00', 'currency': 'USD'},
        'ngr_amount': {'amount': '600.00', 'currency': 'USD'},
        'commission_amount': {'amount': '120.00', 'currency': 'USD'},
      });

      expect(model.earningsToday, '10.50');
      expect(model.currency, 'USD');
      expect(model.clicks, 100);
      expect(model.conversionRate, closeTo(0.1, 1e-9));
    });

    test('parses data-wrapped envelope with string numbers', () {
      final model = AffiliateDashboardModel.fromJson({
        'data': {
          'earnings_today': '0',
          'pending_amount': '5.25',
          'clicks': '0',
          'registrations': '0',
        },
      });

      expect(model.pendingAmount, '5.25');
      expect(model.conversionRate, 0);
    });
  });

  group('AffiliateLinkModel.fromJson', () {
    test('parses link fields', () {
      final model = AffiliateLinkModel.fromJson({
        'id': 'link-1',
        'campaign_name': 'blog',
        'referral_code': 'ABC123',
        'referral_url': 'https://example.com/r/ABC123',
      });

      expect(model.id, 'link-1');
      expect(model.referralUrl, 'https://example.com/r/ABC123');
    });
  });

  group('AffiliatePayoutModel.fromJson', () {
    test('parses payout with money map', () {
      final model = AffiliatePayoutModel.fromJson({
        'id': 'payout-1',
        'amount': {'amount': '40.00', 'currency': 'EUR'},
        'status': 'paid',
      });

      expect(model.amount, '40.00');
      expect(model.currency, 'EUR');
      expect(model.status, 'paid');
    });

    test('falls back to top-level currency and requested status', () {
      final model = AffiliatePayoutModel.fromJson({'id': 'payout-2'});

      expect(model.amount, '0');
      expect(model.currency, 'USD');
      expect(model.status, 'requested');
    });
  });
}
