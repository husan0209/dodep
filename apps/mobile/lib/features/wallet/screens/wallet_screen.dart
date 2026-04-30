import 'package:flutter/material.dart';

class WalletScreen extends StatefulWidget {
  const WalletScreen({super.key});

  @override
  State<WalletScreen> createState() => _WalletScreenState();
}

class _WalletScreenState extends State<WalletScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  final List<Map<String, dynamic>> _balances = [
    {'currency': 'RUB', 'balance': 15000.00, 'locked': 500.00},
    {'currency': 'USD', 'balance': 100.00, 'locked': 20.00},
    {'currency': 'EUR', 'balance': 50.00, 'locked': 0.00},
  ];

  final List<Map<String, dynamic>> _transactions = [
    {
      'id': '1',
      'type': 'deposit',
      'amount': 1000.00,
      'currency': 'RUB',
      'status': 'completed',
      'method': 'Card',
      'date': DateTime.now().subtract(const Duration(hours: 2)),
    },
    {
      'id': '2',
      'type': 'withdraw',
      'amount': -500.00,
      'currency': 'RUB',
      'status': 'pending',
      'method': 'Bank Transfer',
      'date': DateTime.now().subtract(const Duration(days: 1)),
    },
    {
      'id': '3',
      'type': 'bet',
      'amount': -100.00,
      'currency': 'RUB',
      'status': 'completed',
      'method': 'Bet',
      'date': DateTime.now().subtract(const Duration(days: 2)),
    },
  ];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Кошелёк'),
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Депозит'),
            Tab(text: 'Вывод'),
            Tab(text: 'История'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          _buildDepositTab(),
          _buildWithdrawTab(),
          _buildHistoryTab(),
        ],
      ),
    );
  }

  Widget _buildDepositTab() {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Balances
          ..._balances.map((balance) => _buildBalanceCard(balance)),
          const SizedBox(height: 24),
          // Payment methods
          const Text('Способ оплаты', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
          const SizedBox(height: 12),
          _buildPaymentMethod('card', '💳', 'Банковская карта'),
          _buildPaymentMethod('sbp', '📱', 'СБП'),
          _buildPaymentMethod('crypto', '₿', 'Cryptocurrency'),
          const SizedBox(height: 24),
          // Amount
          TextField(
            keyboardType: TextInputType.number,
            decoration: const InputDecoration(
              labelText: 'Сумма',
              suffixText: '₽',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 16),
          // Quick amounts
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [500, 1000, 2000, 5000, 10000]
                .map((amount) => ActionChip(
                      label: Text('+${amount}₽'),
                      onPressed: () {},
                    ))
                .toList(),
          ),
          const SizedBox(height: 24),
          ElevatedButton(
            onPressed: () {},
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 16),
            ),
            child: const Text('Пополнить'),
          ),
        ],
      ),
    );
  }

  Widget _buildWithdrawTab() {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // KYC warning
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.amber.shade50,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(Icons.warning, color: Colors.amber.shade700),
                const SizedBox(width: 8),
                const Expanded(
                  child: Text(
                    'Пройдите верификацию для вывода средств',
                    style: TextStyle(fontSize: 14),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 24),
          // Withdraw methods
          const Text('Способ вывода', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
          const SizedBox(height: 12),
          _buildPaymentMethod('card', '💳', 'Банковская карта'),
          _buildPaymentMethod('sbp', '📱', 'СБП'),
          _buildPaymentMethod('crypto', '₿', 'Cryptocurrency'),
          const SizedBox(height: 24),
          // Amount
          TextField(
            keyboardType: TextInputType.number,
            decoration: const InputDecoration(
              labelText: 'Сумма',
              suffixText: '₽',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 24),
          ElevatedButton(
            onPressed: () {},
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 16),
            ),
            child: const Text('Вывести'),
          ),
        ],
      ),
    );
  }

  Widget _buildHistoryTab() {
    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: _transactions.length,
      itemBuilder: (context, index) {
        final tx = _transactions[index];
        return Card(
          margin: const EdgeInsets.only(bottom: 12),
          child: ListTile(
            leading: CircleAvatar(
              backgroundColor: tx['amount'] > 0
                  ? Colors.green.shade100
                  : Colors.red.shade100,
              child: Icon(
                tx['amount'] > 0 ? Icons.arrow_downward : Icons.arrow_upward,
                color: tx['amount'] > 0 ? Colors.green : Colors.red,
              ),
            ),
            title: Text(_getTransactionType(tx['type'])),
            subtitle: Text(
              '${tx['method']} • ${_formatDate(tx['date'])}',
            ),
            trailing: Text(
              '${tx['amount'] > 0 ? '+' : ''}${tx['amount'].toStringAsFixed(2)} ${tx['currency']}',
              style: TextStyle(
                color: tx['amount'] > 0 ? Colors.green : Colors.red,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildBalanceCard(Map<String, dynamic> balance) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  balance['currency'],
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Баланс: ${balance['balance'].toStringAsFixed(2)}',
                  style: TextStyle(color: Colors.grey.shade600),
                ),
                if (balance['locked'] > 0)
                  Text(
                    'В ставках: ${balance['locked'].toStringAsFixed(2)}',
                    style: TextStyle(color: Colors.grey.shade600, fontSize: 12),
                  ),
              ],
            ),
            IconButton(
              icon: const Icon(Icons.add_circle_outline),
              onPressed: () {
                _tabController.animateTo(0);
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPaymentMethod(String id, String icon, String name) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Text(icon, style: const TextStyle(fontSize: 24)),
        title: Text(name),
        trailing: const Icon(Icons.chevron_right),
        onTap: () {},
      ),
    );
  }

  String _getTransactionType(String type) {
    switch (type) {
      case 'deposit':
        return 'Пополнение';
      case 'withdraw':
        return 'Вывод';
      case 'bet':
        return 'Ставка';
      case 'win':
        return 'Выигрыш';
      default:
        return type;
    }
  }

  String _formatDate(DateTime date) {
    return '${date.day}.${date.month}.${date.year} ${date.hour}:${date.minute.toString().padLeft(2, '0')}';
  }
}
