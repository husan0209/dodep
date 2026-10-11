import '../../domain/entities/rg_limits.dart';

/// Unwraps the RG service envelope: {"data": {...}, "meta": {...}}.
Map<String, dynamic> _data(Map<String, dynamic> json) =>
    json['data'] is Map<String, dynamic>
        ? json['data'] as Map<String, dynamic>
        : json;

String _amount(dynamic v) => v == null ? '' : v.toString();

List<Map<String, dynamic>> _list(dynamic v) =>
    v is List ? v.whereType<Map<String, dynamic>>().toList() : const [];

DateTime? _date(dynamic v) {
  if (v == null) return null;
  if (v is DateTime) return v;
  final s = v.toString();
  if (s.isEmpty) return null;
  return DateTime.tryParse(s);
}

/// Parses `GET /api/v1/rg/limits` and `PUT /api/v1/rg/limits`:
/// `{limits: {...}, pending: [{...}]}`.
class RGLimitsEnvelopeModel extends RGLimits {
  const RGLimitsEnvelopeModel({
    super.depositDaily,
    super.depositWeekly,
    super.depositMonthly,
    super.lossDaily,
    super.lossWeekly,
    super.lossMonthly,
    super.wagerDaily,
    super.wagerWeekly,
    super.sessionMinutes,
    super.realityCheckMinutes,
    super.pending,
  });

  factory RGLimitsEnvelopeModel.fromJson(Map<String, dynamic> json) {
    final d = _data(json);
    final l = d['limits'] is Map<String, dynamic>
        ? d['limits'] as Map<String, dynamic>
        : d;
    int asInt(dynamic v) => v is int ? v : int.tryParse('${v ?? ''}') ?? 0;

    return RGLimitsEnvelopeModel(
      depositDaily: MoneyCap(_amount(l['deposit_daily'])),
      depositWeekly: MoneyCap(_amount(l['deposit_weekly'])),
      depositMonthly: MoneyCap(_amount(l['deposit_monthly'])),
      lossDaily: MoneyCap(_amount(l['loss_daily'])),
      lossWeekly: MoneyCap(_amount(l['loss_weekly'])),
      lossMonthly: MoneyCap(_amount(l['loss_monthly'])),
      wagerDaily: MoneyCap(_amount(l['wager_daily'])),
      wagerWeekly: MoneyCap(_amount(l['wager_weekly'])),
      sessionMinutes: asInt(l['session_minutes']),
      realityCheckMinutes: asInt(l['reality_check_minutes']),
      pending:
          _list(d['pending']).map(PendingLimitChangeModel.fromJson).toList(),
    );
  }
}

class PendingLimitChangeModel extends PendingLimitChange {
  const PendingLimitChangeModel({
    required super.id,
    required super.limitType,
    required super.oldValue,
    required super.newValue,
    required super.effectiveAt,
  });

  factory PendingLimitChangeModel.fromJson(Map<String, dynamic> json) {
    return PendingLimitChangeModel(
      id: '${json['id'] ?? ''}',
      limitType: '${json['limit_type'] ?? ''}',
      oldValue: _amount(json['old_value']),
      newValue: _amount(json['new_value']),
      effectiveAt:
          _date(json['effective_at']) ?? DateTime.fromMillisecondsSinceEpoch(0),
    );
  }
}

class RGExclusionModel extends RGExclusion {
  const RGExclusionModel({
    required super.id,
    required super.type,
    required super.status,
    required super.until,
    required super.permanent,
  });

  factory RGExclusionModel.fromJson(Map<String, dynamic> json) {
    return RGExclusionModel(
      id: '${json['id'] ?? ''}',
      type: '${json['type'] ?? ''}',
      status: '${json['status'] ?? ''}',
      until: _date(json['until']),
      permanent: (json['permanent'] ?? false) as bool,
    );
  }
}

/// Parses `GET /api/v1/rg/status`.
class RGStatusModel extends RGStatus {
  const RGStatusModel({
    required super.gamblingAllowed,
    required super.blockedReason,
    required super.limits,
    super.exclusion,
  });

  factory RGStatusModel.fromJson(Map<String, dynamic> json) {
    final d = _data(json);
    final limits = RGLimitsEnvelopeModel.fromJson(d);
    final e = d['exclusion'];
    return RGStatusModel(
      gamblingAllowed: (d['gambling_allowed'] ?? false) as bool,
      blockedReason: '${d['blocked_reason'] ?? ''}',
      limits: limits,
      exclusion:
          e is Map<String, dynamic> ? RGExclusionModel.fromJson(e) : null,
    );
  }
}

/// Request body for `PUT /api/v1/rg/limits`.
/// Field names must match the service exactly; omitted keys mean
/// "no change" (the service treats nil as no change).
class SetRGLimitsBody {
  final Map<String, String?> money;
  final int? sessionMinutes;
  final int? realityCheckMinutes;

  const SetRGLimitsBody({
    this.money = const {},
    this.sessionMinutes,
    this.realityCheckMinutes,
  });

  /// Null when nothing would change, so the caller can skip the request.
  Map<String, dynamic>? toJson() {
    final out = <String, dynamic>{};
    for (final e in money.entries) {
      final v = e.value?.trim();
      if (v != null && v.isNotEmpty) out[e.key] = v;
    }
    if (sessionMinutes != null) out['session_minutes'] = sessionMinutes;
    if (realityCheckMinutes != null) {
      out['reality_check_minutes'] = realityCheckMinutes;
    }
    return out.isEmpty ? null : out;
  }
}

/// Request body for `POST /api/v1/rg/self-exclusion`.
class SelfExclusionBody {
  final String period;

  const SelfExclusionBody(this.period);

  Map<String, dynamic> toJson() => {'period': period};
}

/// Request body for `POST /api/v1/rg/self-exclusion/revoke`.
class RevokeSelfExclusionBody {
  const RevokeSelfExclusionBody();

  Map<String, dynamic> toJson() => {'confirm': true};
}
