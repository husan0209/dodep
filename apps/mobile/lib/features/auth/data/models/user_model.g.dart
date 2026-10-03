// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

UserModel _$UserModelFromJson(Map<String, dynamic> json) => UserModel(
      id: (json['id'] as num).toInt(),
      email: json['email'] as String,
      username: json['username'] as String,
      phone: json['phone'] as String?,
      country: json['country'] as String?,
      currency: json['currency'] as String?,
      kycLevel: (json['kycLevel'] as num?)?.toInt() ?? 0,
      isActive: json['isActive'] as bool? ?? true,
      createdAt: DateTime.parse(json['createdAt'] as String),
    );

Map<String, dynamic> _$UserModelToJson(UserModel instance) => <String, dynamic>{
      'id': instance.id,
      'email': instance.email,
      'username': instance.username,
      'phone': instance.phone,
      'country': instance.country,
      'currency': instance.currency,
      'kycLevel': instance.kycLevel,
      'isActive': instance.isActive,
      'createdAt': instance.createdAt.toIso8601String(),
    };
