import 'package:dio/dio.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:injectable/injectable.dart';

/// Module for registering external dependencies
@module
abstract class RegisterModule {
  /// Dio HTTP client
  @singleton
  Dio get dio => Dio(
        BaseOptions(
          baseUrl: 'http://localhost:8080',
          connectTimeout: const Duration(seconds: 30),
          receiveTimeout: const Duration(seconds: 30),
          headers: {
            'Content-Type': 'application/json',
          },
        ),
      );

  /// Hive database
  @singleton
  HiveInterface get hive => Hive;

  /// Secure storage
  @singleton
  @preResolve
  Future<Box> get secureStorage async {
    return await Hive.openBox('secure_storage');
  }
}
