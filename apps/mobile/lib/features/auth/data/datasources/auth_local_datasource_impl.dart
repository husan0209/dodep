import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:injectable/injectable.dart';

import '../../../core/error/exceptions.dart';
import '../models/auth_tokens.dart';
import '../models/user_model.dart';
import 'auth_local_datasource.dart';

/// Implementation of AuthLocalDataSource
@LazySingleton(as: AuthLocalDataSource)
class AuthLocalDataSourceImpl implements AuthLocalDataSource {
  final FlutterSecureStorage _secureStorage;
  final Box _box;

  AuthLocalDataSourceImpl({
    required FlutterSecureStorage secureStorage,
    required Box box,
  })  : _secureStorage = secureStorage,
        _box = box;

  static const String _accessTokenKey = 'access_token';
  static const String _refreshTokenKey = 'refresh_token';
  static const String _userKey = 'cached_user';

  @override
  String? getAccessToken() {
    return _box.get(_accessTokenKey);
  }

  @override
  String? getRefreshToken() {
    return _box.get(_refreshTokenKey);
  }

  @override
  Future<void> saveTokens(AuthTokens tokens) async {
    await _box.put(_accessTokenKey, tokens.accessToken);
    await _box.put(_refreshTokenKey, tokens.refreshToken);
  }

  @override
  Future<void> clearTokens() async {
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
