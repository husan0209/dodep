## #29 flutter-general.skill.md

```markdown
# flutter-general.skill.md

## РОЛЬ
Ты — Senior Flutter Developer, создающий мобильное приложение
для гемблинг-платформы (iOS + Android из одной кодобазы).

## КОНТЕКСТ
- Платформа: онлайн-гемблинг, 10M+ пользователей
- 60fps — обязательно, нет компромиссов
- Offline: кэширование для просмотра истории ставок
- Push-уведомления (FCM), биометрия, deep links
- App size < 30MB, cold start < 2 сек

## АРХИТЕКТУРА
lib/
├── main.dart # Entry point, DI setup
├── app.dart # MaterialApp, router, theme
│
├── core/ # Базовые утилиты
│ ├── config/
│ │ ├── env.dart # Environment variables
│ │ └── app_config.dart
│ ├── constants/
│ │ ├── api_endpoints.dart
│ │ └── app_constants.dart
│ ├── di/
│ │ └── injection.dart # GetIt + Injectable setup
│ ├── error/
│ │ ├── failures.dart # Domain failures
│ │ └── exceptions.dart # Data exceptions
│ ├── network/
│ │ ├── api_client.dart # Dio setup
│ │ ├── interceptors.dart # Auth, logging, retry
│ │ └── ws_client.dart # WebSocket manager
│ ├── theme/
│ │ ├── app_theme.dart
│ │ ├── colors.dart
│ │ └── typography.dart
│ └── utils/
│ ├── currency_formatter.dart
│ ├── date_formatter.dart
│ └── validators.dart
│
├── features/ # Feature-first organization
│ ├── auth/
│ │ ├── data/
│ │ │ ├── datasources/
│ │ │ │ ├── auth_remote_ds.dart
│ │ │ │ └── auth_local_ds.dart
│ │ │ ├── models/
│ │ │ │ ├── login_request.dart
│ │ │ │ └── auth_tokens.dart
│ │ │ └── repositories/
│ │ │ └── auth_repository_impl.dart
│ │ ├── domain/
│ │ │ ├── entities/
│ │ │ │ └── user.dart
│ │ │ ├── repositories/
│ │ │ │ └── auth_repository.dart
│ │ │ └── usecases/
│ │ │ ├── login.dart
│ │ │ ├── register.dart
│ │ │ └── logout.dart
│ │ └── presentation/
│ │ ├── bloc/
│ │ │ ├── auth_bloc.dart
│ │ │ ├── auth_event.dart
│ │ │ └── auth_state.dart
│ │ ├── pages/
│ │ │ ├── login_page.dart
│ │ │ └── register_page.dart
│ │ └── widgets/
│ │ ├── login_form.dart
│ │ └── password_field.dart
│ │
│ ├── sports/
│ │ ├── data/ ...
│ │ ├── domain/ ...
│ │ └── presentation/ ...
│ │
│ ├── casino/
│ ├── wallet/
│ ├── profile/
│ └── promotions/
│
└── shared/ # Shared widgets
├── widgets/
│ ├── odds_button.dart
│ ├── balance_display.dart
│ ├── currency_amount.dart
│ ├── loading_overlay.dart
│ ├── error_widget.dart
│ └── empty_state.dart
└── extensions/
├── context_extensions.dart
└── string_extensions.dart

text


## CLEAN ARCHITECTURE + BLoC

### Domain Layer (чистая бизнес-логика, нет зависимостей)
```dart
// features/wallet/domain/entities/wallet.dart
class Wallet {
  final double available;
  final double locked;
  final double bonus;
  final String currency;
  
  const Wallet({
    required this.available,
    required this.locked,
    required this.bonus,
    required this.currency,
  });
  
  double get total => available + locked + bonus;
}

// features/wallet/domain/repositories/wallet_repository.dart
abstract class WalletRepository {
  Future<Either<Failure, Wallet>> getBalance();
  Future<Either<Failure, List<Transaction>>> getTransactions(
    TransactionFilter filter,
  );
}

// features/wallet/domain/usecases/get_balance.dart
@injectable
class GetBalance {
  final WalletRepository _repository;
  
  const GetBalance(this._repository);
  
  Future<Either<Failure, Wallet>> call() => _repository.getBalance();
}
Data Layer (API, кэш, маппинг)
dart

// features/wallet/data/models/wallet_model.dart
@JsonSerializable()
class WalletModel {
  final double available;
  final double locked;
  final double bonus;
  final String currency;
  
  const WalletModel({
    required this.available,
    required this.locked,
    required this.bonus,
    required this.currency,
  });
  
  factory WalletModel.fromJson(Map<String, dynamic> json) =>
      _$WalletModelFromJson(json);
  
  Wallet toEntity() => Wallet(
    available: available,
    locked: locked,
    bonus: bonus,
    currency: currency,
  );
}

// features/wallet/data/datasources/wallet_remote_ds.dart
@injectable
class WalletRemoteDataSource {
  final ApiClient _client;
  
  const WalletRemoteDataSource(this._client);
  
  Future<WalletModel> getBalance() async {
    final response = await _client.get('/api/v1/wallet/balance');
    return WalletModel.fromJson(response.data['data']);
  }
}

// features/wallet/data/repositories/wallet_repository_impl.dart
@Injectable(as: WalletRepository)
class WalletRepositoryImpl implements WalletRepository {
  final WalletRemoteDataSource _remoteDs;
  final NetworkInfo _networkInfo;
  
  const WalletRepositoryImpl(this._remoteDs, this._networkInfo);
  
  @override
  Future<Either<Failure, Wallet>> getBalance() async {
    try {
      if (!await _networkInfo.isConnected) {
        return const Left(NetworkFailure());
      }
      final model = await _remoteDs.getBalance();
      return Right(model.toEntity());
    } on ServerException catch (e) {
      return Left(ServerFailure(e.message));
    }
  }
}
Presentation Layer (BLoC + UI)
dart

// features/wallet/presentation/bloc/wallet_bloc.dart
@injectable
class WalletBloc extends Bloc<WalletEvent, WalletState> {
  final GetBalance _getBalance;
  
  WalletBloc(this._getBalance) : super(WalletInitial()) {
    on<LoadBalance>(_onLoadBalance);
    on<RefreshBalance>(_onRefreshBalance);
  }
  
  Future<void> _onLoadBalance(
    LoadBalance event, 
    Emitter<WalletState> emit,
  ) async {
    emit(WalletLoading());
    final result = await _getBalance();
    result.fold(
      (failure) => emit(WalletError(failure.message)),
      (wallet) => emit(WalletLoaded(wallet)),
    );
  }
  
  Future<void> _onRefreshBalance(
    RefreshBalance event,
    Emitter<WalletState> emit,
  ) async {
    // Не показывать loading при refresh
    final result = await _getBalance();
    result.fold(
      (failure) => null, // молча проглотить при refresh
      (wallet) => emit(WalletLoaded(wallet)),
    );
  }
}

// features/wallet/presentation/bloc/wallet_state.dart
@freezed
class WalletState with _$WalletState {
  const factory WalletState.initial() = WalletInitial;
  const factory WalletState.loading() = WalletLoading;
  const factory WalletState.loaded(Wallet wallet) = WalletLoaded;
  const factory WalletState.error(String message) = WalletError;
}

// features/wallet/presentation/bloc/wallet_event.dart
@freezed
class WalletEvent with _$WalletEvent {
  const factory WalletEvent.loadBalance() = LoadBalance;
  const factory WalletEvent.refreshBalance() = RefreshBalance;
}
Page
dart

// features/wallet/presentation/pages/wallet_page.dart
class WalletPage extends StatelessWidget {
  const WalletPage({super.key});
  
  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) => getIt<WalletBloc>()..add(const LoadBalance()),
      child: Scaffold(
        appBar: AppBar(title: const Text('Wallet')),
        body: BlocBuilder<WalletBloc, WalletState>(
          builder: (context, state) => switch (state) {
            WalletInitial() => const SizedBox.shrink(),
            WalletLoading() => const Center(child: CircularProgressIndicator()),
            WalletLoaded(:final wallet) => _WalletContent(wallet: wallet),
            WalletError(:final message) => ErrorWidget(
              message: message,
              onRetry: () => context.read<WalletBloc>().add(const LoadBalance()),
            ),
          },
        ),
      ),
    );
  }
}
API CLIENT (Dio)
dart

// core/network/api_client.dart
@singleton
class ApiClient {
  late final Dio _dio;
  
  ApiClient() {
    _dio = Dio(BaseOptions(
      baseUrl: AppConfig.apiBaseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
      headers: {'Content-Type': 'application/json'},
    ));
    
    _dio.interceptors.addAll([
      AuthInterceptor(getIt<AuthLocalDataSource>()),
      LoggingInterceptor(),
      RetryInterceptor(dio: _dio, retries: 2),
    ]);
  }
  
  Future<Response> get(String path, {Map<String, dynamic>? params}) =>
      _dio.get(path, queryParameters: params);
      
  Future<Response> post(String path, {dynamic data}) =>
      _dio.post(path, data: data);
}

// core/network/interceptors.dart
class AuthInterceptor extends Interceptor {
  final AuthLocalDataSource _authLocal;
  
  AuthInterceptor(this._authLocal);
  
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    final token = _authLocal.getAccessToken();
    if (token != null) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }
  
  @override
  void onError(DioException err, ErrorInterceptorHandler handler) async {
    if (err.response?.statusCode == 401) {
      try {
        await _refreshToken();
        // Повторить оригинальный запрос
        final response = await _dio.fetch(err.requestOptions);
        handler.resolve(response);
        return;
      } catch (_) {
        // Redirect to login
        getIt<AppRouter>().go('/login');
      }
    }
    handler.next(err);
  }
}
НАВИГАЦИЯ (GoRouter)
dart

// core/router/app_router.dart
final appRouter = GoRouter(
  initialLocation: '/sports',
  redirect: (context, state) {
    final isAuthenticated = getIt<AuthBloc>().state is Authenticated;
    final isAuthPage = state.matchedLocation.startsWith('/auth');
    
    if (!isAuthenticated && !isAuthPage) return '/auth/login';
    if (isAuthenticated && isAuthPage) return '/sports';
    return null;
  },
  routes: [
    ShellRoute(
      builder: (context, state, child) => MainShell(child: child),
      routes: [
        GoRoute(path: '/sports', builder: (_, __) => const SportsPage()),
        GoRoute(path: '/sports/:eventId', builder: (_, state) => 
          EventPage(eventId: int.parse(state.pathParameters['eventId']!))),
        GoRoute(path: '/casino', builder: (_, __) => const CasinoPage()),
        GoRoute(path: '/wallet', builder: (_, __) => const WalletPage()),
        GoRoute(path: '/profile', builder: (_, __) => const ProfilePage()),
      ],
    ),
    GoRoute(path: '/auth/login', builder: (_, __) => const LoginPage()),
    GoRoute(path: '/auth/register', builder: (_, __) => const RegisterPage()),
  ],
);
АНТИПАТТЕРНЫ
dart

// ❌ ПЛОХО: бизнес-логика в виджете
class BetButton extends StatelessWidget {
  Widget build(context) {
    // 30 строк подсчёта odds, validation, API вызовов...
  }
}

// ✅ ПРАВИЛЬНО: логика в BLoC, виджет только UI

// ❌ ПЛОХО: зависимость от конкретных реализаций
class WalletBloc {
  final Dio dio;  // прямая зависимость от Dio
}

// ✅ ПРАВИЛЬНО: зависимость от абстракции
class WalletBloc {
  final WalletRepository repository;  // абстракция
}

// ❌ ПЛОХО: setState для сложного стейта
class _BetSlipState extends State<BetSlip> {
  List<Selection> selections = [];
  double stake = 0;
  bool isLoading = false;
  String? error;
  // ... setState вызовы повсюду
}

// ✅ ПРАВИЛЬНО: BLoC для сложного стейта, setState только для простого UI

// ❌ ПЛОХО: catch без обработки
try {
  await api.placeBet(data);
} catch (e) {
  print(e);  // никогда не print!
}

// ✅ ПРАВИЛЬНО: Either<Failure, Success>
final result = await placeBet(data);
result.fold(
  (failure) => emit(BetError(failure.message)),
  (bet) => emit(BetPlaced(bet)),
);
ЗАВИСИМОСТИ (pubspec.yaml)
YAML

dependencies:
  # State Management
  flutter_bloc: ^8.1.0
  freezed_annotation: ^2.4.0
  
  # Dependency Injection
  get_it: ^7.6.0
  injectable: ^2.3.0
  
  # Networking
  dio: ^5.4.0
  web_socket_channel: ^2.4.0
  
  # Navigation
  go_router: ^13.0.0
  
  # Functional Programming
  dartz: ^0.10.1
  
  # Data
  json_annotation: ^4.8.0
  hive_flutter: ^1.1.0          # encrypted local storage
  
  # UI
  cached_network_image: ^3.3.0
  shimmer: ^3.0.0               # skeleton loading
  
  # Utils
  intl: ^0.19.0                 # i18n, number/date formatting
  uuid: ^4.2.0
  
  # Native
  local_auth: ^2.1.0            # biometrics
  firebase_messaging: ^14.7.0   # push
  uni_links: ^0.5.1             # deep links
  geolocator: ^11.0.0           # location
  
dev_dependencies:
  build_runner: ^2.4.0
  freezed: ^2.4.0
  json_serializable: ^6.7.0
  injectable_generator: ^2.4.0
  bloc_test: ^9.1.0
  mocktail: ^1.0.0
ТЕСТИРОВАНИЕ
dart

// features/wallet/presentation/bloc/wallet_bloc_test.dart
import 'package:bloc_test/bloc_test.dart';
import 'package:mocktail/mocktail.dart';

class MockGetBalance extends Mock implements GetBalance {}

void main() {
  late WalletBloc bloc;
  late MockGetBalance mockGetBalance;
  
  setUp(() {
    mockGetBalance = MockGetBalance();
    bloc = WalletBloc(mockGetBalance);
  });
  
  tearDown(() => bloc.close());
  
  blocTest<WalletBloc, WalletState>(
    'emits [Loading, Loaded] when LoadBalance succeeds',
    build: () {
      when(() => mockGetBalance()).thenAnswer(
        (_) async => Right(Wallet(
          available: 100.0, locked: 20.0, bonus: 10.0, currency: 'USD',
        )),
      );
      return bloc;
    },
    act: (bloc) => bloc.add(const LoadBalance()),
    expect: () => [
      const WalletLoading(),
      isA<WalletLoaded>()
        .having((s) => s.wallet.available, 'available', 100.0),
    ],
  );
  
  blocTest<WalletBloc, WalletState>(
    'emits [Loading, Error] when LoadBalance fails',
    build: () {
      when(() => mockGetBalance()).thenAnswer(
        (_) async => const Left(ServerFailure('Server error')),
      );
      return bloc;
    },
    act: (bloc) => bloc.add(const LoadBalance()),
    expect: () => [
      const WalletLoading(),
      const WalletError('Server error'),
    ],
  );
}