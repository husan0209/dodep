import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../features/auth/presentation/bloc/auth_state.dart';
import '../features/auth/presentation/pages/login_page.dart';
import '../features/auth/presentation/pages/register_page.dart';
import '../features/sportsbook/presentation/pages/sportsbook_page.dart';
import '../features/casino/presentation/pages/casino_page.dart';
import '../features/wallet/presentation/pages/wallet_page.dart';
import '../features/profile/presentation/pages/profile_page.dart';
import '../features/bonuses/presentation/pages/bonuses_page.dart';
import '../features/notifications/presentation/pages/notifications_page.dart';
import 'main_shell.dart';

/// App router configuration
GoRouter appRouter({required AuthState authState}) {
  final isAuthenticated = authState is AuthAuthenticated;

  return GoRouter(
    initialLocation: isAuthenticated ? '/sportsbook' : '/auth/login',
    redirect: (context, state) {
      final currentAuthState = context.read<AuthBloc>().state;
      final isCurrentlyAuthenticated = currentAuthState is AuthAuthenticated;
      final isAuthPage = state.matchedLocation.startsWith('/auth');

      if (!isCurrentlyAuthenticated && !isAuthPage) {
        return '/auth/login';
      }
      if (isCurrentlyAuthenticated && isAuthPage) {
        return '/sportsbook';
      }
      return null;
    },
    routes: [
      // Auth routes
      GoRoute(
        path: '/auth/login',
        name: 'login',
        builder: (context, state) => const LoginPage(),
      ),
      GoRoute(
        path: '/auth/register',
        name: 'register',
        builder: (context, state) => const RegisterPage(),
      ),

      // Main app shell with bottom navigation
      ShellRoute(
        builder: (context, state, child) => MainShell(child: child),
        routes: [
          GoRoute(
            path: '/sportsbook',
            name: 'sportsbook',
            pageBuilder: (context, state) => const NoTransitionPage(child: SportsbookPage()),
          ),
          GoRoute(
            path: '/casino',
            name: 'casino',
            pageBuilder: (context, state) => const NoTransitionPage(child: CasinoPage()),
          ),
          GoRoute(
            path: '/wallet',
            name: 'wallet',
            pageBuilder: (context, state) => const NoTransitionPage(child: WalletPage()),
          ),
          GoRoute(
            path: '/profile',
            name: 'profile',
            pageBuilder: (context, state) => const NoTransitionPage(child: ProfilePage()),
          ),
          GoRoute(
            path: '/bonuses',
            name: 'bonuses',
            pageBuilder: (context, state) => const NoTransitionPage(child: BonusesPage()),
          ),
          GoRoute(
            path: '/notifications',
            name: 'notifications',
            pageBuilder: (context, state) => const NoTransitionPage(child: NotificationsPage()),
          ),
        ],
      ),
    ],
  );
}

/// Simple page without transition
class NoTransitionPage extends CustomTransitionPage<void> {
  const NoTransitionPage({
    required super.child,
    super.name,
    super.arguments,
    super.restorationId,
  }) : super(
          transitionsBuilder: (context, animation, secondaryAnimation, child) => child,
        );
}
