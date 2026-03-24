import '../../../core/error/exceptions.dart';
import '../models/auth_tokens.dart';
import '../models/user_model.dart';

/// Authentication local data source interface
abstract class AuthLocalDataSource {
  /// Get access token
  String? getAccessToken();

  /// Get refresh token
  String? getRefreshToken();

  /// Save tokens
  Future<void> saveTokens(AuthTokens tokens);

  /// Clear tokens
  Future<void> clearTokens();

  /// Get cached user
  UserModel? getCachedUser();

  /// Cache user
  Future<void> cacheUser(UserModel user);

  /// Clear cached user
  Future<void> clearCachedUser();

  /// Check if user is authenticated
  bool isAuthenticated();
}
