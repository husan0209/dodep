import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'core/config/app_config.dart';
import 'core/di/injection.dart';
import 'core/theme/app_theme.dart';
import 'core/router/app_router.dart';
import 'features/auth/presentation/bloc/auth_bloc.dart';
import 'features/auth/presentation/bloc/auth_state.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Initialize Hive
  await Hive.initFlutter();
  await Hive.openBox('app_storage');
  await Hive.openBox('secure_storage');

  // Initialize dependencies
  await configureDependencies();

  // Get auth state from local storage
  final authBloc = getIt<AuthBloc>();
  final localDataSource = getIt();
  
  if (localDataSource.isAuthenticated()) {
    authBloc.add(const AuthEvent.getCurrentUser());
  }

  runApp(
    MultiBlocProvider(
      providers: [
        BlocProvider<AuthBloc>.value(value: authBloc),
      ],
      child: const DODApp(),
    ),
  );
}

class DODApp extends StatelessWidget {
  const DODApp({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<AuthBloc, AuthState>(
      builder: (context, authState) {
        return MaterialApp.router(
          title: 'DOD',
          debugShowCheckedModeBanner: false,
          theme: AppTheme.lightTheme,
          darkTheme: AppTheme.darkTheme,
          themeMode: ThemeMode.dark,
          routerConfig: appRouter(authState: authState),
        );
      },
    );
  }
}
