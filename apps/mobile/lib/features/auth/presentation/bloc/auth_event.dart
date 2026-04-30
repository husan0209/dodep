import 'package:freezed_annotation/freezed_annotation.dart';

part 'auth_event.freezed.dart';

/// Authentication events
@freezed
class AuthEvent with _$AuthEvent {
  /// Login event
  const factory AuthEvent.login({
    required String email,
    required String password,
  }) = AuthLogin;

  /// Register event
  const factory AuthEvent.register({
    required String email,
    required String password,
    required String username,
  }) = AuthRegister;

  /// Logout event
  const factory AuthEvent.logout() = AuthLogout;

  /// Get current user event
  const factory AuthEvent.getCurrentUser() = AuthGetCurrentUser;

  /// Token refresh event
  const factory AuthEvent.refreshToken() = AuthRefreshToken;
}
