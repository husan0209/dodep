import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../../../core/error/failures.dart';
import '../entities/user.dart';
import '../repositories/auth_repository.dart';

/// Login use case params
class LoginParams extends Equatable {
  final String email;
  final String password;

  const LoginParams({
    required this.email,
    required this.password,
  });

  @override
  List<Object> get props => [email, password];
}

/// Login use case
class Login {
  final AuthRepository _repository;

  const Login(this._repository);

  Future<Either<Failure, User>> call(LoginParams params) async {
    // Validate email
    if (!params.email.contains('@')) {
      return const Left(ValidationFailure('Некорректный email', code: 'INVALID_EMAIL'));
    }

    // Validate password
    if (params.password.length < 6) {
      return const Left(ValidationFailure('Пароль должен быть не менее 6 символов', code: 'PASSWORD_TOO_SHORT'));
    }

    return await _repository.login(
      email: params.email,
      password: params.password,
    );
  }
}
