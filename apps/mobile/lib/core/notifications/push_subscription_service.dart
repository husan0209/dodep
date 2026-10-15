import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:injectable/injectable.dart';

import '../di/injection.dart';
import '../network/api_client.dart';

/// Subscribes the device FCM token for push notifications.
///
/// Contract: `POST /api/v1/notifications/push/subscribe` with
/// `{fcm_token, platform}` and `POST .../unsubscribe` with `{fcm_token}`.
/// These routes are served by the API gateway (the Go notification-service
/// itself is gRPC-only). Uses only packages already in pubspec
/// (`firebase_messaging`, `ApiClient`/`dio`); no new dependencies.
@injectable
class PushSubscriptionService {
  final ApiClient _api;
  final FirebaseMessaging _messaging;

  PushSubscriptionService({ApiClient? api, FirebaseMessaging? messaging})
      : _api = api ?? getIt<ApiClient>(),
        _messaging = messaging ?? FirebaseMessaging.instance;

  static const String subscribePath =
      '/api/v1/notifications/push/subscribe';
  static const String unsubscribePath =
      '/api/v1/notifications/push/unsubscribe';

  /// Current FCM registration token, null when unavailable.
  Future<String?> currentToken() => _messaging.getToken();

  /// Sends the current FCM token to the backend.
  ///
  /// No-op when no token is available. Dio errors propagate to the caller.
  Future<void> subscribe() async {
    final token = await _messaging.getToken();
    if (token == null || token.isEmpty) {
      return;
    }
    await _api.post(
      subscribePath,
      data: {
        'fcm_token': token,
        'platform': _platformName(),
      },
    );
  }

  /// Removes the current FCM token from the backend, then deletes it
  /// locally so a fresh token is minted on next subscribe.
  Future<void> unsubscribe() async {
    final token = await _messaging.getToken();
    if (token != null && token.isNotEmpty) {
      await _api.post(
        unsubscribePath,
        data: {'fcm_token': token},
      );
    }
    await _messaging.deleteToken();
  }

  String _platformName() {
    switch (defaultTargetPlatform) {
      case TargetPlatform.android:
        return 'android';
      case TargetPlatform.iOS:
        return 'ios';
      default:
        return defaultTargetPlatform.name;
    }
  }
}
