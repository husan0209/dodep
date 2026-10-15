import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:injectable/injectable.dart';

import '../models/auth_tokens.dart';
import '../models/user_model.dart';
import 'auth_local_datasource.dart';

/// Implementation of AuthLocalDataSource.
///
/// Token security model:
/// - access/refresh tokens live in platform secure storage
///   (Android EncryptedSharedPreferences / iOS Keychain) via
///   [FlutterSecureStorage], plus an in-memory cache that backs the
///   synchronous getters below (secure storage is async-only);
/// - the non-sensitive cached user profile stays in the Hive box;
/// - legacy tokens left by older builds in the unencrypted Hive box
///   `secure_storage` are moved on first access (read-through migration):
///   read Hive -> write secure storage -> delete Hive keys.
///
/// The public [AuthLocalDataSource] API is unchanged, so the refresh
/// flow in `AuthRepositoryImpl` works without modifications.
@LazySingleton(as: AuthLocalDataSource)
class AuthLocalDataSourceImpl implements AuthLocalDataSource {
  final Box _box;
  final FlutterSecureStorage _secure;

  String? _accessTokenCache;
  String? _refreshTokenCache;
  bool _migrationAttempted = false;

  AuthLocalDataSourceImpl({
    required Box box,
    FlutterSecureStorage? secureStorage,
  })  : _box = box,
        _secure = secureStorage ?? const FlutterSecureStorage();

  static const String _accessTokenKey = 'access_token';
  static const String _refreshTokenKey = 'refresh_token';
  static const String _userKey = 'cached_user';

  /// Runs the one-time read-through migration and warms the token cache.
  ///
  /// Copies any legacy Hive tokens into secure storage, then deletes the
  /// Hive keys. Idempotent and safe to call at startup: never throws
  /// (returns false on storage errors) so auth keeps working via the
  /// Hive fallback in the sync getters below.
  Future<bool> ensureMigrated() async {
    try {
      _accessTokenCache ??= await _secure.read(key: _accessTokenKey);
      _refreshTokenCache ??= await _secure.read(key: _refreshTokenKey);

      if (!_migrationAttempted) {
        final hiveAccess = _box.get(_accessTokenKey);
        if (hiveAccess is String &&
            hiveAccess.isNotEmpty &&
            (_accessTokenCache == null || _accessTokenCache!.isEmpty)) {
          await _secure.write(key: _accessTokenKey, value: hiveAccess);
          _accessTokenCache = hiveAccess;
        }
        final hiveRefresh = _box.get(_refreshTokenKey);
        if (hiveRefresh is String &&
            hiveRefresh.isNotEmpty &&
            (_refreshTokenCache == null || _refreshTokenCache!.isEmpty)) {
          await _secure.write(key: _refreshTokenKey, value: hiveRefresh);
          _refreshTokenCache = hiveRefresh;
        }
        await _box.delete(_accessTokenKey);
        await _box.delete(_refreshTokenKey);
        _migrationAttempted = true;
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  @override
  String? getAccessToken() {
    final cached = _accessTokenCache;
    if (cached != null && cached.isNotEmpty) {
      return cached;
    }
    // Pre-migration / cold-start fallback: legacy unencrypted Hive value.
    final legacy = _box.get(_accessTokenKey);
    return legacy is String && legacy.isNotEmpty ? legacy : null;
  }

  @override
  String? getRefreshToken() {
    final cached = _refreshTokenCache;
    if (cached != null && cached.isNotEmpty) {
      return cached;
    }
    final legacy = _box.get(_refreshTokenKey);
    return legacy is String && legacy.isNotEmpty ? legacy : null;
  }

  @override
  Future<void> saveTokens(AuthTokens tokens) async {
    await ensureMigrated();
    await _secure.write(key: _accessTokenKey, value: tokens.accessToken);
    await _secure.write(key: _refreshTokenKey, value: tokens.refreshToken);
    _accessTokenCache = tokens.accessToken;
    _refreshTokenCache = tokens.refreshToken;
    await _box.delete(_accessTokenKey);
    await _box.delete(_refreshTokenKey);
  }

  @override
  Future<void> clearTokens() async {
    await ensureMigrated();
    await _secure.delete(key: _accessTokenKey);
    await _secure.delete(key: _refreshTokenKey);
    _accessTokenCache = null;
    _refreshTokenCache = null;
    await _box.delete(_accessTokenKey);
    await _box.delete(_refreshTokenKey);
  }

  @override
  UserModel? getCachedUser() {
    final userJson = _box.get(_userKey);
    if (userJson == null) return null;

    try {
      final Map<String, dynamic> userMap = jsonDecode(userJson);
      return UserModel.fromJson(userMap);
    } catch (e) {
      return null;
    }
  }

  @override
  Future<void> cacheUser(UserModel user) async {
    final userJson = jsonEncode(user.toJson());
    await _box.put(_userKey, userJson);
  }

  @override
  Future<void> clearCachedUser() async {
    await _box.delete(_userKey);
  }

  @override
  bool isAuthenticated() {
    return getAccessToken() != null;
  }
}
