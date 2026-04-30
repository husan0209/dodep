import 'package:json_annotation/json_annotation.dart';
import '../../domain/entities/user.dart';

part 'user_model.g.dart';

/// User model for JSON serialization
@JsonSerializable()
class UserModel extends User {
  @JsonKey(name: 'id')
  final int _id;

  @JsonKey(name: 'email')
  final String _email;

  @JsonKey(name: 'username')
  final String _username;

  @JsonKey(name: 'phone', includeIfNull: false)
  final String? _phone;

  @JsonKey(name: 'country', includeIfNull: false)
  final String? _country;

  @JsonKey(name: 'currency', includeIfNull: false)
  final String? _currency;

  @JsonKey(name: 'kyc_level', defaultValue: 0)
  final int _kycLevel;

  @JsonKey(name: 'is_active', defaultValue: true)
  final bool _isActive;

  @JsonKey(name: 'created_at', fromJson: _parseDateTime)
  final DateTime _createdAt;

  const UserModel({
    required int id,
    required String email,
    required String username,
    String? phone,
    String? country,
    String? currency,
    int kycLevel = 0,
    bool isActive = true,
    required DateTime createdAt,
  })  : _id = id,
        _email = email,
        _username = username,
        _phone = phone,
        _country = country,
        _currency = currency,
        _kycLevel = kycLevel,
        _isActive = isActive,
        _createdAt = createdAt,
        super(
          id: id,
          email: email,
          username: username,
          phone: phone,
          country: country,
          currency: currency,
          kycLevel: kycLevel,
          isActive: isActive,
          createdAt: createdAt,
        );

  factory UserModel.fromJson(Map<String, dynamic> json) => _$UserModelFromJson(json);

  Map<String, dynamic> toJson() => _$UserModelToJson(this);

  /// Create UserModel from User entity
  factory UserModel.fromEntity(User user) {
    return UserModel(
      id: user.id,
      email: user.email,
      username: user.username,
      phone: user.phone,
      country: user.country,
      currency: user.currency,
      kycLevel: user.kycLevel,
      isActive: user.isActive,
      createdAt: user.createdAt,
    );
  }

  /// Convert to User entity
  @override
  User toEntity() => User(
        id: _id,
        email: _email,
        username: _username,
        phone: _phone,
        country: _country,
        currency: _currency,
        kycLevel: _kycLevel,
        isActive: _isActive,
        createdAt: _createdAt,
      );
}

/// Parse DateTime from JSON
DateTime _parseDateTime(dynamic value) {
  if (value is DateTime) return value;
  if (value is String) return DateTime.parse(value);
  return DateTime.now();
}
