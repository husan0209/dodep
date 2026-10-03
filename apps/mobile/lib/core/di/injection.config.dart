// GENERATED CODE - DO NOT MODIFY BY HAND

// **************************************************************************
// InjectableConfigGenerator
// **************************************************************************

// ignore_for_file: type=lint
// coverage:ignore-file

// ignore_for_file: no_leading_underscores_for_library_prefixes
import 'package:dio/dio.dart' as _i9;
import 'package:flutter_secure_storage/flutter_secure_storage.dart' as _i21;
import 'package:get_it/get_it.dart' as _i1;
import 'package:hive_flutter/hive_flutter.dart' as _i8;
import 'package:injectable/injectable.dart' as _i2;

import '../../features/affiliate/data/datasources/affiliate_remote_datasource.dart'
    as _i15;
import '../../features/affiliate/data/datasources/affiliate_remote_datasource_impl.dart'
    as _i16;
import '../../features/affiliate/data/repositories/affiliate_repository_impl.dart'
    as _i18;
import '../../features/affiliate/domain/repositories/affiliate_repository.dart'
    as _i17;
import '../../features/affiliate/presentation/bloc/affiliate_bloc.dart' as _i27;
import '../../features/auth/data/datasources/auth_local_datasource.dart'
    as _i19;
import '../../features/auth/data/datasources/auth_local_datasource_impl.dart'
    as _i20;
import '../../features/auth/data/datasources/auth_remote_datasource.dart'
    as _i22;
import '../../features/auth/data/datasources/auth_remote_datasource_impl.dart'
    as _i23;
import '../../features/auth/data/repositories/auth_repository_impl.dart'
    as _i25;
import '../../features/auth/domain/repositories/auth_repository.dart' as _i24;
import '../../features/auth/domain/usecases/login.dart' as _i5;
import '../../features/auth/domain/usecases/logout.dart' as _i7;
import '../../features/auth/domain/usecases/register.dart' as _i6;
import '../../features/auth/presentation/bloc/auth_bloc.dart' as _i4;
import '../../features/responsible_gambling/data/datasources/rg_remote_datasource.dart'
    as _i10;
import '../../features/responsible_gambling/data/datasources/rg_remote_datasource_impl.dart'
    as _i11;
import '../../features/responsible_gambling/data/repositories/rg_repository_impl.dart'
    as _i13;
import '../../features/responsible_gambling/domain/repositories/rg_repository.dart'
    as _i12;
import '../../features/responsible_gambling/presentation/bloc/rg_bloc.dart'
    as _i26;
import '../network/api_client.dart' as _i3;
import '../network/ws_client.dart' as _i14;
import 'register_module.dart' as _i28;

extension GetItInjectableX on _i1.GetIt {
// initializes the registration of main-scope dependencies inside of GetIt
  Future<_i1.GetIt> init({
    String? environment,
    _i2.EnvironmentFilter? environmentFilter,
  }) async {
    final gh = _i2.GetItHelper(
      this,
      environment,
      environmentFilter,
    );
    final registerModule = _$RegisterModule();
    gh.singleton<_i3.ApiClient>(() => _i3.ApiClient());
    gh.factory<_i4.AuthBloc>(() => _i4.AuthBloc(
          login: gh<_i5.Login>(),
          register: gh<_i6.Register>(),
          logout: gh<_i7.Logout>(),
        ));
    await gh.singletonAsync<_i8.Box<dynamic>>(
      () => registerModule.secureStorage,
      preResolve: true,
    );
    gh.singleton<_i9.Dio>(() => registerModule.dio);
    gh.singleton<_i8.HiveInterface>(() => registerModule.hive);
    gh.lazySingleton<_i10.RGRemoteDataSource>(
        () => _i11.RGRemoteDataSourceImpl(dio: gh<_i9.Dio>()));
    gh.lazySingleton<_i12.RGRepository>(
        () => _i13.RGRepositoryImpl(remote: gh<_i10.RGRemoteDataSource>()));
    gh.singleton<_i14.WsClient>(() => _i14.WsClient());
    gh.lazySingleton<_i15.AffiliateRemoteDataSource>(
        () => _i16.AffiliateRemoteDataSourceImpl(dio: gh<_i9.Dio>()));
    gh.lazySingleton<_i17.AffiliateRepository>(() =>
        _i18.AffiliateRepositoryImpl(
            remote: gh<_i15.AffiliateRemoteDataSource>()));
    gh.lazySingleton<_i19.AuthLocalDataSource>(
        () => _i20.AuthLocalDataSourceImpl(
              secureStorage: gh<_i21.FlutterSecureStorage>(),
              box: gh<_i8.Box<dynamic>>(),
            ));
    gh.lazySingleton<_i22.AuthRemoteDataSource>(
        () => _i23.AuthRemoteDataSourceImpl(dio: gh<_i9.Dio>()));
    gh.lazySingleton<_i24.AuthRepository>(() => _i25.AuthRepositoryImpl(
          remoteDataSource: gh<_i22.AuthRemoteDataSource>(),
          localDataSource: gh<_i19.AuthLocalDataSource>(),
        ));
    gh.factory<_i26.RGBloc>(
        () => _i26.RGBloc(repository: gh<_i12.RGRepository>()));
    gh.factory<_i27.AffiliateBloc>(
        () => _i27.AffiliateBloc(repository: gh<_i17.AffiliateRepository>()));
    return this;
  }
}

class _$RegisterModule extends _i28.RegisterModule {}
