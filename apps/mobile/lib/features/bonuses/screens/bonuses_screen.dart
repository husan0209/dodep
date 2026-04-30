import 'package:flutter/material.dart';

class BonusesScreen extends StatelessWidget {
  const BonusesScreen({super.key});

  final List<Map<String, dynamic>> _mockBonuses = const [
    {
      'id': '1',
      'name': 'Приветственный бонус',
      'description': '100% на первый депозит до 10000₽',
      'minDeposit': 500,
      'wagering': 35,
      'isActive': true,
      'expiresAt': '2024-04-24',
    },
    {
      'id': '2',
      'name': 'Кэшбэк 10%',
      'description': 'Получите 10% кэшбэк на проигрыши',
      'minDeposit': 0,
      'wagering': 1,
      'isActive': false,
      'expiresAt': null,
    },
    {
      'id': '3',
      'name': 'Фриспины',
      'description': '50 фриспинов в Book of Dead',
      'minDeposit': 1000,
      'wagering': 40,
      'isActive': true,
      'expiresAt': '2024-03-31',
    },
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Бонусы'),
      ),
      body: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: _mockBonuses.length,
        itemBuilder: (context, index) {
          final bonus = _mockBonuses[index];
          return Card(
            margin: const EdgeInsets.only(bottom: 16),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Expanded(
                        child: Text(
                          bonus['name'],
                          style: const TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                      if (bonus['isActive'])
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                          decoration: BoxDecoration(
                            color: Colors.green.shade100,
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            'Активен',
                            style: TextStyle(
                              color: Colors.green.shade700,
                              fontSize: 12,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(
                    bonus['description'],
                    style: TextStyle(color: Colors.grey.shade600),
                  ),
                  const SizedBox(height: 16),
                  Row(
                    children: [
                      Expanded(
                        child: _buildInfoChip(
                          'Мин. депозит',
                          '${bonus['minDeposit']}₽',
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: _buildInfoChip(
                          'Вейджер',
                          'x${bonus['wagering']}',
                        ),
                      ),
                    ],
                  ),
                  if (bonus['expiresAt'] != null) ...[
                    const SizedBox(height: 8),
                    Text(
                      'Истекает: ${bonus['expiresAt']}',
                      style: TextStyle(
                        color: Colors.orange.shade700,
                        fontSize: 12,
                      ),
                    ),
                  ],
                  const SizedBox(height: 16),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: bonus['isActive'] ? () {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Бонус активирован')),
                        );
                      } : null,
                      child: Text(bonus['isActive'] ? 'Активировать' : 'Недоступен'),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildInfoChip(String label, String value) {
    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: Colors.grey.shade100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        children: [
          Text(
            label,
            style: TextStyle(fontSize: 10, color: Colors.grey.shade600),
          ),
          const SizedBox(height: 4),
          Text(
            value,
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
        ],
      ),
    );
  }
}
