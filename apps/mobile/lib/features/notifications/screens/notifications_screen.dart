import 'package:flutter/material.dart';

class NotificationsScreen extends StatefulWidget {
  const NotificationsScreen({super.key});

  @override
  State<NotificationsScreen> createState() => _NotificationsScreenState();
}

class _NotificationsScreenState extends State<NotificationsScreen> {
  final List<Map<String, dynamic>> _mockNotifications = [
    {
      'id': '1',
      'type': 'deposit',
      'title': 'Депозит подтверждён',
      'message': 'Ваш депозит на сумму 1000₽ успешно зачислен',
      'isRead': false,
      'timestamp': DateTime.now().subtract(const Duration(minutes: 30)),
    },
    {
      'id': '2',
      'type': 'bet',
      'title': 'Ставка рассчитана',
      'message': 'Ваша ставка выиграла! Выигрыш: 2500₽',
      'isRead': false,
      'timestamp': DateTime.now().subtract(const Duration(hours: 2)),
    },
    {
      'id': '3',
      'type': 'bonus',
      'title': 'Новый бонус доступен',
      'message': 'Получите 50 фриспинов в Book of Dead',
      'isRead': true,
      'timestamp': DateTime.now().subtract(const Duration(days: 1)),
    },
    {
      'id': '4',
      'type': 'system',
      'title': 'Обновление приложения',
      'message': 'Доступна новая версия приложения. Обновите для лучших впечатлений.',
      'isRead': true,
      'timestamp': DateTime.now().subtract(const Duration(days: 3)),
    },
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Уведомления'),
        actions: [
          IconButton(
            icon: const Icon(Icons.done_all),
            onPressed: () {
              setState(() {
                for (var notification in _mockNotifications) {
                  notification['isRead'] = true;
                }
              });
            },
            tooltip: 'Отметить все как прочитанные',
          ),
        ],
      ),
      body: ListView.builder(
        itemCount: _mockNotifications.length,
        itemBuilder: (context, index) {
          final notification = _mockNotifications[index];
          return Dismissible(
            key: Key(notification['id']),
            direction: DismissDirection.endToStart,
            background: Container(
              color: Colors.red,
              alignment: Alignment.centerRight,
              padding: const EdgeInsets.only(right: 16),
              child: const Icon(Icons.delete, color: Colors.white),
            ),
            onDismissed: (direction) {
              setState(() {
                _mockNotifications.removeAt(index);
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Уведомление удалено')),
              );
            },
            child: Card(
              margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              color: notification['isRead'] ? null : Colors.blue.shade50,
              child: ListTile(
                leading: CircleAvatar(
                  backgroundColor: _getNotificationColor(notification['type']),
                  child: Icon(
                    _getNotificationIcon(notification['type']),
                    color: Colors.white,
                  ),
                ),
                title: Text(
                  notification['title'],
                  style: TextStyle(
                    fontWeight:
                        notification['isRead'] ? FontWeight.normal : FontWeight.bold,
                  ),
                ),
                subtitle: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const SizedBox(height: 4),
                    Text(notification['message']),
                    const SizedBox(height: 4),
                    Text(
                      _formatTimestamp(notification['timestamp']),
                      style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
                    ),
                  ],
                ),
                trailing: notification['isRead']
                    ? null
                    : Container(
                        width: 8,
                        height: 8,
                        decoration: const BoxDecoration(
                          color: Colors.blue,
                          shape: BoxShape.circle,
                        ),
                      ),
                onTap: () {
                  setState(() {
                    notification['isRead'] = true;
                  });
                  // Navigate to detail or action
                },
              ),
            ),
          );
        },
      ),
    );
  }

  Color _getNotificationColor(String type) {
    switch (type) {
      case 'deposit':
        return Colors.green;
      case 'withdraw':
        return Colors.orange;
      case 'bet':
        return Colors.blue;
      case 'bonus':
        return Colors.purple;
      case 'system':
        return Colors.grey;
      default:
        return Colors.grey;
    }
  }

  IconData _getNotificationIcon(String type) {
    switch (type) {
      case 'deposit':
        return Icons.add_circle;
      case 'withdraw':
        return Icons.remove_circle;
      case 'bet':
        return Icons.sports;
      case 'bonus':
        return Icons.card_giftcard;
      case 'system':
        return Icons.notifications;
      default:
        return Icons.notifications;
    }
  }

  String _formatTimestamp(DateTime timestamp) {
    final now = DateTime.now();
    final diff = now.difference(timestamp);

    if (diff.inMinutes < 1) {
      return 'Только что';
    } else if (diff.inMinutes < 60) {
      return '${diff.inMinutes} мин назад';
    } else if (diff.inHours < 24) {
      return '${diff.inHours} ч назад';
    } else if (diff.inDays < 7) {
      return '${diff.inDays} дн назад';
    } else {
      return '${timestamp.day}.${timestamp.month}.${timestamp.year}';
    }
  }
}
