import 'package:equatable/equatable.dart';

/// Regulatory policy surfaced by the RG cabinet.
///
/// Grounding mirrors the service contract (libs/proto/rg/v1/rg.proto,
/// docs/api/rg.md):
/// - Decreasing a limit takes effect immediately.
/// - Increasing a limit enters a cooling period (server returns
///   `pending[]` with `effective_at`).
/// - Self-exclusion blocks gambling, never withdrawals.
/// - 6 months is the minimum period offered for multi-operator (GAMSTOP)
///   synchronisation; permanent is never revocable.
class RGPolicy {
  static const String defaultRealityCheckMinutes = '60';

  static const List<SelfExclusionPeriod> periods = [
    SelfExclusionPeriod(
        code: '24h', label: '24 часа', duration: Duration(hours: 24)),
    SelfExclusionPeriod(
        code: '7d', label: '7 дней', duration: Duration(days: 7)),
    SelfExclusionPeriod(
        code: '30d', label: '30 дней', duration: Duration(days: 30)),
    SelfExclusionPeriod(
        code: '6m', label: '6 месяцев', duration: Duration(days: 180)),
    SelfExclusionPeriod(
        code: '1y', label: '1 год', duration: Duration(days: 365)),
    SelfExclusionPeriod(code: 'permanent', label: 'Навсегда', duration: null),
  ];

  static SelfExclusionPeriod periodForCode(String code) => periods.firstWhere(
        (p) => p.code == code,
        orElse: () => periods[3],
      );
}

/// One self-exclusion option. `code` is the wire value accepted by
/// `POST /api/v1/rg/self-exclusion`.
class SelfExclusionPeriod extends Equatable {
  final String code;
  final String label;
  final Duration? duration;

  const SelfExclusionPeriod({
    required this.code,
    required this.label,
    required this.duration,
  });

  @override
  List<Object?> get props => [code, label, duration];
}

/// Monetary limit. Amounts stay decimal strings end-to-end (NEVER-6).
class MoneyCap extends Equatable {
  final String amount;

  const MoneyCap(this.amount);

  bool get isSet => amount.isNotEmpty && amount != '0';

  @override
  List<Object?> get props => [amount];
}

/// A limit increase waiting out its cooling period.
class PendingLimitChange extends Equatable {
  final String id;
  final String limitType;
  final String oldValue;
  final String newValue;
  final DateTime effectiveAt;

  const PendingLimitChange({
    required this.id,
    required this.limitType,
    required this.oldValue,
    required this.newValue,
    required this.effectiveAt,
  });

  bool isDue(DateTime now) => !effectiveAt.isAfter(now);

  @override
  List<Object?> get props => [id, limitType, oldValue, newValue, effectiveAt];
}

/// Self-exclusion record from the RG service.
class RGExclusion extends Equatable {
  final String id;
  final String type; // self | operator | regulatory
  final String status; // active | expired | revoked
  final DateTime? until; // null = permanent
  final bool permanent;

  const RGExclusion({
    required this.id,
    required this.type,
    required this.status,
    required this.until,
    required this.permanent,
  });

  bool blocksGambling(DateTime now) {
    if (status != 'active') return false;
    if (permanent || until == null) return true;
    return until!.isAfter(now);
  }

  /// A temporary exclusion can only be lifted once it has expired.
  bool get isRevocable => status == 'expired';

  RGExclusion copyWith({String? status}) => RGExclusion(
        id: id,
        type: type,
        status: status ?? this.status,
        until: until,
        permanent: permanent,
      );

  @override
  List<Object?> get props => [id, type, status, until, permanent];
}

/// Full RG control set as returned by `GET /api/v1/rg/limits`.
class RGLimits extends Equatable {
  final MoneyCap depositDaily;
  final MoneyCap depositWeekly;
  final MoneyCap depositMonthly;
  final MoneyCap lossDaily;
  final MoneyCap lossWeekly;
  final MoneyCap lossMonthly;
  final MoneyCap wagerDaily;
  final MoneyCap wagerWeekly;
  final int sessionMinutes;
  final int realityCheckMinutes;
  final List<PendingLimitChange> pending;

  const RGLimits({
    this.depositDaily = const MoneyCap(''),
    this.depositWeekly = const MoneyCap(''),
    this.depositMonthly = const MoneyCap(''),
    this.lossDaily = const MoneyCap(''),
    this.lossWeekly = const MoneyCap(''),
    this.lossMonthly = const MoneyCap(''),
    this.wagerDaily = const MoneyCap(''),
    this.wagerWeekly = const MoneyCap(''),
    this.sessionMinutes = 0,
    this.realityCheckMinutes = 0,
    this.pending = const [],
  });

  /// Increases already past their cooling date but not yet applied
  /// (the `apply-due` scheduler has not run).
  List<PendingLimitChange> dueChanges(DateTime now) =>
      pending.where((p) => p.isDue(now)).toList();

  List<PendingLimitChange> coolingChanges(DateTime now) =>
      pending.where((p) => !p.isDue(now)).toList();

  @override
  List<Object?> get props => [
        depositDaily,
        depositWeekly,
        depositMonthly,
        lossDaily,
        lossWeekly,
        lossMonthly,
        wagerDaily,
        wagerWeekly,
        sessionMinutes,
        realityCheckMinutes,
        pending,
      ];
}

/// Aggregate protection posture from `GET /api/v1/rg/status`.
class RGStatus extends Equatable {
  final bool gamblingAllowed;
  final String blockedReason;
  final RGLimits limits;
  final RGExclusion? exclusion;

  const RGStatus({
    required this.gamblingAllowed,
    required this.blockedReason,
    required this.limits,
    this.exclusion,
  });

  bool get isSelfExcluded =>
      blockedReason == 'RG_SELF_EXCLUDED' || exclusion != null;

  RGStatus copyWith({RGLimits? limits}) => RGStatus(
        gamblingAllowed: gamblingAllowed,
        blockedReason: blockedReason,
        limits: limits ?? this.limits,
        exclusion: exclusion,
      );

  @override
  List<Object?> get props =>
      [gamblingAllowed, blockedReason, limits, exclusion];
}
