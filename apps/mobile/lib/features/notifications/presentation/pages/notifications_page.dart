import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/network/api_client.dart';

/// Notification center: list, unread filter, mark-as-read, delete and
/// pull-to-refresh.
///
/// Canonical contract (`docs/FRONTEND_SPEC.md`):
/// - `GET /api/v1/notifications` — list (under `data`/`notifications`/`items`)
/// - `POST /api/v1/notifications/{id}/read` — mark one as read
///   (action-style, like casino `.../end` and bets `.../cashout`)
/// - `POST /api/v1/notifications/read-all` — mark all as read
/// - `DELETE /api/v1/notifications/{id}` — delete
/// NOTE: the Go notification-service currently exposes gRPC only, so these
/// REST routes must exist on the API gateway for this page to load data.
class NotificationsPage extends StatefulWidget {
  const NotificationsPage({super.key});

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _Notification {
  final String id;
  final String title;
  final String body;
  final String type;
  final bool read;
  final String createdAt;

  const _Notification({
    required this.id,
    required this.title,
    required this.body,
    required this.type,
    required this.read,
    required this.createdAt,
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

  factory _Notification.fromJson(Map<String, dynamic> json) {
    final rawRead = json['is_read'] ?? json['read'] ?? json['seen'];
    return _Notification(
      id: _str(json, const ['id', 'notification_id']),
      title: _str(json, const ['title', 'subject']),
      body: _str(json, const ['body', 'message', 'text']),
      type: _str(json, const ['type', 'category', 'channel']),
      read: rawRead is bool ? rawRead : '$rawRead' == 'true',
      createdAt: _str(json, const ['created_at', 'sent_at', 'timestamp']),
    );
  }

  _Notification markRead() {
    return _Notification(
      id: id,
      title: title,
      body: body,
      type: type,
      read: true,
      createdAt: createdAt,
    );
  }
}

String _timeAgo(String iso) {
  final parsed = DateTime.tryParse(iso);
  if (parsed == null) {
    return '';
  }
  final diff = DateTime.now().difference(parsed.toLocal());
  if (diff.inMinutes < 1) {
    return 'Just now';
  }
  if (diff.inHours < 1) {
    return '${diff.inMinutes}m ago';
  }
  if (diff.inDays < 1) {
    return '${diff.inHours}h ago';
  }
  if (diff.inDays < 30) {
    return '${diff.inDays}d ago';
  }
  return '${parsed.toLocal().year}-'
      '${parsed.toLocal().month.toString().padLeft(2, '0')}-'
      '${parsed.toLocal().day.toString().padLeft(2, '0')}';
}

class _NotificationsPageState extends State<NotificationsPage> {
  List<_Notification> _items = const [];
  bool _unreadOnly = false;
  bool _loading = true;
  String? _error;
  final Set<String> _busy = <String>{};

  @override
  void initState() {
    super.initState();
    _load();
  }

  List<_Notification> get _visible {
    if (!_unreadOnly) {
      return _items;
    }
    return _items.where((item) => !item.read).toList();
  }

  int get _unreadCount => _items.where((item) => !item.read).length;

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final response = await getIt<ApiClient>().get('/api/v1/notifications');
      final data = response.data;
      List<dynamic> raw = const [];
      if (data is List) {
        raw = data;
      } else if (data is Map<String, dynamic>) {
        for (final key in const ['data', 'notifications', 'items']) {
          if (data[key] is List) {
            raw = data[key] as List;
            break;
          }
        }
      }
      final items = raw
          .whereType<Map<String, dynamic>>()
          .map(_Notification.fromJson)
          .where((item) => item.id.isNotEmpty)
          .toList();
      if (!mounted) {
        return;
      }
      setState(() {
        _items = items;
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

  Future<void> _markRead(_Notification item) async {
    if (item.read || _busy.contains(item.id)) {
      return;
    }
    setState(() {
      _busy.add(item.id);
    });
    try {
      await getIt<ApiClient>().post('/api/v1/notifications/${item.id}/read');
      if (!mounted) {
        return;
      }
      setState(() {
        _items = [
          for (final current in _items)
            if (current.id == item.id) current.markRead() else current,
        ];
      });
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
          _busy.remove(item.id);
        });
      }
    }
  }

  Future<void> _markAllRead() async {
    try {
      await getIt<ApiClient>().post('/api/v1/notifications/read-all');
      if (!mounted) {
        return;
      }
      setState(() {
        _items = [for (final item in _items) item.markRead()];
      });
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(_apiError(e))),
      );
    }
  }

  Future<void> _delete(_Notification item) async {
    final index = _items.indexWhere((current) => current.id == item.id);
    setState(() {
      _items = [
        for (final current in _items)
          if (current.id != item.id) current,
      ];
    });
    try {
      await getIt<ApiClient>().delete('/api/v1/notifications/${item.id}');
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _items = [..._items..insert(index.clamp(0, _items.length), item)];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(_apiError(e))),
      );
    }
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

  @override
  Widget build(BuildContext context) {
    final visible = _visible;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          _unreadCount == 0
              ? 'Notifications'
              : 'Notifications ($_unreadCount)',
        ),
        actions: [
          if (_unreadCount > 0)
            TextButton(
              onPressed: _markAllRead,
              child: const Text('Read all'),
            ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: Row(
              children: [
                ChoiceChip(
                  label: const Text('All'),
                  selected: !_unreadOnly,
                  onSelected: (_) => setState(() => _unreadOnly = false),
                ),
                const SizedBox(width: 8),
                ChoiceChip(
                  label: const Text('Unread'),
                  selected: _unreadOnly,
                  onSelected: (_) => setState(() => _unreadOnly = true),
                ),
              ],
            ),
          ),
          Expanded(
            child: _loading
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
                        child: visible.isEmpty
                            ? ListView(
                                children: const [
                                  SizedBox(height: 120),
                                  Center(
                                    child: Text('You are all caught up.'),
                                  ),
                                ],
                              )
                            : ListView.separated(
                                padding: const EdgeInsets.fromLTRB(
                                  16,
                                  8,
                                  16,
                                  16,
                                ),
                                itemCount: visible.length,
                                separatorBuilder: (_, __) =>
                                    const SizedBox(height: 8),
                                itemBuilder: (context, index) {
                                  final item = visible[index];
                                  return Dismissible(
                                    key: ValueKey(item.id),
                                    direction: DismissDirection.endToStart,
                                    background: Container(
                                      alignment: Alignment.centerRight,
                                      padding: const EdgeInsets.only(
                                        right: 16,
                                      ),
                                      decoration: BoxDecoration(
                                        color: Colors.red,
                                        borderRadius: BorderRadius.circular(
                                          12,
                                        ),
                                      ),
                                      child: const Icon(
                                        Icons.delete,
                                        color: Colors.white,
                                      ),
                                    ),
                                    onDismissed: (_) => _delete(item),
                                    child: _NotificationTile(
                                      item: item,
                                      busy: _busy.contains(item.id),
                                      onTap: () => _markRead(item),
                                    ),
                                  );
                                },
                              ),
                      ),
          ),
        ],
      ),
    );
  }
}

class _NotificationTile extends StatelessWidget {
  final _Notification item;
  final bool busy;
  final VoidCallback onTap;

  const _NotificationTile({
    required this.item,
    required this.busy,
    required this.onTap,
  });

  IconData _icon(String type) {
    return switch (type) {
      'bonus' => Icons.card_giftcard,
      'payment' || 'withdrawal' || 'deposit' => Icons.payments_outlined,
      'bet' || 'cashout' => Icons.sports_soccer,
      'security' || 'login' => Icons.security,
      'kyc' => Icons.verified_user_outlined,
      _ => Icons.notifications_outlined,
    };
  }

  @override
  Widget build(BuildContext context) {
    final ago = _timeAgo(item.createdAt);
    return Card(
      color: item.read ? null : Theme.of(context).colorScheme.primaryContainer,
      child: ListTile(
        leading: Icon(_icon(item.type)),
        title: Text(
          item.title.isEmpty ? item.type : item.title,
          style: TextStyle(
            fontWeight: item.read ? FontWeight.normal : FontWeight.bold,
          ),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (item.body.isNotEmpty) Text(item.body),
            if (ago.isNotEmpty)
              Text(
                ago,
                style: Theme.of(context).textTheme.bodySmall,
              ),
          ],
        ),
        trailing: busy
            ? const SizedBox(
                height: 20,
                width: 20,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : item.read
                ? null
                : const Icon(Icons.circle, size: 12, color: Colors.blue),
        onTap: item.read ? null : onTap,
      ),
    );
  }
}
