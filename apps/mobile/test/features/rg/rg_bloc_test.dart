import 'package:bloc_test/bloc_test.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:dod_mobile/core/error/failures.dart';
import 'package:dod_mobile/features/responsible_gambling/domain/entities/rg_limits.dart';
import 'package:dod_mobile/features/responsible_gambling/domain/repositories/rg_repository.dart';
import 'package:dod_mobile/features/responsible_gambling/presentation/bloc/rg_bloc.dart';

class _MockRGRepository extends Mock implements RGRepository {}

final _status = RGStatus(
  gamblingAllowed: true,
  blockedReason: '',
  limits: RGLimits(depositDaily: MoneyCap('100.00')),
);

void main() {
  late _MockRGRepository repository;

  setUp(() {
    repository = _MockRGRepository();
  });

  group('RGBloc', () {
    blocTest<RGBloc, RGState>(
      'emits [loading, loaded] on successful status load',
      build: () {
        when(() => repository.loadStatus()).thenAnswer((_) async => Right(_status));
        return RGBloc(repository: repository);
      },
      act: (bloc) => bloc.add(const RGLoadRequested()),
      expect: () => const [RGLoading(), RGLoaded(status: _status)],
    );

    blocTest<RGBloc, RGState>(
      'emits [loading, error] when status load fails',
      build: () {
        when(() => repository.loadStatus()).thenAnswer(
          (_) async => const Left(NetworkFailure()),
        );
        return RGBloc(repository: repository);
      },
      act: (bloc) => bloc.add(const RGLoadRequested()),
      expect: () => const [
        RGLoading(),
        RGError('Нет подключения к интернету'),
      ],
    );

    blocTest<RGBloc, RGState>(
      'saving keeps the screen visible, then reloads status',
      build: () {
        when(() => repository.setLimits(any(),
                sessionMinutes: any(named: 'sessionMinutes'),
                realityCheckMinutes: any(named: 'realityCheckMinutes')))
            .thenAnswer((_) async => Right(_status.limits));
        when(() => repository.loadStatus()).thenAnswer((_) async => Right(_status));
        return RGBloc(repository: repository);
      },
      seed: () => const RGLoaded(status: _status),
      act: (bloc) => bloc.add(const RGSaveLimitsRequested(
        money: {'deposit_daily': '200.00'},
      )),
      expect: () => [
        const RGLoaded(status: _status, saving: true),
        const RGLoaded(
          status: _status,
          notice: 'Лимиты обновлены и действуют сразу.',
        ),
      ],
      verify: (_) {
        verify(() => repository.loadStatus()).called(1);
      },
    );

    blocTest<RGBloc, RGState>(
      'failed save preserves previous screen in the error state',
      build: () {
        when(() => repository.setLimits(any(),
                sessionMinutes: any(named: 'sessionMinutes'),
                realityCheckMinutes: any(named: 'realityCheckMinutes')))
            .thenAnswer((_) async => const Left(ServerFailure('rg down')));
        return RGBloc(repository: repository);
      },
      seed: () => const RGLoaded(status: _status),
      act: (bloc) => bloc.add(const RGSaveLimitsRequested(
        money: {'deposit_daily': '50.00'},
      )),
      expect: () => const [
        RGLoaded(status: _status, saving: true),
        RGError('rg down', previous: _status),
      ],
    );

    blocTest<RGBloc, RGState>(
      'self-exclusion refreshes status and warns withdrawals stay open',
      build: () {
        when(() => repository.selfExclude('6m')).thenAnswer(
          (_) async => const RGExclusion(
            id: 'e1',
            type: 'self',
            status: 'active',
            until: null,
            permanent: false,
          ),
        );
        when(() => repository.loadStatus()).thenAnswer((_) async => Right(_status));
        return RGBloc(repository: repository);
      },
      seed: () => const RGLoaded(status: _status),
      act: (bloc) => bloc.add(const RGSelfExcludeRequested('6m')),
      expect: () => [
        const RGLoaded(status: _status, saving: true),
        const RGLoaded(
          status: _status,
          notice: 'Самоисключение активно. Вывод средств остаётся доступен.',
        ),
      ],
    );

    blocTest<RGBloc, RGState>(
      'timeout message states play paused, withdrawal available',
      build: () {
        when(() => repository.startTimeout('24h')).thenAnswer((_) async => const Right(unit));
        when(() => repository.loadStatus()).thenAnswer((_) async => Right(_status));
        return RGBloc(repository: repository);
      },
      seed: () => const RGLoaded(status: _status),
      act: (bloc) => bloc.add(const RGStartTimeoutRequested('24h')),
      expect: () => [
        const RGLoaded(status: _status, saving: true),
        const RGLoaded(
          status: _status,
          notice: 'Пауза включена. Игра приостановлена, вывод доступен.',
        ),
      ],
    );
  });
}