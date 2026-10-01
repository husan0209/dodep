import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/di/injection.dart';
import '../../domain/repositories/affiliate_repository.dart';
import '../bloc/affiliate_bloc.dart';

/// Affiliate cabinet page: earnings (revshare from NGR), funnel, links, payouts.
/// Data comes from the Go affiliate-service; no hardcoded money values.
class AffiliatePage extends StatelessWidget {
  const AffiliatePage({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) => AffiliateBloc(
        repository: getIt<AffiliateRepository>(),
      )..add(const AffiliateLoadRequested()),
      child: Scaffold(
        appBar: AppBar(title: const Text('Партнёрская программа')),
        body: BlocBuilder<AffiliateBloc, AffiliateState>(
          builder: (context, state) {
            if (state is AffiliateLoading || state is AffiliateInitial) {
              return const Center(child: CircularProgressIndicator());
            }
            if (state is AffiliateError) {
              return Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(state.message),
                    const SizedBox(height: 12),
                    ElevatedButton(
                      onPressed: () => context
                          .read<AffiliateBloc>()
                          .add(const AffiliateLoadRequested()),
                      child: const Text('Повторить'),
                    ),
                  ],
                ),
              );
            }
            if (state is AffiliateLoaded) {
              return RefreshIndicator(
                onRefresh: () async => context
                    .read<AffiliateBloc>()
                    .add(const AffiliateLoadRequested()),
                child: ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    _EarningsCard(
                      title: 'Доход сегодня',
                      amount: state.dashboard.earningsToday,
                      currency: state.dashboard.currency,
                    ),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        Expanded(
                          child: _MetricTile(
                            label: 'Pending',
                            value:
                                '${state.dashboard.pendingAmount} ${state.dashboard.currency}',
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: _MetricTile(
                            label: 'Available',
                            value:
                                '${state.dashboard.availableAmount} ${state.dashboard.currency}',
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    _MetricTile(
                      label: 'NGR → комиссия',
                      value:
                          '${state.dashboard.ngrAmount} → ${state.dashboard.commissionAmount} ${state.dashboard.currency}',
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Воронка: ${state.dashboard.clicks} кликов → '
                      '${state.dashboard.registrations} регистраций → '
                      '${state.dashboard.ftdCount} FTD '
                      '(${(state.dashboard.conversionRate * 100).toStringAsFixed(1)}%)',
                    ),
                    const SizedBox(height: 16),
                    const Text('Ссылки',
                        style: TextStyle(
                            fontSize: 16, fontWeight: FontWeight.bold)),
                    const SizedBox(height: 8),
                    if (state.links.isEmpty)
                      const Text('Ссылок пока нет — создайте в веб-кабинете.'),
                    for (final link in state.links)
                      ListTile(
                        leading: const Icon(Icons.link),
                        title: Text(link.campaignName.isEmpty
                            ? link.referralCode
                            : link.campaignName),
                        subtitle: Text(link.referralUrl),
                      ),
                    const SizedBox(height: 16),
                    const Text('Выплаты',
                        style: TextStyle(
                            fontSize: 16, fontWeight: FontWeight.bold)),
                    const SizedBox(height: 8),
                    if (state.payouts.isEmpty)
                      const Text('Выплат пока нет.'),
                    for (final payout in state.payouts)
                      ListTile(
                        leading: const Icon(Icons.payments_outlined),
                        title: Text('${payout.amount} ${payout.currency}'),
                        trailing: Text(payout.status),
                      ),
                  ],
                ),
              );
            }
            return const SizedBox.shrink();
          },
        ),
      ),
    );
  }
}

class _EarningsCard extends StatelessWidget {
  final String title;
  final String amount;
  final String currency;

  const _EarningsCard({
    required this.title,
    required this.amount,
    required this.currency,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 4),
            Text('$amount $currency',
                style: Theme.of(context).textTheme.headlineSmall),
          ],
        ),
      ),
    );
  }
}

class _MetricTile extends StatelessWidget {
  final String label;
  final String value;

  const _MetricTile({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(label, style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 4),
            Text(value,
                style: const TextStyle(fontWeight: FontWeight.bold)),
          ],
        ),
      ),
    );
  }
}
