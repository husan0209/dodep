import 'package:equatable/equatable.dart';

/// Base failure class for domain layer errors
abstract class Failure extends Equatable {
  final String message;
  final String? code;

  const Failure(this.message, {this.code});

  @override
  List<Object> get props => [message, code ?? ''];
}

/// Server-side errors (API errors)
class ServerFailure extends Failure {
  const ServerFailure(super.message, {super.code});

  factory ServerFailure.fromJson(Map<String, dynamic> json) {
    return ServerFailure(
      json['message'] as String? ?? 'Server error',
      code: json['code'] as String?,
    );
  }
}

/// Network connectivity errors
class NetworkFailure extends Failure {
  const NetworkFailure()
      : super('Нет подключения к интернету', code: 'NETWORK_ERROR');
}

/// Authentication errors
class AuthFailure extends Failure {
  const AuthFailure(super.message, {super.code});

  factory AuthFailure.fromCode(String code) {
    switch (code) {
      case 'AUTH_INVALID_CREDENTIALS':
        return AuthFailure('Неверный email или пароль', code: code);
      case 'AUTH_TOKEN_EXPIRED':
        return AuthFailure('Сессия истекла', code: code);
      case 'AUTH_ACCOUNT_LOCKED':
        return AuthFailure('Аккаунт заблокирован', code: code);
      case 'AUTH_ACCOUNT_SUSPENDED':
        return AuthFailure('Аккаунт приостановлен', code: code);
      default:
        return AuthFailure(code, code: code);
    }
  }
}

/// Validation errors
class ValidationFailure extends Failure {
  const ValidationFailure(super.message, {super.code});
}

/// Insufficient balance errors
class InsufficientBalanceFailure extends Failure {
  const InsufficientBalanceFailure({String? code})
      : super('Недостаточно средств',
            code: code ?? 'WALLET_INSUFFICIENT_BALANCE');
}

/// Bet placement errors
class BetFailure extends Failure {
  const BetFailure(super.message, {super.code});

  factory BetFailure.fromCode(String code) {
    switch (code) {
      case 'BET_EVENT_SUSPENDED':
        return BetFailure('Событие приостановлено', code: code);
      case 'BET_MARKET_CLOSED':
        return BetFailure('Рынок закрыт', code: code);
      case 'BET_ODDS_CHANGED':
        return BetFailure('Коэффициенты изменились', code: code);
      case 'BET_STAKE_TOO_LOW':
        return BetFailure('Ставка слишком маленькая', code: code);
      case 'BET_STAKE_TOO_HIGH':
        return BetFailure('Ставка слишком большая', code: code);
      default:
        return BetFailure(code, code: code);
    }
  }
}

/// Not found errors
class NotFoundFailure extends Failure {
  const NotFoundFailure(super.message, {super.code});
}

/// Unknown/unexpected errors
class UnknownFailure extends Failure {
  const UnknownFailure([String? message])
      : super(message ?? 'Произошла неизвестная ошибка', code: 'UNKNOWN');
}
