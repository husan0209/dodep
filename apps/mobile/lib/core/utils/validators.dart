/// Email validator
bool isValidEmail(String email) {
  return RegExp(r'^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$').hasMatch(email);
}

/// Password validator (min 8 chars, at least 1 uppercase, 1 lowercase, 1 number)
bool isValidPassword(String password) {
  return RegExp(r'^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)[a-zA-Z\d]{8,}$').hasMatch(password);
}

/// Phone validator (international format)
bool isValidPhone(String phone) {
  return RegExp(r'^\+?[1-9]\d{1,14}$').hasMatch(phone);
}

/// Currency code validator (ISO 4217)
bool isValidCurrencyCode(String code) {
  return RegExp(r'^[A-Z]{3}$').hasMatch(code);
}

/// Country code validator (ISO 3166-1 alpha-2)
bool isValidCountryCode(String code) {
  return RegExp(r'^[A-Z]{2}$').hasMatch(code);
}

/// UUID validator
bool isValidUuid(String uuid) {
  return RegExp(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
    caseSensitive: false,
  ).hasMatch(uuid);
}

/// Amount validator (positive number)
bool isValidAmount(num amount, {num? min, num? max}) {
  if (amount <= 0) return false;
  if (min != null && amount < min) return false;
  if (max != null && amount > max) return false;
  return true;
}

/// URL validator
bool isValidUrl(String url) {
  return Uri.tryParse(url)?.isAbsolute ?? false;
}
