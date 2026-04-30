import 'package:json_annotation/json_annotation.dart';

part 'auth_tokens.g.dart';

/// Authentication tokens model
@JsonSerializable()
class AuthTokens {
  @JsonKey(name: 'access_token')
  final String accessToken;

  @JsonKey(name: 'refresh_token')
  final String refreshToken;

  @JsonKey(name: 'expires_in', defaultValue: 900)
  final int expiresIn;

  @JsonKey(name: 'token_type', defaultValue: 'Bearer')
  final String tokenType;

  const AuthTokens({
    required this.accessToken,
    required this.refreshToken,
    this.expiresIn = 900,
    this.tokenType = 'Bearer',
  });

  factory AuthTokens.fromJson(Map<String, dynamic> json) => _$AuthTokensFromJson(json);

  Map<String, dynamic> toJson() => _$AuthTokensToJson(this);
}
