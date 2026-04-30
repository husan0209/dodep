/// Base exception class for data layer
abstract class AppException implements Exception {
  final String message;
  final String? code;

  const AppException(this.message, {this.code});

  @override
  String toString() => '$runtimeType: $message${code != null ? ' (code: $code)' : ''}';
}

/// Server-side exceptions (HTTP errors)
class ServerException extends AppException {
  final int statusCode;

  const ServerException(super.message, {this.statusCode = 500, super.code});

  factory ServerException.fromResponse(int statusCode, Map<String, dynamic> body) {
    return ServerException(
      body['error']?['message'] as String? ?? 'Server error',
      statusCode: statusCode,
      code: body['error']?['code'] as String?,
    );
  }
}

/// Network connectivity exceptions
class NetworkException extends AppException {
  const NetworkException() : super('Нет подключения к интернету', code: 'NETWORK_ERROR');
}

/// Authentication exceptions
class AuthException extends AppException {
  const AuthException(super.message, {super.code});

  factory AuthException.fromCode(String code) {
    switch (code) {
      case 'AUTH_INVALID_CREDENTIALS':
        return const AuthException('Неверный email или пароль', code: code);
      case 'AUTH_TOKEN_EXPIRED':
        return const AuthException('Токен истёк', code: code);
      case 'AUTH_ACCOUNT_LOCKED':
        return const AuthException('Аккаунт заблокирован', code: code);
      default:
        return AuthException(code, code: code);
    }
  }
}

/// Validation exceptions
class ValidationException extends AppException {
  final Map<String, String>? fieldErrors;

  const ValidationException(
    super.message, {
    this.fieldErrors,
    super.code,
  });
}

/// Not found exceptions
class NotFoundException extends AppException {
  const NotFoundException(super.message, {super.code});
}

/// Timeout exceptions
class TimeoutException extends AppException {
  const TimeoutException() : super('Время ожидания истекло', code: 'TIMEOUT');
}

/// Cache exceptions
class CacheException extends AppException {
  const CacheException(super.message, {super.code});
}

/// WebSocket exceptions
class WebSocketException extends AppException {
  const WebSocketException(super.message, {super.code});
}
