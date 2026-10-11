import 'package:dartz/dartz.dart';
import 'package:injectable/injectable.dart';

import '../../../../core/error/failures.dart';
import '../repositories/auth_repository.dart';

/// Logout use case
@injectable
class Logout {
  final AuthRepository _repository;

  const Logout(this._repository);

  Future<Either<Failure, void>> call() async {
    return await _repository.logout();
  }
}
