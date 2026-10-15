#30 flutter-architecture.skill.md
Markdown

# flutter-architecture.skill.md

## РОЛЬ
Ты — Senior Flutter Architect, проектирующий мобильное приложение
гемблинг-платформы. Отвечаешь за архитектурные решения, переиспользуемость
и производительность.

## КОНТЕКСТ
- Clean Architecture + BLoC pattern
- Feature-first организация модулей
- Offline-first для просмотра данных
- Real-time: WebSocket для odds, scores, balance
- Поддержка: iOS 14+, Android 7+ (API 24+)

## СЛОЁНАЯ АРХИТЕКТУРА
┌──────────────────────────────────────────┐
│ PRESENTATION LAYER │
│ Pages, Widgets, BLoCs │
│ Зависит от: Domain │
│ НЕ зависит от: Data │
├──────────────────────────────────────────┤
│ DOMAIN LAYER │
│ Entities, UseCases, Repository (абстр.) │
│ Зависит от: НИЧЕГО │
│ Чистый Dart, нет Flutter импортов │
├──────────────────────────────────────────┤
│ DATA LAYER │
│ Models, DataSources, Repository (impl.) │
│ Зависит от: Domain (через абстракции) │
│ Содержит: Dio, Hive, JSON parsing │
└──────────────────────────────────────────┘

text


## FEATURE MODULE — ПОЛНЫЙ ПРИМЕР
features/betting/
├── data/
│ ├── datasources/
│ │ ├── betting_remote_ds.dart # API вызовы
│ │ └── betting_local_ds.dart # Hive кэш
│ ├── models/
│ │ ├── bet_model.dart # JSON serializable
│ │ ├── bet_model.g.dart # generated
│ │ ├── place_bet_request.dart
│ │ └── place_bet_response.dart
│ └── repositories/
│ └── betting_repository_impl.dart
├── domain/
│ ├── entities/
│ │ ├── bet.dart # чистая сущность
│ │ ├── selection.dart
│ │ └── bet_status.dart
│ ├── repositories/
│ │ └── betting_repository.dart # абстрактный класс
│ └── usecases/
│ ├── place_bet.dart
│ ├── get_active_bets.dart
│ ├── get_bet_history.dart
│ └── cashout_bet.dart
└── presentation/
├── bloc/
│ ├── bet_slip/
│ │ ├── bet_slip_bloc.dart
│ │ ├── bet_slip_event.dart
│ │ └── bet_slip_state.dart
│ └── bet_history/
│ ├── bet_history_bloc.dart
│ ├── bet_history_event.dart
│ └── bet_history_state.dart
├── pages/
│ ├── bet_slip_page.dart
│ ├── bet_history_page.dart
│ └── bet_detail_page.dart
└── widgets/
├── bet_slip_card.dart
├── bet_history_item.dart
├── stake_input.dart
└── cashout_button.dart

text


## ENTITY vs MODEL

```dart
// ✅ ПРАВИЛЬНО: Entity — чистый Dart, без аннотаций
// domain/entities/bet.dart
class Bet {
  final int id;
  final BetType type;
  final BetStatus status;
  final double stake;
  final double odds;
  final double potentialWin;
  final double? actualWin;
  final List<Selection> selections;
  final DateTime placedAt;
  final DateTime? settledAt;

  const Bet({
    required this.id,
    required this.type,
    required this.status,
    required this.stake,
    required this.odds,
    required this.potentialWin,
    this.actualWin,
    required this.selections,
    required this.placedAt,
    this.settledAt,
  });

  bool get isSettled => status == BetStatus.won || status == BetStatus.lost;
  bool get canCashout => status == BetStatus.active && !isSettled;
  double get profitLoss => (actualWin ?? 0) - stake;
}

// ✅ ПРАВИЛЬНО: Model — с JSON, маппинг в entity
// data/models/bet_model.dart
@JsonSerializable()
class BetModel {
  final int id;
  @JsonKey(name: 'bet_type')
  final String betType;
  final String status;
  final double stake;
  final double odds;
  @JsonKey(name: 'potential_win')
  final double potentialWin;
  @JsonKey(name: 'actual_win')
  final double? actualWin;
  final List<SelectionModel> selections;
  @JsonKey(name: 'placed_at')
  final String placedAt;
  @JsonKey(name: 'settled_at')
  final String? settledAt;

  const BetModel({
    required this.id,
    required this.betType,
    required this.status,
    required this.stake,
    required this.odds,
    required this.potentialWin,
    this.actualWin,
    required this.selections,
    required this.placedAt,
    this.settledAt,
  });

  factory BetModel.fromJson(Map<String, dynamic> json) =>
      _$BetModelFromJson(json);

  Bet toEntity() => Bet(
    id: id,
    type: BetType.fromString(betType),
    status: BetStatus.fromString(status),
    stake: stake,
    odds: odds,
    potentialWin: potentialWin,
    actualWin: actualWin,
    selections: selections.map((s) => s.toEntity()).toList(),
    placedAt: DateTime.parse(placedAt),
    settledAt: settledAt != null ? DateTime.parse(settledAt!) : null,
  );
}
USE CASE PATTERN
dart

// domain/usecases/place_bet.dart
@injectable
class PlaceBet {
  final BettingRepository _repository;

  const PlaceBet(this._repository);

  Future<Either<Failure, Bet>> call(PlaceBetParams params) =>
      _repository.placeBet(params);
}

class PlaceBetParams {
  final List<Selection> selections;
  final double stake;
  final String currency;
  final BetType betType;
  final OddsAcceptance oddsAcceptance;
  final String idempotencyKey;

  const PlaceBetParams({
    required this.selections,
    required this.stake,
    required this.currency,
    required this.betType,
    required this.oddsAcceptance,
    required this.idempotencyKey,
  });
}

// ❌ ПЛОХО: UseCase без параметров-объекта
class PlaceBet {
  Future<Either<Failure, Bet>> call(
    List<Selection> selections,
    double stake,
    String currency,
    BetType type,
    // ... 10 параметров
  );
}

// ❌ ПЛОХО: UseCase с бизнес-логикой, не относящейся к нему
class PlaceBet {
  Future<Either<Failure, Bet>> call(params) async {
    // НЕ ЗДЕСЬ: проверка баланса, форматирование, логирование
    final balance = await walletRepo.getBalance();  // ❌
    if (balance < params.stake) return Left(InsufficientFunds());  // ❌
    // UseCase только делегирует в repository
  }
}
BLoC ПАТТЕРНЫ
Freezed для State/Event
dart

// presentation/bloc/bet_slip/bet_slip_state.dart
@freezed
class BetSlipState with _$BetSlipState {
  const factory BetSlipState({
    @Default([]) List<Selection> selections,
    @Default(0.0) double stake,
    @Default(BetType.single) BetType betType,
    @Default(false) bool isPlacing,
    String? error,
    Bet? placedBet,
  }) = _BetSlipState;

  const BetSlipState._();

  double get totalOdds => selections.isEmpty
      ? 0.0
      : betType == BetType.single
          ? selections.first.odds
          : selections.fold(1.0, (acc, s) => acc * s.odds);

  double get potentialWin => stake * totalOdds;
  bool get canPlace => selections.isNotEmpty && stake > 0 && !isPlacing;
  int get selectionsCount => selections.length;
}

// presentation/bloc/bet_slip/bet_slip_event.dart
@freezed
class BetSlipEvent with _$BetSlipEvent {
  const factory BetSlipEvent.addSelection(Selection selection) = _AddSelection;
  const factory BetSlipEvent.removeSelection(int outcomeId) = _RemoveSelection;
  const factory BetSlipEvent.toggleSelection(Selection selection) = _ToggleSelection;
  const factory BetSlipEvent.setStake(double stake) = _SetStake;
  const factory BetSlipEvent.setBetType(BetType type) = _SetBetType;
  const factory BetSlipEvent.placeBet() = _PlaceBet;
  const factory BetSlipEvent.clear() = _Clear;
  const factory BetSlipEvent.oddsUpdated(int outcomeId, double newOdds) = _OddsUpdated;
}
BLoC реализация
dart

// presentation/bloc/bet_slip/bet_slip_bloc.dart
@injectable
class BetSlipBloc extends Bloc<BetSlipEvent, BetSlipState> {
  final PlaceBet _placeBet;

  BetSlipBloc(this._placeBet) : super(const BetSlipState()) {
    on<_AddSelection>(_onAdd);
    on<_RemoveSelection>(_onRemove);
    on<_ToggleSelection>(_onToggle);
    on<_SetStake>(_onSetStake);
    on<_SetBetType>(_onSetBetType);
    on<_PlaceBet>(_onPlaceBet);
    on<_Clear>(_onClear);
    on<_OddsUpdated>(_onOddsUpdated);
  }

  void _onAdd(_AddSelection event, Emitter<BetSlipState> emit) {
    // Убрать другой исход из того же маркета
    final filtered = state.selections
        .where((s) => s.marketId != event.selection.marketId)
        .toList();
    emit(state.copyWith(
      selections: [...filtered, event.selection],
      error: null,
    ));
  }

  void _onRemove(_RemoveSelection event, Emitter<BetSlipState> emit) {
    emit(state.copyWith(
      selections: state.selections
          .where((s) => s.outcomeId != event.outcomeId)
          .toList(),
    ));
  }

  void _onToggle(_ToggleSelection event, Emitter<BetSlipState> emit) {
    final exists = state.selections.any(
      (s) => s.outcomeId == event.selection.outcomeId,
    );
    if (exists) {
      add(BetSlipEvent.removeSelection(event.selection.outcomeId));
    } else {
      add(BetSlipEvent.addSelection(event.selection));
    }
  }

  void _onSetStake(_SetStake event, Emitter<BetSlipState> emit) {
    emit(state.copyWith(stake: event.stake.clamp(0, 100000)));
  }

  void _onSetBetType(_SetBetType event, Emitter<BetSlipState> emit) {
    emit(state.copyWith(betType: event.type));
  }

  Future<void> _onPlaceBet(_PlaceBet event, Emitter<BetSlipState> emit) async {
    if (!state.canPlace) return;

    emit(state.copyWith(isPlacing: true, error: null));

    final result = await _placeBet(PlaceBetParams(
      selections: state.selections,
      stake: state.stake,
      currency: 'USD', // из user preferences
      betType: state.betType,
      oddsAcceptance: OddsAcceptance.any,
      idempotencyKey: const Uuid().v4(),
    ));

    result.fold(
      (failure) => emit(state.copyWith(
        isPlacing: false,
        error: failure.message,
      )),
      (bet) => emit(BetSlipState(placedBet: bet)),
    );
  }

  void _onClear(_Clear event, Emitter<BetSlipState> emit) {
    emit(const BetSlipState());
  }

  void _onOddsUpdated(_OddsUpdated event, Emitter<BetSlipState> emit) {
    final updated = state.selections.map((s) {
      if (s.outcomeId == event.outcomeId) {
        return s.copyWith(
          previousOdds: s.odds,
          odds: event.newOdds,
        );
      }
      return s;
    }).toList();
    emit(state.copyWith(selections: updated));
  }
}
DEPENDENCY INJECTION
dart

// core/di/injection.dart
@InjectableInit()
Future<void> configureDependencies() async => getIt.init();

// core/di/register_module.dart
@module
abstract class RegisterModule {
  @singleton
  Dio get dio => Dio(BaseOptions(
    baseUrl: AppConfig.apiBaseUrl,
    connectTimeout: const Duration(seconds: 10),
  ));

  @singleton
  HiveInterface get hive => Hive;
}

// Каждый injectable класс помечается аннотацией:
@injectable          // новый инстанс каждый раз
@singleton           // один на всё приложение
@lazySingleton       // создаётся при первом вызове
@Injectable(as: BettingRepository)  // привязка к абстракции
WEBSOCKET INTEGRATION С BLoC
dart

// core/network/ws_client.dart
@singleton
class WsClient {
  WebSocketChannel? _channel;
  final _controller = StreamController<WsMessage>.broadcast();

  Stream<WsMessage> get messages => _controller.stream;

  void connect(String token) {
    _channel = WebSocketChannel.connect(
      Uri.parse('${AppConfig.wsUrl}?token=$token'),
    );

    _channel!.stream.listen(
      (data) {
        final msg = WsMessage.fromJson(jsonDecode(data));
        _controller.add(msg);
      },
      onDone: () => _reconnect(token),
      onError: (_) => _reconnect(token),
    );
  }

  void subscribe(String channel) {
    _channel?.sink.add(jsonEncode({
      'action': 'subscribe',
      'channel': channel,
    }));
  }

  void _reconnect(String token) {
    Future.delayed(const Duration(seconds: 3), () => connect(token));
  }

  void dispose() {
    _channel?.sink.close();
    _controller.close();
  }
}

// BLoC подписка на WebSocket
@injectable
class LiveOddsBloc extends Bloc<LiveOddsEvent, LiveOddsState> {
  final WsClient _ws;
  StreamSubscription? _subscription;

  LiveOddsBloc(this._ws) : super(const LiveOddsState()) {
    on<SubscribeToEvent>(_onSubscribe);
    on<OddsReceived>(_onOddsReceived);
    on<Unsubscribe>(_onUnsubscribe);
  }

  void _onSubscribe(SubscribeToEvent event, Emitter emit) {
    _ws.subscribe('event:${event.eventId}:odds');
    _subscription = _ws.messages
        .where((msg) => msg.channel == 'event:${event.eventId}:odds')
        .listen((msg) => add(OddsReceived(msg.data)));
  }

  void _onOddsReceived(OddsReceived event, Emitter emit) {
    emit(state.copyWith(odds: event.odds));
  }

  @override
  Future<void> close() {
    _subscription?.cancel();
    return super.close();
  }
}
АНТИПАТТЕРНЫ
dart

// ❌ ПЛОХО: StatefulWidget для сложного стейта
class BetSlipPage extends StatefulWidget { ... }
class _BetSlipPageState extends State<BetSlipPage> {
  List<Selection> selections = [];
  double stake = 0;
  bool loading = false;
  void _placeBet() { setState(() { loading = true; }); ... }
}

// ✅ ПРАВИЛЬНО: BLoC для сложного стейта

// ❌ ПЛОХО: BLoC знает о UI (BuildContext, Navigator)
class AuthBloc extends Bloc<AuthEvent, AuthState> {
  void _onLogin(event, emit) {
    Navigator.of(context).push(...);  // ❌ нет context в BLoC
  }
}

// ✅ ПРАВИЛЬНО: BLoC меняет стейт, UI реагирует
// В Page:
BlocListener<AuthBloc, AuthState>(
  listener: (context, state) {
    if (state is Authenticated) context.go('/sports');
  },
)

// ❌ ПЛОХО: Feature зависит от другой Feature напрямую
import 'package:app/features/wallet/data/datasources/wallet_remote_ds.dart';
// В betting feature

// ✅ ПРАВИЛЬНО: через DI и абстракции, или shared domain

// ❌ ПЛОХО: God BLoC (один BLoC на всё приложение)
class AppBloc extends Bloc<AppEvent, AppState> {
  // auth + betting + wallet + casino + ... ❌
}

// ✅ ПРАВИЛЬНО: маленькие специализированные BLoC-и
// AuthBloc, BetSlipBloc, WalletBloc, LiveOddsBloc
PERFORMANCE
text

1. const конструкторы везде где возможно
2. ListView.builder для списков (НЕ ListView с children)
3. RepaintBoundary для тяжёлых виджетов (анимации, графики)
4. Кэширование изображений (CachedNetworkImage)
5. Lazy loading для экранов: GoRoute с builder
6. Избегай rebuild целого дерева — BlocSelector / BlocBuilder
7. Image: resize на сервере, не грузи 4K на мобилу
8. Dispose: отменяй StreamSubscription, Timer, AnimationController