import 'package:equatable/equatable.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:injectable/injectable.dart';

import '../../domain/entities/rg_limits.dart';
import '../../domain/repositories/rg_repository.dart';

/// BLoC events for the RG cabinet.
abstract class RGEvent extends Equatable {
  const RGEvent();

  @override
  List<Object?> get props => [];
}

class RGLoadRequested extends RGEvent {
  const RGLoadRequested();
}

class RGSaveLimitsRequested extends RGEvent {
  final Map<String, String?> money;
  final int? sessionMinutes;
  final int? realityCheckMinutes;

  const RGSaveLimitsRequested({
    required this.money,
    this.sessionMinutes,
    this.realityCheckMinutes,
  });

  @override
  List<Object?> get props => [money, sessionMinutes, realityCheckMinutes];
}

class RGSelfExcludeRequested extends RGEvent {
  final String period;

  const RGSelfExcludeRequested(this.period);

  @override
  List<Object?> get props => [period];
}

class RGRevokeSelfExclusionRequested extends RGEvent {
  const RGRevokeSelfExclusionRequested();
}

class RGStartTimeoutRequested extends RGEvent {
  final String period;

  const RGStartTimeoutRequested(this.period);

  @override
  List<Object?> get props => [period];
}

/// BLoC states for the RG cabinet.
abstract class RGState extends Equatable {
  const RGState();

  @override
  List<Object?> get props => [];
}

class RGInitial extends RGState {
  const RGInitial();
}

class RGLoading extends RGState {
  const RGLoading();
}

class RGLoaded extends RGState {
  final RGStatus status;
  final bool saving;
  final String? notice;

  const RGLoaded({
    required this.status,
    this.saving = false,
    this.notice,
  });

  RGLoaded copyWith({RGStatus? status, bool? saving, String? notice}) {
    return RGLoaded(
      status: status ?? this.status,
      saving: saving ?? this.saving,
      notice: notice,
    );
  }

  @override
  List<Object?> get props => [status, saving, notice];
}

class RGError extends RGState {
  final String message;
  final RGStatus? previous;

  const RGError(this.message, {this.previous});

  @override
  List<Object?> get props => [message, previous];
}

/// The RG cabinet: status, limits with cooling semantics, self-exclusion
/// and time-outs. Reads come from the RG service (JWT identity), so no
/// user id is threaded through the UI.
@injectable
class RGBloc extends Bloc<RGEvent, RGState> {
  final RGRepository _repository;

  RGBloc({required RGRepository repository})
      : _repository = repository,
        super(const RGInitial()) {
    on<RGLoadRequested>(_onLoad);
    on<RGSaveLimitsRequested>(_onSaveLimits);
    on<RGSelfExcludeRequested>(_onSelfExclude);
    on<RGRevokeSelfExclusionRequested>(_onRevoke);
    on<RGStartTimeoutRequested>(_onTimeout);
  }

  Future<void> _onLoad(RGLoadRequested event, Emitter<RGState> emit) async {
    emit(const RGLoading());
    final result = await _repository.loadStatus();
    result.fold(
      (failure) => emit(RGError(failure.message)),
      (status) => emit(RGLoaded(status: status)),
    );
  }

  Future<void> _onSaveLimits(
    RGSaveLimitsRequested event,
    Emitter<RGState> emit,
  ) async {
    final current = state;
    if (current is! RGLoaded) return;
    emit(current.copyWith(saving: true, notice: null));

    final result = await _repository.setLimits(
      event.money,
      sessionMinutes: event.sessionMinutes,
      realityCheckMinutes: event.realityCheckMinutes,
    );

    if (result.isLeft()) {
      final failure = result.fold((l) => l, (_) => null);
      emit(RGError(failure?.message ?? 'Не удалось сохранить лимиты',
          previous: current.status));
      return;
    }

    final limits = result.getOrElse(() => current.status.limits);
    // Re-read status so the UI reflects exclusion/timeout state too.
    final refreshed = (await _repository.loadStatus())
        .getOrElse(() => current.status.copyWith(limits: limits));
    emit(current.copyWith(
      status: refreshed,
      saving: false,
      notice: _saveNotice(refreshed.limits, limits.pending.length),
    ));
  }

  Future<void> _onSelfExclude(
    RGSelfExcludeRequested event,
    Emitter<RGState> emit,
  ) async {
    final current = state;
    if (current is! RGLoaded) return;
    emit(current.copyWith(saving: true, notice: null));

    final result = await _repository.selfExclude(event.period);
    if (result.isLeft()) {
      final failure = result.fold((l) => l, (_) => null);
      emit(RGError(failure?.message ?? 'Не удалось включить самоисключение',
          previous: current.status));
      return;
    }

    final refreshed =
        (await _repository.loadStatus()).getOrElse(() => current.status);
    emit(current.copyWith(
      status: refreshed,
      saving: false,
      notice: 'Самоисключение активно. Вывод средств остаётся доступен.',
    ));
  }

  Future<void> _onRevoke(
    RGRevokeSelfExclusionRequested event,
    Emitter<RGState> emit,
  ) async {
    final current = state;
    if (current is! RGLoaded) return;
    emit(current.copyWith(saving: true, notice: null));

    final result = await _repository.revokeSelfExclusion();
    if (result.isLeft()) {
      final failure = result.fold((l) => l, (_) => null);
      emit(RGError(failure?.message ?? 'Не удалось снять исключение',
          previous: current.status));
      return;
    }
    final refreshed =
        (await _repository.loadStatus()).getOrElse(() => current.status);
    emit(current.copyWith(
      status: refreshed,
      saving: false,
      notice: 'Исключение снято.',
    ));
  }

  Future<void> _onTimeout(
    RGStartTimeoutRequested event,
    Emitter<RGState> emit,
  ) async {
    final current = state;
    if (current is! RGLoaded) return;
    emit(current.copyWith(saving: true, notice: null));

    final result = await _repository.startTimeout(event.period);
    if (result.isLeft()) {
      final failure = result.fold((l) => l, (_) => null);
      emit(RGError(failure?.message ?? 'Не удалось включить паузу',
          previous: current.status));
      return;
    }
    final refreshed =
        (await _repository.loadStatus()).getOrElse(() => current.status);
    emit(current.copyWith(
      status: refreshed,
      saving: false,
      notice: 'Пауза включена. Игра приостановлена, вывод доступен.',
    ));
  }

  /// Explains cooling semantics: decreases apply immediately, increases wait
  /// for `effective_at`.
  String _saveNotice(RGLimits limits, int pendingCount) {
    if (pendingCount == 0) {
      return 'Лимиты обновлены и действуют сразу.';
    }
    final cooling = limits.coolingChanges(DateTime.now()).length;
    if (cooling == 0) {
      return 'Лимит повышен — ожидает применения по расписанию.';
    }
    return 'Понижение действует сразу, повышение — после cooling-периода.';
  }
}
