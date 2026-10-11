import 'package:equatable/equatable.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:injectable/injectable.dart';

import '../../domain/entities/affiliate_dashboard.dart';
import '../../domain/repositories/affiliate_repository.dart';

/// BLoC events for the affiliate cabinet.
abstract class AffiliateEvent extends Equatable {
  const AffiliateEvent();

  @override
  List<Object?> get props => [];
}

class AffiliateLoadRequested extends AffiliateEvent {
  const AffiliateLoadRequested();
}

/// BLoC states for the affiliate cabinet.
abstract class AffiliateState extends Equatable {
  const AffiliateState();

  @override
  List<Object?> get props => [];
}

class AffiliateInitial extends AffiliateState {
  const AffiliateInitial();
}

class AffiliateLoading extends AffiliateState {
  const AffiliateLoading();
}

class AffiliateLoaded extends AffiliateState {
  final AffiliateDashboard dashboard;
  final List<AffiliateLink> links;
  final List<AffiliatePayout> payouts;

  const AffiliateLoaded({
    required this.dashboard,
    required this.links,
    required this.payouts,
  });

  @override
  List<Object?> get props => [dashboard, links, payouts];
}

class AffiliateError extends AffiliateState {
  final String message;

  const AffiliateError(this.message);

  @override
  List<Object?> get props => [message];
}

/// Loads dashboard + links + payouts in one refresh.
/// Partial failures degrade to empty lists so the cabinet stays usable.
@injectable
class AffiliateBloc extends Bloc<AffiliateEvent, AffiliateState> {
  final AffiliateRepository _repository;

  AffiliateBloc({required AffiliateRepository repository})
      : _repository = repository,
        super(const AffiliateInitial()) {
    on<AffiliateLoadRequested>(_onLoad);
  }

  Future<void> _onLoad(
    AffiliateLoadRequested event,
    Emitter<AffiliateState> emit,
  ) async {
    emit(const AffiliateLoading());

    final dashboardResult = await _repository.getDashboard();
    if (dashboardResult.isLeft()) {
      final failure = dashboardResult.fold((l) => l, (_) => null);
      emit(AffiliateError(failure?.message ?? 'Failed to load dashboard'));
      return;
    }
    final dashboard = dashboardResult.getOrElse(
      () => throw StateError('unreachable'),
    );

    final links =
        (await _repository.getLinks()).getOrElse(() => <AffiliateLink>[]);
    final payouts =
        (await _repository.getPayouts()).getOrElse(() => <AffiliatePayout>[]);

    emit(AffiliateLoaded(
      dashboard: dashboard,
      links: links,
      payouts: payouts,
    ));
  }
}
