import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../../../core/error/failures.dart';
import '../entities/user.dart';
import '../repositories/auth_repository.dart';

/// Register use case params
class RegisterParams extends Equatable {
  final String email;
  final String password;
  final String username;

  const RegisterParams({
    required this.email,
    required this.password,
    required this.username,
  });

  @override
  List<Object> get props => [email, password, username];
}

/// Register use case
class Register {
  final AuthRepository _repository;

  const Register(this._repository);

  Future<Either<Failure, User>> call(RegisterParams params) async {
    // Validate email
    if (!params.email.contains('@')) {
      return const Left(ValidationFailure('Некорректный email', code: 'INVALID_EMAIL'));
    }

    // Validate password
    if (params.password.length < 8) {
      return const Left(ValidationFailure('Пароль должен быть не менее 8 символов', code: 'PASSWORD_TOO_SHORT'));
    }

    // Validate username
    if (params.username.length < 3) {
      return const Left(ValidationFailure('Имя пользователя должно быть не менее 3 символов', code: 'USERNAME_TOO_SHORT'));
    }

    return await _repository.register(
      email: params.email,
      password: params.password,
      username: params.username,
    );
  }
}
