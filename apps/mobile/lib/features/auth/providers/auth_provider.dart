import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/network/api_client.dart';

class AuthState {
  final bool isAuthenticated;
  final bool isLoading;
  final String? error;
  final Map<String, dynamic>? user;

  const AuthState({
    this.isAuthenticated = false,
    this.isLoading = false,
    this.error,
    this.user,
  });

  AuthState copyWith({
    bool? isAuthenticated,
    bool? isLoading,
    String? error,
    Map<String, dynamic>? user,
  }) {
    return AuthState(
      isAuthenticated: isAuthenticated ?? this.isAuthenticated,
      isLoading: isLoading ?? this.isLoading,
      error: error,
      user: user ?? this.user,
    );
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  final ApiClient _apiClient = ApiClient();

  AuthNotifier() : super(const AuthState());

  Future<bool> login(String email, String password) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final response = await _apiClient.login(email, password);
      await _apiClient.setAuthToken(response['access_token']);
      state = state.copyWith(
        isAuthenticated: true,
        isLoading: false,
        user: response['user'],
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
      return false;
    }
  }

  Future<bool> register(String email, String password, String username) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final response = await _apiClient.register(email, password, username);
      await _apiClient.setAuthToken(response['access_token']);
      state = state.copyWith(
        isAuthenticated: true,
        isLoading: false,
        user: response['user'],
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
      return false;
    }
  }

  Future<void> logout() async {
    try {
      await _apiClient.logout();
    } catch (e) {
      // Ignore logout errors
    }
    state = const AuthState(isAuthenticated: false);
  }

  Future<void> fetchUser() async {
    try {
      final user = await _apiClient.getMe();
      state = state.copyWith(user: user, isAuthenticated: true);
    } catch (e) {
      state = const AuthState(isAuthenticated: false);
    }
  }
}

final authNotifierProvider = StateNotifierProvider<AuthNotifier, AuthState>(
  (ref) => AuthNotifier(),
);

final authStateProvider = Provider<AuthState>((ref) {
  return ref.watch(authNotifierProvider);
});
