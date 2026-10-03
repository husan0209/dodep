// coverage:ignore-file
// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'auth_event.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

T _$identity<T>(T value) => value;

final _privateConstructorUsedError = UnsupportedError(
    'It seems like you constructed your class using `MyClass._()`. This constructor is only meant to be used by freezed and you are not supposed to need it nor use it.\nPlease check the documentation here for more information: https://github.com/rrousselGit/freezed#adding-getters-and-methods-to-our-models');

/// @nodoc
mixin _$AuthEvent {
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String email, String password) login,
    required TResult Function(String email, String password, String username)
        register,
    required TResult Function() logout,
    required TResult Function() getCurrentUser,
    required TResult Function() refreshToken,
  }) =>
      throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String email, String password)? login,
    TResult? Function(String email, String password, String username)? register,
    TResult? Function()? logout,
    TResult? Function()? getCurrentUser,
    TResult? Function()? refreshToken,
  }) =>
      throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String email, String password)? login,
    TResult Function(String email, String password, String username)? register,
    TResult Function()? logout,
    TResult Function()? getCurrentUser,
    TResult Function()? refreshToken,
    required TResult orElse(),
  }) =>
      throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(AuthLogin value) login,
    required TResult Function(AuthRegister value) register,
    required TResult Function(AuthLogout value) logout,
    required TResult Function(AuthGetCurrentUser value) getCurrentUser,
    required TResult Function(AuthRefreshToken value) refreshToken,
  }) =>
      throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(AuthLogin value)? login,
    TResult? Function(AuthRegister value)? register,
    TResult? Function(AuthLogout value)? logout,
    TResult? Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult? Function(AuthRefreshToken value)? refreshToken,
  }) =>
      throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(AuthLogin value)? login,
    TResult Function(AuthRegister value)? register,
    TResult Function(AuthLogout value)? logout,
    TResult Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult Function(AuthRefreshToken value)? refreshToken,
    required TResult orElse(),
  }) =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $AuthEventCopyWith<$Res> {
  factory $AuthEventCopyWith(AuthEvent value, $Res Function(AuthEvent) then) =
      _$AuthEventCopyWithImpl<$Res, AuthEvent>;
}

/// @nodoc
class _$AuthEventCopyWithImpl<$Res, $Val extends AuthEvent>
    implements $AuthEventCopyWith<$Res> {
  _$AuthEventCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;
}

/// @nodoc
abstract class _$$AuthLoginImplCopyWith<$Res> {
  factory _$$AuthLoginImplCopyWith(
          _$AuthLoginImpl value, $Res Function(_$AuthLoginImpl) then) =
      __$$AuthLoginImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String email, String password});
}

/// @nodoc
class __$$AuthLoginImplCopyWithImpl<$Res>
    extends _$AuthEventCopyWithImpl<$Res, _$AuthLoginImpl>
    implements _$$AuthLoginImplCopyWith<$Res> {
  __$$AuthLoginImplCopyWithImpl(
      _$AuthLoginImpl _value, $Res Function(_$AuthLoginImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? email = null,
    Object? password = null,
  }) {
    return _then(_$AuthLoginImpl(
      email: null == email
          ? _value.email
          : email // ignore: cast_nullable_to_non_nullable
              as String,
      password: null == password
          ? _value.password
          : password // ignore: cast_nullable_to_non_nullable
              as String,
    ));
  }
}

/// @nodoc

class _$AuthLoginImpl implements AuthLogin {
  const _$AuthLoginImpl({required this.email, required this.password});

  @override
  final String email;
  @override
  final String password;

  @override
  String toString() {
    return 'AuthEvent.login(email: $email, password: $password)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$AuthLoginImpl &&
            (identical(other.email, email) || other.email == email) &&
            (identical(other.password, password) ||
                other.password == password));
  }

  @override
  int get hashCode => Object.hash(runtimeType, email, password);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$AuthLoginImplCopyWith<_$AuthLoginImpl> get copyWith =>
      __$$AuthLoginImplCopyWithImpl<_$AuthLoginImpl>(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String email, String password) login,
    required TResult Function(String email, String password, String username)
        register,
    required TResult Function() logout,
    required TResult Function() getCurrentUser,
    required TResult Function() refreshToken,
  }) {
    return login(email, password);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String email, String password)? login,
    TResult? Function(String email, String password, String username)? register,
    TResult? Function()? logout,
    TResult? Function()? getCurrentUser,
    TResult? Function()? refreshToken,
  }) {
    return login?.call(email, password);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String email, String password)? login,
    TResult Function(String email, String password, String username)? register,
    TResult Function()? logout,
    TResult Function()? getCurrentUser,
    TResult Function()? refreshToken,
    required TResult orElse(),
  }) {
    if (login != null) {
      return login(email, password);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(AuthLogin value) login,
    required TResult Function(AuthRegister value) register,
    required TResult Function(AuthLogout value) logout,
    required TResult Function(AuthGetCurrentUser value) getCurrentUser,
    required TResult Function(AuthRefreshToken value) refreshToken,
  }) {
    return login(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(AuthLogin value)? login,
    TResult? Function(AuthRegister value)? register,
    TResult? Function(AuthLogout value)? logout,
    TResult? Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult? Function(AuthRefreshToken value)? refreshToken,
  }) {
    return login?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(AuthLogin value)? login,
    TResult Function(AuthRegister value)? register,
    TResult Function(AuthLogout value)? logout,
    TResult Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult Function(AuthRefreshToken value)? refreshToken,
    required TResult orElse(),
  }) {
    if (login != null) {
      return login(this);
    }
    return orElse();
  }
}

abstract class AuthLogin implements AuthEvent {
  const factory AuthLogin(
      {required final String email,
      required final String password}) = _$AuthLoginImpl;

  String get email;
  String get password;
  @JsonKey(ignore: true)
  _$$AuthLoginImplCopyWith<_$AuthLoginImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class _$$AuthRegisterImplCopyWith<$Res> {
  factory _$$AuthRegisterImplCopyWith(
          _$AuthRegisterImpl value, $Res Function(_$AuthRegisterImpl) then) =
      __$$AuthRegisterImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String email, String password, String username});
}

/// @nodoc
class __$$AuthRegisterImplCopyWithImpl<$Res>
    extends _$AuthEventCopyWithImpl<$Res, _$AuthRegisterImpl>
    implements _$$AuthRegisterImplCopyWith<$Res> {
  __$$AuthRegisterImplCopyWithImpl(
      _$AuthRegisterImpl _value, $Res Function(_$AuthRegisterImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? email = null,
    Object? password = null,
    Object? username = null,
  }) {
    return _then(_$AuthRegisterImpl(
      email: null == email
          ? _value.email
          : email // ignore: cast_nullable_to_non_nullable
              as String,
      password: null == password
          ? _value.password
          : password // ignore: cast_nullable_to_non_nullable
              as String,
      username: null == username
          ? _value.username
          : username // ignore: cast_nullable_to_non_nullable
              as String,
    ));
  }
}

/// @nodoc

class _$AuthRegisterImpl implements AuthRegister {
  const _$AuthRegisterImpl(
      {required this.email, required this.password, required this.username});

  @override
  final String email;
  @override
  final String password;
  @override
  final String username;

  @override
  String toString() {
    return 'AuthEvent.register(email: $email, password: $password, username: $username)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$AuthRegisterImpl &&
            (identical(other.email, email) || other.email == email) &&
            (identical(other.password, password) ||
                other.password == password) &&
            (identical(other.username, username) ||
                other.username == username));
  }

  @override
  int get hashCode => Object.hash(runtimeType, email, password, username);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$AuthRegisterImplCopyWith<_$AuthRegisterImpl> get copyWith =>
      __$$AuthRegisterImplCopyWithImpl<_$AuthRegisterImpl>(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String email, String password) login,
    required TResult Function(String email, String password, String username)
        register,
    required TResult Function() logout,
    required TResult Function() getCurrentUser,
    required TResult Function() refreshToken,
  }) {
    return register(email, password, username);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String email, String password)? login,
    TResult? Function(String email, String password, String username)? register,
    TResult? Function()? logout,
    TResult? Function()? getCurrentUser,
    TResult? Function()? refreshToken,
  }) {
    return register?.call(email, password, username);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String email, String password)? login,
    TResult Function(String email, String password, String username)? register,
    TResult Function()? logout,
    TResult Function()? getCurrentUser,
    TResult Function()? refreshToken,
    required TResult orElse(),
  }) {
    if (register != null) {
      return register(email, password, username);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(AuthLogin value) login,
    required TResult Function(AuthRegister value) register,
    required TResult Function(AuthLogout value) logout,
    required TResult Function(AuthGetCurrentUser value) getCurrentUser,
    required TResult Function(AuthRefreshToken value) refreshToken,
  }) {
    return register(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(AuthLogin value)? login,
    TResult? Function(AuthRegister value)? register,
    TResult? Function(AuthLogout value)? logout,
    TResult? Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult? Function(AuthRefreshToken value)? refreshToken,
  }) {
    return register?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(AuthLogin value)? login,
    TResult Function(AuthRegister value)? register,
    TResult Function(AuthLogout value)? logout,
    TResult Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult Function(AuthRefreshToken value)? refreshToken,
    required TResult orElse(),
  }) {
    if (register != null) {
      return register(this);
    }
    return orElse();
  }
}

abstract class AuthRegister implements AuthEvent {
  const factory AuthRegister(
      {required final String email,
      required final String password,
      required final String username}) = _$AuthRegisterImpl;

  String get email;
  String get password;
  String get username;
  @JsonKey(ignore: true)
  _$$AuthRegisterImplCopyWith<_$AuthRegisterImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class _$$AuthLogoutImplCopyWith<$Res> {
  factory _$$AuthLogoutImplCopyWith(
          _$AuthLogoutImpl value, $Res Function(_$AuthLogoutImpl) then) =
      __$$AuthLogoutImplCopyWithImpl<$Res>;
}

/// @nodoc
class __$$AuthLogoutImplCopyWithImpl<$Res>
    extends _$AuthEventCopyWithImpl<$Res, _$AuthLogoutImpl>
    implements _$$AuthLogoutImplCopyWith<$Res> {
  __$$AuthLogoutImplCopyWithImpl(
      _$AuthLogoutImpl _value, $Res Function(_$AuthLogoutImpl) _then)
      : super(_value, _then);
}

/// @nodoc

class _$AuthLogoutImpl implements AuthLogout {
  const _$AuthLogoutImpl();

  @override
  String toString() {
    return 'AuthEvent.logout()';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType && other is _$AuthLogoutImpl);
  }

  @override
  int get hashCode => runtimeType.hashCode;

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String email, String password) login,
    required TResult Function(String email, String password, String username)
        register,
    required TResult Function() logout,
    required TResult Function() getCurrentUser,
    required TResult Function() refreshToken,
  }) {
    return logout();
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String email, String password)? login,
    TResult? Function(String email, String password, String username)? register,
    TResult? Function()? logout,
    TResult? Function()? getCurrentUser,
    TResult? Function()? refreshToken,
  }) {
    return logout?.call();
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String email, String password)? login,
    TResult Function(String email, String password, String username)? register,
    TResult Function()? logout,
    TResult Function()? getCurrentUser,
    TResult Function()? refreshToken,
    required TResult orElse(),
  }) {
    if (logout != null) {
      return logout();
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(AuthLogin value) login,
    required TResult Function(AuthRegister value) register,
    required TResult Function(AuthLogout value) logout,
    required TResult Function(AuthGetCurrentUser value) getCurrentUser,
    required TResult Function(AuthRefreshToken value) refreshToken,
  }) {
    return logout(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(AuthLogin value)? login,
    TResult? Function(AuthRegister value)? register,
    TResult? Function(AuthLogout value)? logout,
    TResult? Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult? Function(AuthRefreshToken value)? refreshToken,
  }) {
    return logout?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(AuthLogin value)? login,
    TResult Function(AuthRegister value)? register,
    TResult Function(AuthLogout value)? logout,
    TResult Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult Function(AuthRefreshToken value)? refreshToken,
    required TResult orElse(),
  }) {
    if (logout != null) {
      return logout(this);
    }
    return orElse();
  }
}

abstract class AuthLogout implements AuthEvent {
  const factory AuthLogout() = _$AuthLogoutImpl;
}

/// @nodoc
abstract class _$$AuthGetCurrentUserImplCopyWith<$Res> {
  factory _$$AuthGetCurrentUserImplCopyWith(_$AuthGetCurrentUserImpl value,
          $Res Function(_$AuthGetCurrentUserImpl) then) =
      __$$AuthGetCurrentUserImplCopyWithImpl<$Res>;
}

/// @nodoc
class __$$AuthGetCurrentUserImplCopyWithImpl<$Res>
    extends _$AuthEventCopyWithImpl<$Res, _$AuthGetCurrentUserImpl>
    implements _$$AuthGetCurrentUserImplCopyWith<$Res> {
  __$$AuthGetCurrentUserImplCopyWithImpl(_$AuthGetCurrentUserImpl _value,
      $Res Function(_$AuthGetCurrentUserImpl) _then)
      : super(_value, _then);
}

/// @nodoc

class _$AuthGetCurrentUserImpl implements AuthGetCurrentUser {
  const _$AuthGetCurrentUserImpl();

  @override
  String toString() {
    return 'AuthEvent.getCurrentUser()';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType && other is _$AuthGetCurrentUserImpl);
  }

  @override
  int get hashCode => runtimeType.hashCode;

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String email, String password) login,
    required TResult Function(String email, String password, String username)
        register,
    required TResult Function() logout,
    required TResult Function() getCurrentUser,
    required TResult Function() refreshToken,
  }) {
    return getCurrentUser();
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String email, String password)? login,
    TResult? Function(String email, String password, String username)? register,
    TResult? Function()? logout,
    TResult? Function()? getCurrentUser,
    TResult? Function()? refreshToken,
  }) {
    return getCurrentUser?.call();
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String email, String password)? login,
    TResult Function(String email, String password, String username)? register,
    TResult Function()? logout,
    TResult Function()? getCurrentUser,
    TResult Function()? refreshToken,
    required TResult orElse(),
  }) {
    if (getCurrentUser != null) {
      return getCurrentUser();
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(AuthLogin value) login,
    required TResult Function(AuthRegister value) register,
    required TResult Function(AuthLogout value) logout,
    required TResult Function(AuthGetCurrentUser value) getCurrentUser,
    required TResult Function(AuthRefreshToken value) refreshToken,
  }) {
    return getCurrentUser(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(AuthLogin value)? login,
    TResult? Function(AuthRegister value)? register,
    TResult? Function(AuthLogout value)? logout,
    TResult? Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult? Function(AuthRefreshToken value)? refreshToken,
  }) {
    return getCurrentUser?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(AuthLogin value)? login,
    TResult Function(AuthRegister value)? register,
    TResult Function(AuthLogout value)? logout,
    TResult Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult Function(AuthRefreshToken value)? refreshToken,
    required TResult orElse(),
  }) {
    if (getCurrentUser != null) {
      return getCurrentUser(this);
    }
    return orElse();
  }
}

abstract class AuthGetCurrentUser implements AuthEvent {
  const factory AuthGetCurrentUser() = _$AuthGetCurrentUserImpl;
}

/// @nodoc
abstract class _$$AuthRefreshTokenImplCopyWith<$Res> {
  factory _$$AuthRefreshTokenImplCopyWith(_$AuthRefreshTokenImpl value,
          $Res Function(_$AuthRefreshTokenImpl) then) =
      __$$AuthRefreshTokenImplCopyWithImpl<$Res>;
}

/// @nodoc
class __$$AuthRefreshTokenImplCopyWithImpl<$Res>
    extends _$AuthEventCopyWithImpl<$Res, _$AuthRefreshTokenImpl>
    implements _$$AuthRefreshTokenImplCopyWith<$Res> {
  __$$AuthRefreshTokenImplCopyWithImpl(_$AuthRefreshTokenImpl _value,
      $Res Function(_$AuthRefreshTokenImpl) _then)
      : super(_value, _then);
}

/// @nodoc

class _$AuthRefreshTokenImpl implements AuthRefreshToken {
  const _$AuthRefreshTokenImpl();

  @override
  String toString() {
    return 'AuthEvent.refreshToken()';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType && other is _$AuthRefreshTokenImpl);
  }

  @override
  int get hashCode => runtimeType.hashCode;

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String email, String password) login,
    required TResult Function(String email, String password, String username)
        register,
    required TResult Function() logout,
    required TResult Function() getCurrentUser,
    required TResult Function() refreshToken,
  }) {
    return refreshToken();
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String email, String password)? login,
    TResult? Function(String email, String password, String username)? register,
    TResult? Function()? logout,
    TResult? Function()? getCurrentUser,
    TResult? Function()? refreshToken,
  }) {
    return refreshToken?.call();
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String email, String password)? login,
    TResult Function(String email, String password, String username)? register,
    TResult Function()? logout,
    TResult Function()? getCurrentUser,
    TResult Function()? refreshToken,
    required TResult orElse(),
  }) {
    if (refreshToken != null) {
      return refreshToken();
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(AuthLogin value) login,
    required TResult Function(AuthRegister value) register,
    required TResult Function(AuthLogout value) logout,
    required TResult Function(AuthGetCurrentUser value) getCurrentUser,
    required TResult Function(AuthRefreshToken value) refreshToken,
  }) {
    return refreshToken(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(AuthLogin value)? login,
    TResult? Function(AuthRegister value)? register,
    TResult? Function(AuthLogout value)? logout,
    TResult? Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult? Function(AuthRefreshToken value)? refreshToken,
  }) {
    return refreshToken?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(AuthLogin value)? login,
    TResult Function(AuthRegister value)? register,
    TResult Function(AuthLogout value)? logout,
    TResult Function(AuthGetCurrentUser value)? getCurrentUser,
    TResult Function(AuthRefreshToken value)? refreshToken,
    required TResult orElse(),
  }) {
    if (refreshToken != null) {
      return refreshToken(this);
    }
    return orElse();
  }
}

abstract class AuthRefreshToken implements AuthEvent {
  const factory AuthRefreshToken() = _$AuthRefreshTokenImpl;
}
