import 'package:flutter_test/flutter_test.dart';

import 'package:dod_mobile/features/responsible_gambling/data/models/rg_limits_model.dart';
import 'package:dod_mobile/features/responsible_gambling/domain/entities/rg_limits.dart';

void main() {
  group('RGLimitsEnvelopeModel.fromJson', () {
    test('parses service envelope with limits and pending', () {
      final model = RGLimitsEnvelopeModel.fromJson(const {
        'data': {
          'limits': {
            'deposit_daily': '100.00',
            'deposit_weekly': '500',
            'loss_daily': '25',
            'session_minutes': 60,
            'reality_check_minutes': 30,
          },
          'pending': [
            {
              'id': 'p1',
              'limit_type': 'deposit_daily',
              'old_value': '100',
              'new_value': '300',
              'effective_at': '2030-01-02T10:00:00Z',
            },
          ],
        },
        'meta': {'request_id': 'req_1'},
      });

      expect(model.depositDaily.amount, '100.00');
      expect(model.depositWeekly.amount, '500');
      expect(model.lossDaily.isSet, isTrue);
      expect(model.wagerDaily.isSet, isFalse);
      expect(model.sessionMinutes, 60);
      expect(model.realityCheckMinutes, 30);
      expect(model.pending.length, 1);
      expect(model.pending.first.newValue, '300');
      expect(
        model.dueChanges(DateTime.parse('2030-01-03T00:00:00Z')).length,
        1,
      );
      expect(
        model.coolingChanges(DateTime.parse('2029-01-01T00:00:00Z')).length,
        1,
      );
    });

    test('treats bare limits object (no envelope) as valid', () {
      final model = RGLimitsEnvelopeModel.fromJson(const {
        'deposit_daily': '50',
        'pending': <dynamic>[],
      });
      expect(model.depositDaily.amount, '50');
      expect(model.pending, isEmpty);
    });

    test('zero amount is reported as unset', () {
      final model = RGLimitsEnvelopeModel.fromJson(const {
        'limits': {'deposit_daily': '0'},
      });
      expect(model.depositDaily.amount, '0');
      expect(model.depositDaily.isSet, isFalse);
    });
  });

  group('RGStatusModel.fromJson', () {
    test('parses blocked status with active exclusion', () {
      final model = RGStatusModel.fromJson(const {
        'data': {
          'gambling_allowed': false,
          'blocked_reason': 'RG_SELF_EXCLUDED',
          'limits': <String, dynamic>{},
          'exclusion': {
            'id': 'e1',
            'type': 'self',
            'status': 'active',
            'until': '2030-01-01T00:00:00Z',
            'permanent': false,
          },
        },
      });

      expect(model.gamblingAllowed, isFalse);
      expect(model.blockedReason, 'RG_SELF_EXCLUDED');
      expect(model.isSelfExcluded, isTrue);
      expect(model.exclusion!.blocksGambling(DateTime.now()), isTrue);
      expect(model.exclusion!.isRevocable, isFalse);
    });

    test('permanent exclusion blocks forever and is never revocable', () {
      final model = RGStatusModel.fromJson(const {
        'data': {
          'gambling_allowed': false,
          'blocked_reason': 'RG_SELF_EXCLUDED',
          'exclusion': {
            'id': 'e2',
            'type': 'self',
            'status': 'active',
            'permanent': true,
          },
        },
      });
      expect(model.exclusion!.blocksGambling(DateTime.now()), isTrue);
      expect(model.exclusion!.isRevocable, isFalse);
    });

    test('expired exclusion is revocable and does not block', () {
      final model = RGStatusModel.fromJson(const {
        'data': {
          'gambling_allowed': true,
          'blocked_reason': '',
          'exclusion': {
            'id': 'e3',
            'status': 'expired',
            'until': '2020-01-01T00:00:00Z',
            'permanent': false,
          },
        },
      });
      expect(model.exclusion!.blocksGambling(DateTime.now()), isFalse);
      expect(model.exclusion!.isRevocable, isTrue);
    });
  });

  group('request bodies', () {
    test('SetRGLimitsBody omits blanks and null when empty', () {
      expect(const SetRGLimitsBody().toJson(), isNull);
      expect(const SetRGLimitsBody(money: {'deposit_daily': ' '}).toJson(),
          isNull);
    });

    test('SetRGLimitsBody serializes service field names', () {
      final json = const SetRGLimitsBody(
        money: {'deposit_daily': '100.00', 'loss_weekly': ''},
        sessionMinutes: 90,
        realityCheckMinutes: 30,
      ).toJson();
      expect(json?['deposit_daily'], '100.00');
      expect(json!.containsKey('loss_weekly'), isFalse);
      expect(json['session_minutes'], 90);
      expect(json['reality_check_minutes'], 30);
    });

    test('SelfExclusionBody and revoke body', () {
      expect(const SelfExclusionBody('6m').toJson(), {'period': '6m'});
      expect(const RevokeSelfExclusionBody().toJson(), {'confirm': true});
    });
  });

  group('RGPolicy', () {
    test('offers 6m minimum for multi-operator sync plus permanent', () {
      final codes = RGPolicy.periods.map((p) => p.code).toList();
      expect(
          codes, containsAllInOrder(<String>['24h', '7d', '30d', '6m', '1y']));
      expect(codes.last, 'permanent');
    });

    test('periodForCode falls back to 6m for unknown codes', () {
      expect(RGPolicy.periodForCode('permanent').code, 'permanent');
      expect(RGPolicy.periodForCode('nonsense').code, '6m');
    });
  });
}
