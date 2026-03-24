import '../../../core/error/exceptions.dart';
import '../models/auth_tokens.dart';
import '../models/user_model.dart';

/// Authentication remote data source interface
abstract class AuthRemoteDataSource {
  /// Login
  Future<AuthTokens> login({
    required String email,
    required String password,
  });

  /// Register
  Future<AuthTokens> register({
    required String email,
    required String password,
    required String username,
  });

  /// Logout
  Future<void> logout();

  /// Get current user
  Future<UserModel> getCurrentUser();

  /// Refresh tokens
  Future<AuthTokens> refreshTokens({
    required String refreshToken,
  });
}
