import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/di/injection.dart';
import '../../domain/entities/rg_limits.dart';
import '../../domain/repositories/rg_repository.dart';
import '../bloc/rg_bloc.dart';

const _limitFields = <String, String>{
  'deposit_daily': 'Депозит / день',
  'deposit_weekly': 'Депозит / неделя',
  'deposit_monthly': 'Депозит / месяц',
  'wager_daily': 'Ставка / день',
  'wager_weekly': 'Ставка / неделя',
  'loss_daily': 'Проигрыш / день',
  'loss_weekly': 'Проигрыш / неделя',
  'loss_monthly': 'Проигрыш / месяц',
};

const _timeoutPeriods = <String, String>{
  '24h': '24 часа',
  '48h': '48 часов',
  '7d': '7 дней',
  '30d': '30 дней',
};

/// Responsible-gambling cabinet. Talks to the RG service through
/// [RGRepository]; the player identity comes from the JWT, never the UI.
class RGPage extends StatelessWidget {
  const RGPage({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) => RGBloc(repository: getIt<RGRepository>())
        ..add(const RGLoadRequested()),
      child: const _RGView(),
    );
  }
}

class _RGView extends StatefulWidget {
  const _RGView();

  @override
  State<_RGView> createState() => _RGViewState();
}

class _RGViewState extends State<_RGView> {
  final Map<String, TextEditingController> _money = {
    for (final k in _limitFields.keys) k: TextEditingController(),
  };
  final TextEditingController _session = TextEditingController();
  final TextEditingController _reality = TextEditingController();
  SelfExclusionPeriod _period = RGPolicy.periods[3];
  String _timeout = '24h';
  bool _prefilled = false;
  bool _understood = false;

  @override
  void dispose() {
    for (final c in _money.values) {
      c.dispose();
    }
    _session.dispose();
    _reality.dispose();
    super.dispose();
  }

  void _prefill(RGLimits l) {
    if (_prefilled) return;
    _prefilled = true;
    final values = <String, MoneyCap>{
      'deposit_daily': l.depositDaily,
      'deposit_weekly': l.depositWeekly,
      'deposit_monthly': l.depositMonthly,
      'wager_daily': l.wagerDaily,
      'wager_weekly': l.wagerWeekly,
      'loss_daily': l.lossDaily,
      'loss_weekly': l.lossWeekly,
      'loss_monthly': l.lossMonthly,
    };
    for (final e in values.entries) {
      if (e.value.isSet) _money[e.key]!.text = e.value.amount;
    }
    if (l.sessionMinutes > 0) _session.text = '${l.sessionMinutes}';
    if (l.realityCheckMinutes > 0) {
      _reality.text = '${l.realityCheckMinutes}';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Ответственная игра')),
      body: BlocConsumer<RGBloc, RGState>(
        listener: (context, state) {
          final msg = state is RGLoaded
              ? state.notice
              : state is RGError
                  ? state.message
                  : null;
          if (msg != null && state is! RGLoading) {
            ScaffoldMessenger.of(context)
                .showSnackBar(SnackBar(content: Text(msg)));
          }
        },
        builder: (context, state) {
          if (state is RGLoading || state is RGInitial) {
            return const Center(child: CircularProgressIndicator());
          }
          if (state is RGError && state.previous == null) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(state.message),
                  const SizedBox(height: 12),
                  ElevatedButton(
                    onPressed: () =>
                        context.read<RGBloc>().add(const RGLoadRequested()),
                    child: const Text('Повторить'),
                  ),
                ],
              ),
            );
          }
          final status =
              state is RGLoaded ? state.status : (state as RGError).previous!;
          _prefill(status.limits);
          return _body(context, state, status);
        },
      ),
    );
  }

  Widget _body(BuildContext context, RGState state, RGStatus status) {
    final saving = state is RGLoaded && state.saving;
    final exclusion = status.exclusion;
    final blocked = exclusion?.blocksGambling(DateTime.now()) ??
        (status.isSelfExcluded && !status.gamblingAllowed);
    final limits = status.limits;

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        if (blocked)
          _Banner(
            color: Colors.red,
            title: 'Игра недоступна',
            body: exclusion == null
                ? 'Действует ограничение RG (${status.blockedReason}). '
                    'Вывод средств остаётся доступен.'
                : exclusion.permanent || exclusion.until == null
                    ? 'Самоисключение активно бессрочно. '
                        'Вывод средств остаётся доступен.'
                    : 'Самоисключение активно до '
                        '${exclusion.until!.toLocal()}. '
                        'Вывод средств остаётся доступен.',
          ),
        if (!blocked && !status.gamblingAllowed)
          _Banner(
            color: Colors.amber,
            title: 'Есть активное ограничение',
            body: '${status.blockedReason}. Вывод средств остаётся доступен.',
          ),
        if (exclusion != null && exclusion.isRevocable)
          OutlinedButton(
            onPressed: saving
                ? null
                : () => context
                    .read<RGBloc>()
                    .add(const RGRevokeSelfExclusionRequested()),
            child: const Text('Снять истёкшее самоисключение'),
          ),
        const SizedBox(height: 16),

        // Cooling / pending increases
        if (limits.pending.isNotEmpty) ...[
          _Banner(
            color: Colors.blue,
            title: 'Повышения в cooling-периоде',
            body: limits.pending
                .map((p) =>
                    '${_label(p.limitType)}: ${p.oldValue} → ${p.newValue} '
                    '(с ${p.effectiveAt.toLocal()})')
                .join('\n'),
          ),
          const SizedBox(height: 16),
        ],

        const Text('Понижение лимита действует сразу, повышение — после '
            'cooling-периода. Вывод всегда доступен.'),
        const SizedBox(height: 12),
        for (final e in _limitFields.entries)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: TextField(
              controller: _money[e.key],
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
              decoration: InputDecoration(
                labelText: e.value,
                border: const OutlineInputBorder(),
              ),
            ),
          ),
        Row(
          children: [
            Expanded(
              child: TextField(
                controller: _session,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Сессия, мин',
                  border: OutlineInputBorder(),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: TextField(
                controller: _reality,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Reality check, мин',
                  border: OutlineInputBorder(),
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        ElevatedButton(
          onPressed: saving ? null : _saveLimits,
          child: saving
              ? const SizedBox(
                  height: 20,
                  width: 20,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Text('Сохранить лимиты'),
        ),
        const SizedBox(height: 24),

        const Text('Пауза (cooling-off)',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          children: [
            for (final e in _timeoutPeriods.entries)
              ChoiceChip(
                label: Text(e.value),
                selected: _timeout == e.key,
                onSelected: (_) => setState(() => _timeout = e.key),
              ),
          ],
        ),
        const SizedBox(height: 8),
        OutlinedButton(
          onPressed: saving
              ? null
              : () =>
                  context.read<RGBloc>().add(RGStartTimeoutRequested(_timeout)),
          child: const Text('Включить паузу'),
        ),
        const SizedBox(height: 24),

        const Text('Самоисключение',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final p in RGPolicy.periods)
              ChoiceChip(
                label: Text(p.label),
                selected: _period == p,
                onSelected: (_) => setState(() => _period = p),
              ),
          ],
        ),
        CheckboxListTile(
          value: _understood,
          onChanged: (v) => setState(() => _understood = v ?? false),
          title: const Text('Я понимаю, что не смогу играть до конца срока. '
              'Вывод останется доступен.'),
          controlAffinity: ListTileControlAffinity.leading,
          contentPadding: EdgeInsets.zero,
        ),
        OutlinedButton(
          onPressed: saving || !_understood
              ? null
              : () => context
                  .read<RGBloc>()
                  .add(RGSelfExcludeRequested(_period.code)),
          style: OutlinedButton.styleFrom(foregroundColor: Colors.red),
          child: const Text('Активировать самоисключение'),
        ),
      ],
    );
  }

  void _saveLimits() {
    final money = <String, String?>{
      for (final k in _limitFields.keys)
        k: _money[k]!.text.trim().isEmpty ? null : _money[k]!.text.trim(),
    };
    context.read<RGBloc>().add(RGSaveLimitsRequested(
          money: money,
          sessionMinutes: int.tryParse(_session.text.trim()),
          realityCheckMinutes: int.tryParse(_reality.text.trim()),
        ));
  }

  static String _label(String limitType) =>
      _limitFields[limitType] ?? limitType;
}

class _Banner extends StatelessWidget {
  final Color color;
  final String title;
  final String body;

  const _Banner({
    required this.color,
    required this.title,
    required this.body,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Color.lerp(color, Colors.white, 0.9),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title,
              style: TextStyle(
                fontWeight: FontWeight.bold,
                color: Color.lerp(color, Colors.black, 0.88),
              )),
          const SizedBox(height: 4),
          Text(body,
              style: TextStyle(color: Color.lerp(color, Colors.black, 0.88))),
        ],
      ),
    );
  }
}
