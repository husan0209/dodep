import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:uuid/uuid.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/network/api_client.dart';

/// Bonuses page: active bonus banner, bonus list with wagering progress,
/// claim (activate) and forfeit (cancel).
///
/// Backend contract (Go bonus-service, `services/go/bonus/routes.go`):
/// - `GET /api/v1/bonuses` → `{bonuses: [...], total}`
/// - `GET /api/v1/bonuses/active` → `{bonus: ... | null}`
/// - `GET /api/v1/bonuses/{id}/wagering` → `{required, completed,
///   remaining, progress_percentage, is_completed}`
/// - `POST /api/v1/bonuses/{id}/activate` and `POST /api/v1/bonuses/{id}/cancel`
///   require an `X-Idempotency-Key` UUID header.
/// user_id is always taken from the JWT — never sent from the client.
class BonusesPage extends StatefulWidget {
  const BonusesPage({super.key});

  @override
  State<BonusesPage> createState() => _BonusesPageState();
}

class _Bonus {
  final String id;
  final String type;
  final String status;
  final String amount;
  final String currency;
  final String wageringRequired;
  final String wageringCompleted;
  final int multiplier;
  final String expiresAt;

  const _Bonus({
    required this.id,
    required this.type,
    required this.status,
    required this.amount,
    required this.currency,
    required this.wageringRequired,
    required this.wageringCompleted,
    required this.multiplier,
    required this.expiresAt,
  });

  static String _str(Map<String, dynamic> json, List<String> keys) {
    for (final key in keys) {
      final value = json[key];
      if (value != null && '$value'.isNotEmpty) {
        return '$value';
      }
    }
    return '';
  }

  factory _Bonus.fromJson(Map<String, dynamic> json) {
    return _Bonus(
      id: _str(json, const ['ID', 'id', 'bonus_id']),
      type: _str(json, const ['Type', 'type', 'bonus_type']),
      status: _str(json, const ['Status', 'status']),
      amount: _str(json, const ['BonusAmount', 'bonus_amount', 'amount']),
      currency: _str(json, const ['Currency', 'currency']),
      wageringRequired: _str(
        json,
        const ['WageringRequired', 'wagering_required', 'required'],
      ),
      wageringCompleted: _str(
        json,
        const ['WageringCompleted', 'wagering_completed', 'completed'],
      ),
      multiplier: int.tryParse(
            _str(json, const ['WageringMultiplier', 'wagering_multiplier']),
          ) ??
          0,
      expiresAt: _str(json, const ['ExpiresAt', 'expires_at']),
    );
  }

  bool get canClaim => status == 'pending';
  bool get canForfeit => status == 'pending' || status == 'active';
}

class _Wagering {
  final String required;
  final String completed;
  final String remaining;
  final double progress;
  final bool done;

  const _Wagering({
    required this.required,
    required this.completed,
    required this.remaining,
    required this.progress,
    required this.done,
  });

  factory _Wagering.fromJson(Map<String, dynamic> json) {
    double asDouble(dynamic value) {
      if (value is num) {
        return value.toDouble();
      }
      return double.tryParse('$value') ?? 0;
    }

    return _Wagering(
      required: '${json['required'] ?? '0'}',
      completed: '${json['completed'] ?? '0'}',
      remaining: '${json['remaining'] ?? '0'}',
      progress: (asDouble(json['progress_percentage']) / 100).clamp(0, 1),
      done: json['is_completed'] == true,
    );
  }
}

class _BonusesPageState extends State<BonusesPage> {
  List<_Bonus> _bonuses = const [];
  _Bonus? _active;
  final Map<String, _Wagering> _wagering = <String, _Wagering>{};
  bool _loading = true;
  String? _error;
  final Set<String> _busy = <String>{};

  @override
  void initState() {
    super.initState();
    _load();
  }

  List<dynamic> _asList(dynamic data) {
    if (data is List) {
      return data;
    }
    if (data is Map<String, dynamic>) {
      for (final key in const ['bonuses', 'data', 'items']) {
        if (data[key] is List) {
          return data[key] as List;
        }
      }
    }
    return const [];
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = getIt<ApiClient>();
      final listResponse = await api.get('/api/v1/bonuses');
      _Bonus? active;
      try {
        final activeResponse = await api.get('/api/v1/bonuses/active');
        final body = activeResponse.data;
        Map<String, dynamic>? bonusJson;
        if (body is Map<String, dynamic>) {
          final bonus = body['bonus'] ?? body['data'];
          if (bonus is Map<String, dynamic>) {
            bonusJson = bonus;
          } else if (body.containsKey('ID') || body.containsKey('id')) {
            bonusJson = body;
          }
        }
        if (bonusJson != null) {
          active = _Bonus.fromJson(bonusJson);
        }
      } on DioException {
        // Active bonus is a nice-to-have banner; the list is authoritative.
        active = null;
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _bonuses = _asList(listResponse.data)
            .whereType<Map<String, dynamic>>()
            .map(_Bonus.fromJson)
            .where((bonus) => bonus.id.isNotEmpty)
            .toList();
        _active = active;
        _loading = false;
      });
      _refreshWagering();
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = _apiError(e);
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = '$e';
      });
    }
  }

  Future<void> _refreshWagering() async {
    for (final bonus in _bonuses) {
      if (bonus.status != 'active') {
        continue;
      }
      try {
        final response = await getIt<ApiClient>().get(
          '/api/v1/bonuses/${bonus.id}/wagering',
        );
        final body = response.data;
        if (body is Map<String, dynamic> && mounted) {
          setState(() {
            _wagering[bonus.id] = _Wagering.fromJson(body);
          });
        }
      } on DioException {
        // Progress is supplementary; ignore per-item failures.
        continue;
      }
    }
  }

  Future<void> _mutate(_Bonus bonus, String action) async {
    if (_busy.contains(bonus.id)) {
      return;
    }
    setState(() {
      _busy.add(bonus.id);
    });
    try {
      await getIt<ApiClient>().dio.post(
            '/api/v1/bonuses/${bonus.id}/$action',
            options: Options(
              headers: {
                'X-Idempotency-Key': const Uuid().v4(),
              },
            ),
          );
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            action == 'activate'
                ? 'Bonus claimed.'
                : 'Bonus forfeited. Remaining wagering is cancelled.',
          ),
        ),
      );
      await _load();
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(_apiError(e))),
      );
    } finally {
      if (mounted) {
        setState(() {
          _busy.remove(bonus.id);
        });
      }
    }
  }

  String _apiError(DioException e) {
    final data = e.response?.data;
    if (data is Map) {
      if (data['error'] is String) {
        return data['error'] as String;
      }
      final err = data['error'];
      if (err is Map && err['message'] is String) {
        return err['message'] as String;
      }
    }
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return 'Request timed out. Check your connection and retry.';
      case DioExceptionType.connectionError:
        return 'No connection to the server.';
      default:
        return e.message ?? 'Failed to load bonuses.';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Bonuses'),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.cloud_off, size: 48),
                        const SizedBox(height: 12),
                        Text(_error!, textAlign: TextAlign.center),
                        const SizedBox(height: 12),
                        ElevatedButton(
                          onPressed: _load,
                          child: const Text('Retry'),
                        ),
                      ],
                    ),
                  ),
                )
              : RefreshIndicator(
                  onRefresh: _load,
                  child: _bonuses.isEmpty && _active == null
                      ? ListView(
                          children: const [
                            SizedBox(height: 120),
                            Center(
                              child: Text('No bonuses available right now.'),
                            ),
                          ],
                        )
                      : ListView(
                          padding: const EdgeInsets.all(16),
                          children: [
                            if (_active != null) ...[
                              _ActiveBanner(bonus: _active!),
                              const SizedBox(height: 12),
                            ],
                            for (final bonus in _bonuses)
                              _BonusCard(
                                bonus: bonus,
                                wagering: _wagering[bonus.id],
                                busy: _busy.contains(bonus.id),
                                onClaim: bonus.canClaim
                                    ? () => _mutate(bonus, 'activate')
                                    : null,
                                onForfeit: bonus.canForfeit
                                    ? () => _mutate(bonus, 'cancel')
                                    : null,
                              ),
                          ],
                        ),
                ),
    );
  }
}

class _ActiveBanner extends StatelessWidget {
  final _Bonus bonus;

  const _ActiveBanner({required this.bonus});

  @override
  Widget build(BuildContext context) {
    return Card(
      color: Theme.of(context).colorScheme.primaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Active bonus',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 4),
            Text('${bonus.amount} ${bonus.currency} · ${bonus.type}'),
          ],
        ),
      ),
    );
  }
}

class _BonusCard extends StatelessWidget {
  final _Bonus bonus;
  final _Wagering? wagering;
  final bool busy;
  final VoidCallback? onClaim;
  final VoidCallback? onForfeit;

  const _BonusCard({
    required this.bonus,
    required this.wagering,
    required this.busy,
    required this.onClaim,
    required this.onForfeit,
  });

  Color _statusColor(String status) {
    return switch (status) {
      'active' => Colors.green,
      'pending' => Colors.orange,
      'completed' => Colors.blue,
      'expired' || 'cancelled' => Colors.grey,
      _ => Colors.grey,
    };
  }

  @override
  Widget build(BuildContext context) {
    final progress = wagering;
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    '${bonus.type} · ${bonus.amount} ${bonus.currency}',
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                ),
                Chip(
                  label: Text(bonus.status),
                  backgroundColor: Color.lerp(
                    _statusColor(bonus.status),
                    Colors.white,
                    0.85,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              'Wagering: ${bonus.wageringCompleted} / '
              '${bonus.wageringRequired}'
              '${bonus.multiplier > 0 ? ' (x${bonus.multiplier})' : ''}',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            if (bonus.expiresAt.isNotEmpty)
              Text(
                'Expires: ${bonus.expiresAt}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            if (progress != null) ...[
              const SizedBox(height: 8),
              LinearProgressIndicator(value: progress.progress),
              const SizedBox(height: 4),
              Text(
                progress.done
                    ? 'Wagering complete — bonus converts to real balance.'
                    : 'Remaining: ${progress.remaining}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
            if (onClaim != null || onForfeit != null) ...[
              const SizedBox(height: 12),
              Row(
                children: [
                  if (onClaim != null)
                    Expanded(
                      child: ElevatedButton(
                        onPressed: busy ? null : onClaim,
                        child: busy
                            ? const SizedBox(
                                height: 18,
                                width: 18,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : const Text('Claim'),
                      ),
                    ),
                  if (onClaim != null && onForfeit != null)
                    const SizedBox(width: 8),
                  if (onForfeit != null)
                    Expanded(
                      child: OutlinedButton(
                        onPressed: busy ? null : onForfeit,
                        child: const Text('Forfeit'),
                      ),
                    ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}
