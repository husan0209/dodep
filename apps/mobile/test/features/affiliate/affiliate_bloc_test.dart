import 'package:bloc_test/bloc_test.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:dod_mobile/core/error/failures.dart';
import 'package:dod_mobile/features/affiliate/domain/entities/affiliate_dashboard.dart';
import 'package:dod_mobile/features/affiliate/domain/repositories/affiliate_repository.dart';
import 'package:dod_mobile/features/affiliate/presentation/bloc/affiliate_bloc.dart';

class _MockAffiliateRepository extends Mock implements AffiliateRepository {}

const _dashboard = AffiliateDashboard(
  earningsToday: '10.50',
  earningsThisMonth: '120.00',
  pendingAmount: '80.00',
  availableAmount: '40.00',
  paidAmount: '200.00',
  currency: 'USD',
  clicks: 100,
  registrations: 10,
  ftdCount: 4,
  activePlayers: 6,
  ggrAmount: '1000.00',
  ngrAmount: '600.00',
  commissionAmount: '120.00',
);

const _links = [
  AffiliateLink(
    id: 'link-1',
    campaignName: 'blog',
    referralCode: 'ABC123',
    referralUrl: 'https://example.com/r/ABC123',
  ),
];

const _payouts = [
  AffiliatePayout(
    id: 'payout-1',
    amount: '40.00',
    currency: 'USD',
    status: 'paid',
  ),
];

void main() {
  late _MockAffiliateRepository repository;

  setUp(() {
    repository = _MockAffiliateRepository();
  });

  group('AffiliateBloc', () {
    blocTest<AffiliateBloc, AffiliateState>(
      'emits [loading, loaded] when all sources succeed',
      build: () {
        when(() => repository.getDashboard())
            .thenAnswer((_) async => const Right(_dashboard));
        when(() => repository.getLinks())
            .thenAnswer((_) async => const Right(_links));
        when(() => repository.getPayouts())
            .thenAnswer((_) async => const Right(_payouts));
        return AffiliateBloc(repository: repository);
      },
      act: (bloc) => bloc.add(const AffiliateLoadRequested()),
      expect: () => const [
        AffiliateLoading(),
        AffiliateLoaded(
          dashboard: _dashboard,
          links: _links,
          payouts: _payouts,
        ),
      ],
      verify: (_) {
        verify(() => repository.getDashboard()).called(1);
        verify(() => repository.getLinks()).called(1);
        verify(() => repository.getPayouts()).called(1);
      },
    );

    blocTest<AffiliateBloc, AffiliateState>(
      'emits [loading, error] when dashboard fails',
      build: () {
        when(() => repository.getDashboard()).thenAnswer(
          (_) async => const Left(ServerFailure('boom', code: '500')),
        );
        return AffiliateBloc(repository: repository);
      },
      act: (bloc) => bloc.add(const AffiliateLoadRequested()),
      expect: () => const [
        AffiliateLoading(),
        AffiliateError('boom'),
      ],
      verify: (_) {
        verify(() => repository.getDashboard()).called(1);
        verifyNever(() => repository.getLinks());
        verifyNever(() => repository.getPayouts());
      },
    );

    blocTest<AffiliateBloc, AffiliateState>(
      'degrades to empty lists when links and payouts fail',
      build: () {
        when(() => repository.getDashboard())
            .thenAnswer((_) async => const Right(_dashboard));
        when(() => repository.getLinks()).thenAnswer(
          (_) async => const Left(NetworkFailure()),
        );
        when(() => repository.getPayouts()).thenAnswer(
          (_) async => const Left(NetworkFailure()),
        );
        return AffiliateBloc(repository: repository);
      },
      act: (bloc) => bloc.add(const AffiliateLoadRequested()),
      expect: () => const [
        AffiliateLoading(),
        AffiliateLoaded(
          dashboard: _dashboard,
          links: <AffiliateLink>[],
          payouts: <AffiliatePayout>[],
        ),
      ],
    );
  });
}
