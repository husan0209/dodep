import 'package:equatable/equatable.dart';

/// User entity
class User extends Equatable {
  final int id;
  final String email;
  final String username;
  final String? phone;
  final String? country;
  final String? currency;
  final int kycLevel;
  final bool isActive;
  final DateTime createdAt;

  const User({
    required this.id,
    required this.email,
    required this.username,
    this.phone,
    this.country,
    this.currency,
    this.kycLevel = 0,
    this.isActive = true,
    required this.createdAt,
  });

  @override
  List<Object?> get props => [
        id,
        email,
        username,
        phone,
        country,
        currency,
        kycLevel,
        isActive,
        createdAt,
      ];

  /// Check if user is verified (KYC level >= 1)
  bool get isVerified => kycLevel >= 1;

  /// Check if user can withdraw (KYC level >= 2)
  bool get canWithdraw => kycLevel >= 2;
}
