import 'package:freezed_annotation/freezed_annotation.dart';

import '../../domain/entities/user.dart';

part 'auth_state.freezed.dart';

/// Authentication state
@freezed
class AuthState with _$AuthState {
  /// Initial state
  const factory AuthState.initial() = AuthInitial;

  /// Loading state
  const factory AuthState.loading() = AuthLoading;

  /// Authenticated state
  const factory AuthState.authenticated(User user) = AuthAuthenticated;

  /// Unauthenticated state
  const factory AuthState.unauthenticated() = AuthUnauthenticated;

  /// Error state
  const factory AuthState.error(String message) = AuthError;
}
