import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:injectable/injectable.dart';

import '../../domain/usecases/login.dart';
import '../../domain/usecases/logout.dart';
import '../../domain/usecases/register.dart';
import 'auth_event.dart';
import 'auth_state.dart';

/// Authentication BLoC
@injectable
class AuthBloc extends Bloc<AuthEvent, AuthState> {
  final Login _login;
  final Register _register;
  final Logout _logout;

  AuthBloc({
    required Login login,
    required Register register,
    required Logout logout,
  })  : _login = login,
        _register = register,
        _logout = logout,
        super(const AuthState.initial()) {
    on<AuthLogin>(_onLogin);
    on<AuthRegister>(_onRegister);
    on<AuthLogout>(_onLogout);
    on<AuthGetCurrentUser>(_onGetCurrentUser);
    on<AuthRefreshToken>(_onRefreshToken);
  }

  /// Handle login event
  Future<void> _onLogin(AuthLogin event, Emitter<AuthState> emit) async {
    emit(const AuthState.loading());

    final result = await _login(LoginParams(
      email: event.email,
      password: event.password,
    ));

    result.fold(
      (failure) => emit(AuthState.error(failure.message)),
      (user) => emit(AuthState.authenticated(user)),
    );
  }

  /// Handle register event
  Future<void> _onRegister(AuthRegister event, Emitter<AuthState> emit) async {
    emit(const AuthState.loading());

    final result = await _register(RegisterParams(
      email: event.email,
      password: event.password,
      username: event.username,
    ));

    result.fold(
      (failure) => emit(AuthState.error(failure.message)),
      (user) => emit(AuthState.authenticated(user)),
    );
  }

  /// Handle logout event
  Future<void> _onLogout(AuthLogout event, Emitter<AuthState> emit) async {
    await _logout();
    emit(const AuthState.unauthenticated());
  }

  /// Handle get current user event
  Future<void> _onGetCurrentUser(AuthGetCurrentUser event, Emitter<AuthState> emit) async {
    // Implementation would call repository to get current user
    // For now, emit unauthenticated as placeholder
    emit(const AuthState.unauthenticated());
  }

  /// Handle token refresh event
  Future<void> _onRefreshToken(AuthRefreshToken event, Emitter<AuthState> emit) async {
    // Implementation would call repository to refresh tokens
    // For now, do nothing
  }
}
