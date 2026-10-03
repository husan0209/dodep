// GENERATED CODE - DO NOT MODIFY BY HAND

// **************************************************************************
// InjectableConfigGenerator
// **************************************************************************

// ignore_for_file: type=lint
// coverage:ignore-file

// ignore_for_file: no_leading_underscores_for_library_prefixes
import 'package:dio/dio.dart' as _i5;
import 'package:get_it/get_it.dart' as _i1;
import 'package:hive_flutter/hive_flutter.dart' as _i4;
import 'package:injectable/injectable.dart' as _i2;

import '../../features/affiliate/data/datasources/affiliate_remote_datasource.dart'
    as _i11;
import '../../features/affiliate/data/datasources/affiliate_remote_datasource_impl.dart'
    as _i12;
import '../../features/affiliate/data/repositories/affiliate_repository_impl.dart'
    as _i14;
import '../../features/affiliate/domain/repositories/affiliate_repository.dart'
    as _i13;
import '../../features/affiliate/presentation/bloc/affiliate_bloc.dart' as _i25;
import '../../features/auth/data/datasources/auth_local_datasource.dart'
    as _i15;
import '../../features/auth/data/datasources/auth_local_datasource_impl.dart'
    as _i16;
import '../../features/auth/data/datasources/auth_remote_datasource.dart'
    as _i17;
import '../../features/auth/data/datasources/auth_remote_datasource_impl.dart'
    as _i18;
import '../../features/auth/data/repositories/auth_repository_impl.dart'
    as _i20;
import '../../features/auth/domain/repositories/auth_repository.dart' as _i19;
import '../../features/auth/domain/usecases/login.dart' as _i21;
import '../../features/auth/domain/usecases/logout.dart' as _i22;
import '../../features/auth/domain/usecases/register.dart' as _i24;
import '../../features/auth/presentation/bloc/auth_bloc.dart' as _i26;
import '../../features/responsible_gambling/data/datasources/rg_remote_datasource.dart'
    as _i6;
import '../../features/responsible_gambling/data/datasources/rg_remote_datasource_impl.dart'
    as _i7;
import '../../features/responsible_gambling/data/repositories/rg_repository_impl.dart'
    as _i9;
import '../../features/responsible_gambling/domain/repositories/rg_repository.dart'
    as _i8;
import '../../features/responsible_gambling/presentation/bloc/rg_bloc.dart'
    as _i23;
import '../network/api_client.dart' as _i3;
import '../network/ws_client.dart' as _i10;
import 'register_module.dart' as _i27;

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
    await gh.singletonAsync<_i4.Box<dynamic>>(
      () => registerModule.secureStorage,
      preResolve: true,
    );
    gh.singleton<_i5.Dio>(() => registerModule.dio);
    gh.singleton<_i4.HiveInterface>(() => registerModule.hive);
    gh.lazySingleton<_i6.RGRemoteDataSource>(
        () => _i7.RGRemoteDataSourceImpl(dio: gh<_i5.Dio>()));
    gh.lazySingleton<_i8.RGRepository>(
        () => _i9.RGRepositoryImpl(remote: gh<_i6.RGRemoteDataSource>()));
    gh.singleton<_i10.WsClient>(() => _i10.WsClient());
    gh.lazySingleton<_i11.AffiliateRemoteDataSource>(
        () => _i12.AffiliateRemoteDataSourceImpl(dio: gh<_i5.Dio>()));
    gh.lazySingleton<_i13.AffiliateRepository>(() =>
        _i14.AffiliateRepositoryImpl(
            remote: gh<_i11.AffiliateRemoteDataSource>()));
    gh.lazySingleton<_i15.AuthLocalDataSource>(
        () => _i16.AuthLocalDataSourceImpl(box: gh<_i4.Box<dynamic>>()));
    gh.lazySingleton<_i17.AuthRemoteDataSource>(
        () => _i18.AuthRemoteDataSourceImpl(dio: gh<_i5.Dio>()));
    gh.lazySingleton<_i19.AuthRepository>(() => _i20.AuthRepositoryImpl(
          remoteDataSource: gh<_i17.AuthRemoteDataSource>(),
          localDataSource: gh<_i15.AuthLocalDataSource>(),
        ));
    gh.factory<_i21.Login>(() => _i21.Login(gh<_i19.AuthRepository>()));
    gh.factory<_i22.Logout>(() => _i22.Logout(gh<_i19.AuthRepository>()));
    gh.factory<_i23.RGBloc>(
        () => _i23.RGBloc(repository: gh<_i8.RGRepository>()));
    gh.factory<_i24.Register>(() => _i24.Register(gh<_i19.AuthRepository>()));
    gh.factory<_i25.AffiliateBloc>(
        () => _i25.AffiliateBloc(repository: gh<_i13.AffiliateRepository>()));
    gh.factory<_i26.AuthBloc>(() => _i26.AuthBloc(
          login: gh<_i21.Login>(),
          register: gh<_i24.Register>(),
          logout: gh<_i22.Logout>(),
        ));
    return this;
  }
}

class _$RegisterModule extends _i27.RegisterModule {}
