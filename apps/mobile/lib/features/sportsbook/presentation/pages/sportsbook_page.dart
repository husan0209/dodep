import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:uuid/uuid.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/network/api_client.dart';

/// Multiplies two non-negative decimal strings using integer arithmetic and
/// returns the product trimmed to at most 8 fractional digits.
/// Used for odds/stake previews — transport values always stay strings.
String _mulDecimal(String a, String b) {
  try {
    return _mulDecimalOrThrow(a, b);
  } on FormatException {
    // Non-decimal odds formats (fractional, american) cannot be previewed
    // with integer math — the caller hides the preview instead of crashing.
    return '';
  }
}

String _mulDecimalOrThrow(String a, String b) {
  (BigInt, int) parts(String s) {
    final split = s.split('.');
    final scale = split.length == 2 ? split[1].length : 0;
    final digits = split.join();
    return (BigInt.parse(digits), scale);
  }

  final left = parts(a);
  final right = parts(b);
  final product = left.$1 * right.$1;
  var scale = left.$2 + right.$2;
  var text = product.toString();
  while (scale > 8) {
    // Truncate (never round up money previews) beyond 8 decimals.
    text = text.substring(0, text.length - 1);
    scale -= 1;
  }
  if (scale == 0) {
    return text;
  }
  text = text.padLeft(scale + 1, '0');
  final point = text.length - scale;
  var result = '${text.substring(0, point)}.${text.substring(point)}';
  while (result.endsWith('0')) {
    result = result.substring(0, result.length - 1);
  }
  if (result.endsWith('.')) {
    result = result.substring(0, result.length - 1);
  }
  return result;
}

const _decimalAmount = RegExp(r'^\d+(\.\d{1,8})?$');
const _nonZero = RegExp(r'[1-9]');

/// Sportsbook page: event list with live/prematch and sport filters,
/// decimal odds display and a local bet slip.
///
/// Event feed: `GET /api/v1/sports/events` (list under `data`/`events`/
/// `items`), odds resync per event: `GET /api/v1/sports/events/{id}/odds`.
/// Bet placement follows the web client contract `POST /api/v1/bets` with a
/// UUID `idempotency_key`; stake is sent as a decimal string.
class SportsbookPage extends StatefulWidget {
  const SportsbookPage({super.key});

  @override
  State<SportsbookPage> createState() => _SportsbookPageState();
}

class _Outcome {
  final String id;
  final String name;
  final String odds;

  const _Outcome({required this.id, required this.name, required this.odds});
}

class _Market {
  final String id;
  final String name;
  final List<_Outcome> outcomes;

  const _Market({required this.id, required this.name, required this.outcomes});
}

class _SportEvent {
  final String id;
  final String sport;
  final String league;
  final String home;
  final String away;
  final String startsAt;
  final bool live;

  final List<_Market> markets;

  const _SportEvent({
    required this.id,
    required this.sport,
    required this.league,
    required this.home,
    required this.away,
    required this.startsAt,
    required this.live,
    required this.markets,
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

  factory _SportEvent.fromJson(Map<String, dynamic> json) {
    final markets = <_Market>[];
    final rawMarkets = json['markets'];
    if (rawMarkets is List) {
      for (final raw in rawMarkets) {
        if (raw is! Map<String, dynamic>) {
          continue;
        }
        final outcomes = <_Outcome>[];
        final rawOutcomes = raw['outcomes'];
        if (rawOutcomes is List) {
          for (final o in rawOutcomes) {
            if (o is! Map<String, dynamic>) {
              continue;
            }
            final odds = _str(o, const ['odds', 'price']);
            if ('${o['outcome_id'] ?? o['id'] ?? ''}'.isEmpty || odds.isEmpty) {
              continue;
            }
            outcomes.add(_Outcome(
              id: '${o['outcome_id'] ?? o['id']}',
              name: _str(o, const ['outcome_name', 'name', 'label']),
              odds: odds,
            ));
          }
        }
        if (outcomes.isEmpty) {
          continue;
        }
        markets.add(_Market(
          id: '${raw['market_id'] ?? raw['id'] ?? ''}',
          name: _str(raw, const ['market_name', 'name']),
          outcomes: outcomes,
        ));
      }
    }
    final live = json['live'] ?? json['is_live'] ?? false;
    return _SportEvent(
      id: _str(json, const ['id', 'event_id']),
      sport: _str(json, const ['sport', 'sport_name']),
      league: _str(json, const ['league', 'league_name', 'competition']),
      home: _str(json, const ['home_team', 'home', 'team_home']),
      away: _str(json, const ['away_team', 'away', 'team_away']),
      startsAt: _str(json, const ['starts_at', 'start_time', 'kickoff']),
      live: live is bool ? live : '$live' == 'true',
      markets: markets,
    );
  }
}

class _SlipEntry {
  final String eventId;
  final String eventTitle;
  final String marketId;
  final String marketName;
  final String outcomeId;
  final String outcomeName;
  final String odds;

  const _SlipEntry({
    required this.eventId,
    required this.eventTitle,
    required this.marketId,
    required this.marketName,
    required this.outcomeId,
    required this.outcomeName,
    required this.odds,
  });
}

class _SportsbookPageState extends State<SportsbookPage> {
  List<_SportEvent> _events = const [];
  bool _liveOnly = false;
  String _sport = 'all';
  bool _loading = true;
  String? _error;

  /// One selection per event: keyed by event id.
  final Map<String, _SlipEntry> _slip = <String, _SlipEntry>{};

  @override
  void initState() {
    super.initState();
    _load();
  }

  List<String> get _sports {
    final set = <String>{};
    for (final event in _events) {
      if (event.sport.isNotEmpty) {
        set.add(event.sport);
      }
    }
    final sorted = set.toList()..sort();
    return ['all', ...sorted];
  }

  List<_SportEvent> get _filtered {
    return _events.where((event) {
      if (_liveOnly && !event.live) {
        return false;
      }
      if (_sport != 'all' && event.sport != _sport) {
        return false;
      }
      return true;
    }).toList();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final response = await getIt<ApiClient>().get(
        '/api/v1/sports/events',
        params: {'live': _liveOnly ? 'true' : 'false'},
      );
      final data = response.data;
      List<dynamic> items = const [];
      if (data is List) {
        items = data;
      } else if (data is Map<String, dynamic>) {
        for (final key in const ['data', 'events', 'items']) {
          if (data[key] is List) {
            items = data[key] as List;
            break;
          }
        }
      }
      final events = items
          .whereType<Map<String, dynamic>>()
          .map(_SportEvent.fromJson)
          .where((event) => event.id.isNotEmpty)
          .toList();
      if (!mounted) {
        return;
      }
      setState(() {
        _events = events;
        _loading = false;
      });
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

  void _toggleSelection(_SportEvent event, _Market market, _Outcome outcome) {
    setState(() {
      final existing = _slip[event.id];
      if (existing != null && existing.outcomeId == outcome.id) {
        _slip.remove(event.id);
      } else {
        _slip[event.id] = _SlipEntry(
          eventId: event.id,
          eventTitle: '${event.home} — ${event.away}',
          marketId: market.id,
          marketName: market.name,
          outcomeId: outcome.id,
          outcomeName: outcome.name,
          odds: outcome.odds,
        );
      }
    });
  }

  void _openSlip() {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => _BetSlipSheet(
        entries: _slip.values.toList(),
        onRemove: (eventId) => setState(() => _slip.remove(eventId)),
        onPlaced: (message) {
          setState(_slip.clear);
          Navigator.of(sheetContext).pop();
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(message)),
          );
        },
      ),
    );
  }

  String _apiError(DioException e) {
    final data = e.response?.data;
    if (data is Map) {
      final err = data['error'];
      if (err is Map && err['message'] is String) {
        return err['message'] as String;
      }
      if (data['message'] is String) {
        return data['message'] as String;
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
        return e.message ?? 'Failed to load events.';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Sportsbook'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loading ? null : _load,
            tooltip: 'Resync odds',
          ),
        ],
      ),
      floatingActionButton: _slip.isEmpty
          ? null
          : FloatingActionButton.extended(
              onPressed: _openSlip,
              icon: const Icon(Icons.receipt_long),
              label: Text('Slip · ${_slip.length}'),
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
                  child: _EventsView(
                    liveOnly: _liveOnly,
                    onModeChanged: (value) {
                      setState(() => _liveOnly = value);
                      _load();
                    },
                    sports: _sports,
                    sport: _sport,
                    onSportChanged: (value) =>
                        setState(() => _sport = value),
                    events: _filtered,
                    slip: _slip,
                    onToggle: _toggleSelection,
                  ),
                ),
    );
  }
}

class _EventsView extends StatelessWidget {
  final bool liveOnly;
  final ValueChanged<bool> onModeChanged;
  final List<String> sports;
  final String sport;
  final ValueChanged<String> onSportChanged;
  final List<_SportEvent> events;
  final Map<String, _SlipEntry> slip;
  final void Function(_SportEvent, _Market, _Outcome) onToggle;

  const _EventsView({
    required this.liveOnly,
    required this.onModeChanged,
    required this.sports,
    required this.sport,
    required this.onSportChanged,
    required this.events,
    required this.slip,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return CustomScrollView(
      slivers: [
        SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: SegmentedButton<bool>(
              segments: const [
                ButtonSegment(value: false, label: Text('Prematch')),
                ButtonSegment(value: true, label: Text('Live')),
              ],
              selected: {liveOnly},
              onSelectionChanged: (selected) =>
                  onModeChanged(selected.first),
            ),
          ),
        ),
        SliverToBoxAdapter(
          child: SizedBox(
            height: 48,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              itemCount: sports.length,
              separatorBuilder: (_, __) => const SizedBox(width: 8),
              itemBuilder: (context, index) {
                final value = sports[index];
                return ChoiceChip(
                  label: Text(value == 'all' ? 'All' : value),
                  selected: sport == value,
                  onSelected: (_) => onSportChanged(value),
                );
              },
            ),
          ),
        ),
        if (events.isEmpty)
          const SliverFillRemaining(
            hasScrollBody: false,
            child: Center(child: Text('No events right now.')),
          )
        else
          SliverList(
            delegate: SliverChildBuilderDelegate(
              (context, index) => _EventCard(
                event: events[index],
                slip: slip,
                onToggle: onToggle,
              ),
              childCount: events.length,
            ),
          ),
      ],
    );
  }
}

class _EventCard extends StatelessWidget {
  final _SportEvent event;
  final Map<String, _SlipEntry> slip;
  final void Function(_SportEvent, _Market, _Outcome) onToggle;

  const _EventCard({
    required this.event,
    required this.slip,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.fromLTRB(16, 8, 16, 0),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                if (event.live)
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: Colors.red,
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: const Text(
                      'LIVE',
                      style: TextStyle(color: Colors.white, fontSize: 12),
                    ),
                  ),
                if (event.live) const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    '${event.home} — ${event.away}',
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 2),
            Text(
              '${event.sport}'
              '${event.league.isEmpty ? '' : ' · ${event.league}'}'
              '${event.startsAt.isEmpty ? '' : ' · ${event.startsAt}'}',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 8),
            if (event.markets.isEmpty)
              const Text('Odds are not published for this event yet.')
            else
              for (final market in event.markets) ...[
                Text(
                  market.name.isEmpty ? 'Market' : market.name,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 4),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    for (final outcome in market.outcomes)
                      _OddsButton(
                        outcome: outcome,
                        selected:
                            slip[event.id]?.outcomeId == outcome.id,
                        onPressed: () => onToggle(event, market, outcome),
                      ),
                  ],
                ),
                const SizedBox(height: 8),
              ],
          ],
        ),
      ),
    );
  }
}

class _OddsButton extends StatelessWidget {
  final _Outcome outcome;
  final bool selected;
  final VoidCallback onPressed;

  const _OddsButton({
    required this.outcome,
    required this.selected,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onPressed,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: selected
              ? Theme.of(context).colorScheme.primaryContainer
              : Theme.of(context).colorScheme.surfaceVariant,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: selected
                ? Theme.of(context).colorScheme.primary
                : Colors.transparent,
          ),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(outcome.name.isEmpty ? '—' : outcome.name),
            Text(
              outcome.odds,
              style: const TextStyle(fontWeight: FontWeight.bold),
            ),
          ],
        ),
      ),
    );
  }
}

class _BetSlipSheet extends StatefulWidget {
  final List<_SlipEntry> entries;
  final ValueChanged<String> onRemove;
  final ValueChanged<String> onPlaced;

  const _BetSlipSheet({
    required this.entries,
    required this.onRemove,
    required this.onPlaced,
  });

  @override
  State<_BetSlipSheet> createState() => _BetSlipSheetState();
}

class _BetSlipSheetState extends State<_BetSlipSheet> {
  final TextEditingController _stake = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  String _currency = 'USD';
  bool _placing = false;
  late List<_SlipEntry> _local = [...widget.entries];

  @override
  void dispose() {
    _stake.dispose();
    super.dispose();
  }

  String get _totalOdds {
    if (_local.isEmpty) {
      return '';
    }
    var acc = '1';
    for (final entry in _local) {
      acc = _mulDecimal(acc, entry.odds);
      if (acc.isEmpty) {
        return '';
      }
    }
    return acc;
  }

  String get _potentialWin {
    final stake = _stake.text.trim();
    if (stake.isEmpty || !_decimalAmount.hasMatch(stake)) {
      return '';
    }
    return _mulDecimal(stake, _totalOdds);
  }

  Future<void> _place() async {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    setState(() {
      _placing = true;
    });
    try {
      final selections = [
        for (final entry in _local)
          {
            'event_id': int.tryParse(entry.eventId) ?? entry.eventId,
            'market_id': int.tryParse(entry.marketId) ?? entry.marketId,
            'outcome_id': int.tryParse(entry.outcomeId) ?? entry.outcomeId,
            'odds': entry.odds,
            'outcome_name': entry.outcomeName,
          },
      ];
      final response = await getIt<ApiClient>().post(
        '/api/v1/bets',
        data: {
          'bet_type': _local.length == 1 ? 'single' : 'accumulator',
          'selections': selections,
          // Decimal string — never a float (CONVENTIONS NEVER-6).
          'stake': _stake.text.trim(),
          'currency': _currency,
          'accept_odds_changes': 'any',
          'idempotency_key': const Uuid().v4(),
        },
      );
      if (!mounted) {
        return;
      }
      final body = response.data;
      final data = body is Map<String, dynamic> ? (body['data'] ?? body) : null;
      final betId = data is Map ? '${data['id'] ?? ''}' : '';
      final potential =
          data is Map ? '${data['potential_win'] ?? ''}' : '';
      widget.onPlaced(
        'Bet accepted${betId.isEmpty ? '' : ' #$betId'}'
        '${potential.isEmpty ? '' : '. Potential win: $potential'}.',
      );
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      final data = e.response?.data;
      String message = e.message ?? 'Bet was rejected.';
      if (data is Map) {
        final err = data['error'];
        if (err is Map && err['message'] is String) {
          message = err['message'] as String;
        }
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(message)),
      );
    } finally {
      if (mounted) {
        setState(() {
          _placing = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final potential = _potentialWin;
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          left: 16,
          right: 16,
          top: 16,
          bottom: MediaQuery.of(context).viewInsets.bottom + 16,
        ),
        child: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Bet slip (${_local.length})',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              Flexible(
                child: ListView.separated(
                  shrinkWrap: true,
                  itemCount: _local.length,
                  separatorBuilder: (_, __) => const Divider(height: 1),
                  itemBuilder: (context, index) {
                    final entry = _local[index];
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text(entry.eventTitle),
                      subtitle: Text(
                        '${entry.outcomeName} @ ${entry.odds}',
                      ),
                      trailing: IconButton(
                        icon: const Icon(Icons.close),
                        onPressed: () {
                          setState(() => _local.removeAt(index));
                          widget.onRemove(entry.eventId);
                        },
                      ),
                    );
                  },
                ),
              ),
              const SizedBox(height: 8),
              Text(
                'Total odds: ${_totalOdds.isEmpty ? '—' : _totalOdds}',
              ),
              if (potential.isNotEmpty)
                Text('Potential win: $potential $_currency'),
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    flex: 2,
                    child: TextFormField(
                      controller: _stake,
                      keyboardType: const TextInputType.numberWithOptions(
                        decimal: true,
                      ),
                      decoration: const InputDecoration(
                        labelText: 'Stake',
                        hintText: '0.00',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null ||
                            !_decimalAmount.hasMatch(value.trim()) ||
                            !_nonZero.hasMatch(value)) {
                          return 'Enter a positive amount.';
                        }
                        return null;
                      },
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: DropdownButtonFormField<String>(
                      value: _currency,
                      decoration: const InputDecoration(
                        labelText: 'Currency',
                        border: OutlineInputBorder(),
                      ),
                      items: const [
                        DropdownMenuItem(
                          value: 'USD',
                          child: Text('USD'),
                        ),
                        DropdownMenuItem(
                          value: 'EUR',
                          child: Text('EUR'),
                        ),
                        DropdownMenuItem(
                          value: 'RUB',
                          child: Text('RUB'),
                        ),
                      ],
                      onChanged: (value) {
                        if (value != null) {
                          setState(() => _currency = value);
                        }
                      },
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ElevatedButton(
                onPressed: _placing ? null : _place,
                child: _placing
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Place bet'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
