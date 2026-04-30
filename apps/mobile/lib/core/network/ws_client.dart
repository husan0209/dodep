import 'dart:async';
import 'dart:convert';

import 'package:injectable/injectable.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../config/app_config.dart';

/// WebSocket message
class WsMessage {
  final String type;
  final String channel;
  final dynamic data;
  final DateTime timestamp;

  const WsMessage({
    required this.type,
    required this.channel,
    required this.data,
    required this.timestamp,
  });

  factory WsMessage.fromJson(Map<String, dynamic> json) {
    return WsMessage(
      type: json['type'] as String? ?? 'unknown',
      channel: json['channel'] as String? ?? '',
      data: json['data'],
      timestamp: DateTime.tryParse(json['timestamp'] as String? ?? '') ?? DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'channel': channel,
      'data': data,
      'timestamp': timestamp.toIso8601String(),
    };
  }
}

/// WebSocket client manager
@singleton
class WsClient {
  WebSocketChannel? _channel;
  final _controller = StreamController<WsMessage>.broadcast();
  final _subscriptions = <String, Set<void Function(WsMessage)>>{};
  int _reconnectAttempts = 0;
  final int _maxReconnectAttempts = 10;
  Timer? _heartbeatTimer;
  String? _token;

  /// WebSocket messages stream
  Stream<WsMessage> get messages => _controller.stream;

  /// Connection status
  bool get isConnected => _channel != null && _channel!.closeCode == null;

  /// Connect to WebSocket
  void connect(String token) {
    if (isConnected) return;

    _token = token;
    try {
      _channel = WebSocketChannel.connect(
        Uri.parse('${AppConfig.wsUrl}?token=$token'),
      );

      _channel!.stream.listen(
        (data) {
          try {
            final msg = WsMessage.fromJson(jsonDecode(data as String));
            _controller.add(msg);

            // Notify subscribers
            final handlers = _subscriptions[msg.channel];
            if (handlers != null) {
              for (final handler in handlers) {
                handler(msg);
              }
            }
          } catch (e) {
            print('Error parsing WebSocket message: $e');
          }
        },
        onDone: () => _reconnect(),
        onError: (error) {
          print('WebSocket error: $error');
          _reconnect();
        },
      );

      _reconnectAttempts = 0;
      _startHeartbeat();
    } catch (e) {
      print('Failed to connect WebSocket: $e');
      _reconnect();
    }
  }

  /// Subscribe to a channel
  void Function()? subscribe(String channel, void Function(WsMessage) handler) {
    if (!_subscriptions.containsKey(channel)) {
      _subscriptions[channel] = {};
      if (isConnected) {
        _send({'action': 'subscribe', 'channel': channel});
      }
    }
    _subscriptions[channel]!.add(handler);

    // Return unsubscribe function
    return () {
      final handlers = _subscriptions[channel];
      if (handlers != null) {
        handlers.remove(handler);
        if (handlers.isEmpty) {
          _subscriptions.remove(channel);
          if (isConnected) {
            _send({'action': 'unsubscribe', 'channel': channel});
          }
        }
      }
    };
  }

  /// Send message
  void send(Map<String, dynamic> data) {
    if (isConnected) {
      _send(data);
    }
  }

  void _send(Map<String, dynamic> data) {
    _channel?.sink.add(jsonEncode(data));
  }

  /// Start heartbeat
  void _startHeartbeat() {
    _heartbeatTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      _send({'type': 'ping'});
    });
  }

  /// Stop heartbeat
  void _stopHeartbeat() {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
  }

  /// Reconnect with exponential backoff
  void _reconnect() {
    _stopHeartbeat();

    if (_reconnectAttempts >= _maxReconnectAttempts) {
      print('Max reconnection attempts reached');
      return;
    }

    final delay = Duration(milliseconds: 1000 * (_reconnectAttempts + 1));
    _reconnectAttempts++;

    Future.delayed(delay.min(const Duration(seconds: 30)), () {
      if (_token != null) {
        connect(_token!);
      }
    });
  }

  /// Disconnect
  void disconnect() {
    _stopHeartbeat();
    _channel?.sink.close();
    _channel = null;
    _subscriptions.clear();
    _controller.close();
    _token = null;
  }

  /// Dispose
  void dispose() {
    disconnect();
  }
}
