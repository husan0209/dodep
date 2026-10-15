import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:uuid/uuid.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/network/api_client.dart';

/// Matches a non-negative decimal string with up to 8 fractional digits
/// (NUMERIC(18,8) on the backend) and at least one non-zero digit.
/// Amounts are always transported as strings — never `double`.
const _decimalAmount = RegExp(r'^\d+(\.\d{1,8})?$');
const _nonZero = RegExp(r'[1-9]');

bool _isValidAmount(String value) {
  final trimmed = value.trim();
  if (!_decimalAmount.hasMatch(trimmed)) {
    return false;
  }
  return _nonZero.hasMatch(trimmed);
}

/// Wallet page: balances, deposit/withdraw forms and transaction history.
///
/// Contracts (web client compatible):
/// - `GET /api/v1/wallet/balances` — list of `{currency, available, locked,
///   total}` decimal strings.
/// - `GET /api/v1/wallet/transactions` — history.
/// - `POST /api/v1/wallet/deposit` / `POST /api/v1/wallet/withdraw` —
///   `{amount (decimal string), method, currency, idempotency_key}`.
class WalletPage extends StatelessWidget {
  const WalletPage({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Wallet'),
          bottom: const TabBar(
            tabs: [
              Tab(text: 'Balance', icon: Icon(Icons.account_balance_wallet)),
              Tab(text: 'Deposit', icon: Icon(Icons.add_card)),
              Tab(text: 'Withdraw', icon: Icon(Icons.money_off)),
            ],
          ),
        ),
        body: const TabBarView(
          children: [
            _BalanceTab(),
            _MoneyFormTab(mode: _MoneyMode.deposit),
            _MoneyFormTab(mode: _MoneyMode.withdraw),
          ],
        ),
      ),
    );
  }
}

class _Balance {
  final String currency;
  final String available;
  final String locked;
  final String total;

  const _Balance({
    required this.currency,
    required this.available,
    required this.locked,
    required this.total,
  });

  factory _Balance.fromJson(Map<String, dynamic> json) {
    return _Balance(
      currency: '${json['currency'] ?? ''}',
      available: '${json['available'] ?? '0'}',
      locked: '${json['locked'] ?? '0'}',
      total: '${json['total'] ?? '0'}',
    );
  }
}

class _Tx {
  final String id;
  final String type;
  final String amount;
  final String status;
  final String createdAt;

  const _Tx({
    required this.id,
    required this.type,
    required this.amount,
    required this.status,
    required this.createdAt,
  });

  factory _Tx.fromJson(Map<String, dynamic> json) {
    return _Tx(
      id: '${json['id'] ?? ''}',
      type: '${json['type'] ?? 'adjustment'}',
      amount: '${json['amount'] ?? '0'}',
      status: '${json['status'] ?? 'pending'}',
      createdAt: '${json['created_at'] ?? ''}',
    );
  }
}

List<dynamic> _asList(dynamic data) {
  if (data is List) {
    return data;
  }
  if (data is Map<String, dynamic>) {
    for (final key in const ['data', 'transactions', 'balances', 'items']) {
      if (data[key] is List) {
        return data[key] as List;
      }
    }
  }
  return const [];
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
      return e.message ?? 'Request failed.';
  }
}

class _BalanceTab extends StatefulWidget {
  const _BalanceTab();

  @override
  State<_BalanceTab> createState() => _BalanceTabState();
}

class _BalanceTabState extends State<_BalanceTab> {
  List<_Balance> _balances = const [];
  List<_Tx> _history = const [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = getIt<ApiClient>();
      final results = await Future.wait([
        api.get('/api/v1/wallet/balances'),
        api.get('/api/v1/wallet/transactions', params: {'limit': 20}),
      ]);
      if (!mounted) {
        return;
      }
      setState(() {
        _balances = _asList(results[0].data)
            .whereType<Map<String, dynamic>>()
            .map(_Balance.fromJson)
            .toList();
        _history = _asList(results[1].data)
            .whereType<Map<String, dynamic>>()
            .map(_Tx.fromJson)
            .toList();
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

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(
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
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (_balances.isEmpty)
            const Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('No wallets yet. Make your first deposit.'),
              ),
            )
          else
            for (final balance in _balances)
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        balance.currency,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        '${balance.available} ${balance.currency}',
                        style: Theme.of(context).textTheme.headlineSmall,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Locked: ${balance.locked} · Total: ${balance.total}',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ),
          const SizedBox(height: 16),
          Text(
            'History',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          if (_history.isEmpty)
            const Text('No transactions yet.')
          else
            for (final tx in _history)
              ListTile(
                contentPadding: EdgeInsets.zero,
                leading: Icon(
                  tx.type.startsWith('deposit') || tx.type == 'bet_win'
                      ? Icons.arrow_downward
                      : Icons.arrow_upward,
                  color: tx.type.startsWith('deposit') || tx.type == 'bet_win'
                      ? Colors.green
                      : Colors.red,
                ),
                title: Text('${tx.type} · ${tx.amount}'),
                subtitle: Text(tx.createdAt.isEmpty ? tx.id : tx.createdAt),
                trailing: _StatusChip(status: tx.status),
              ),
        ],
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  final String status;

  const _StatusChip({required this.status});

  @override
  Widget build(BuildContext context) {
    final color = switch (status) {
      'completed' => Colors.green,
      'pending' => Colors.orange,
      'failed' => Colors.red,
      _ => Colors.grey,
    };
    return Chip(
      label: Text(status),
      backgroundColor: Color.lerp(color, Colors.white, 0.85),
    );
  }
}

enum _MoneyMode { deposit, withdraw }

class _MoneyFormTab extends StatefulWidget {
  final _MoneyMode mode;

  const _MoneyFormTab({required this.mode});

  @override
  State<_MoneyFormTab> createState() => _MoneyFormTabState();
}

const _methods = ['card', 'bank_transfer', 'crypto'];
const _currencies = ['USD', 'EUR', 'RUB', 'KZT', 'GBP'];

class _MoneyFormTabState extends State<_MoneyFormTab> {
  final TextEditingController _amount = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  String _method = _methods.first;
  String _currency = _currencies.first;
  bool _submitting = false;

  @override
  void dispose() {
    _amount.dispose();
    super.dispose();
  }

  bool get _isDeposit => widget.mode == _MoneyMode.deposit;

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    setState(() {
      _submitting = true;
    });
    try {
      final path = _isDeposit
          ? '/api/v1/wallet/deposit'
          : '/api/v1/wallet/withdraw';
      final response = await getIt<ApiClient>().post(
        path,
        data: {
          // Decimal string — never a float (CONVENTIONS NEVER-6).
          'amount': _amount.text.trim(),
          'method': _method,
          'currency': _currency,
          'idempotency_key': const Uuid().v4(),
        },
      );
      if (!mounted) {
        return;
      }
      final body = response.data;
      final detail = body is Map<String, dynamic>
          ? '${body['deposit_id'] ?? body['withdrawal_id'] ?? ''}'
          : '';
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            _isDeposit
                ? 'Deposit created${detail.isEmpty ? '' : ' ($detail)'}.'
                : 'Withdrawal requested${detail.isEmpty ? '' : ' ($detail)'}.',
          ),
        ),
      );
      _amount.clear();
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
          _submitting = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextFormField(
                controller: _amount,
                keyboardType: const TextInputType.numberWithOptions(
                  decimal: true,
                ),
                decoration: InputDecoration(
                  labelText: 'Amount (${_currency})',
                  hintText: '0.00',
                  border: const OutlineInputBorder(),
                ),
                validator: (value) {
                  if (value == null || !_isValidAmount(value)) {
                    return 'Enter a positive amount, up to 8 decimals.';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: _method,
                decoration: const InputDecoration(
                  labelText: 'Method',
                  border: OutlineInputBorder(),
                ),
                items: [
                  for (final method in _methods)
                    DropdownMenuItem(
                      value: method,
                      child: Text(method),
                    ),
                ],
                onChanged: (value) {
                  if (value != null) {
                    setState(() => _method = value);
                  }
                },
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: _currency,
                decoration: const InputDecoration(
                  labelText: 'Currency',
                  border: OutlineInputBorder(),
                ),
                items: [
                  for (final currency in _currencies)
                    DropdownMenuItem(
                      value: currency,
                      child: Text(currency),
                    ),
                ],
                onChanged: (value) {
                  if (value != null) {
                    setState(() => _currency = value);
                  }
                },
              ),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(_isDeposit ? 'Deposit' : 'Withdraw'),
              ),
              const SizedBox(height: 8),
              Text(
                _isDeposit
                    ? 'Deposits are credited after PSP confirmation.'
                    : 'Withdrawals require completed wagering and KYC limits.',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),
        ),
      ],
    );
  }
}
