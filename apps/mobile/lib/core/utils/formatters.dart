import 'package:intl/intl.dart';

/// Format money amount
String formatMoney(num amount, String currency) {
  return NumberFormat.currency(
    symbol: _getCurrencySymbol(currency),
    decimalDigits: 2,
    locale: 'ru_RU',
  ).format(amount);
}

/// Get currency symbol
String _getCurrencySymbol(String currency) {
  switch (currency.toUpperCase()) {
    case 'RUB':
      return '₽';
    case 'USD':
      return '\$';
    case 'EUR':
      return '€';
    case 'GBP':
      return '£';
    case 'KZT':
      return '₸';
    default:
      return '$currency ';
  }
}

/// Format odds
String formatOdds(num odds, String format) {
  switch (format.toLowerCase()) {
    case 'decimal':
      return odds.toStringAsFixed(2);
    case 'fractional':
      return _formatFractional(odds);
    case 'american':
      return _formatAmerican(odds);
    default:
      return odds.toStringAsFixed(2);
  }
}

/// Format odds as fractional
String _formatFractional(num odds) {
  if (odds <= 1) return '0/1';
  
  final profit = odds - 1;
  const denominator = 100;
  final numerator = (profit * denominator).round();
  
  final gcd = _gcd(numerator, denominator);
  return '${numerator ~/ gcd}/${denominator ~/ gcd}';
}

/// Format odds as American
String _formatAmerican(num odds) {
  if (odds >= 2.0) {
    return '+${((odds - 1) * 100).round()}';
  } else {
    return '${(-(100 / (odds - 1))).round()}';
  }
}

/// Calculate GCD
int _gcd(int a, int b) {
  while (b != 0) {
    final temp = b;
    b = a % b;
    a = temp;
  }
  return a;
}

/// Format date
String formatDate(DateTime date, {String pattern = 'dd.MM.yyyy HH:mm'}) {
  return DateFormat(pattern, 'ru_RU').format(date);
}

/// Format relative time (e.g., "5 min ago")
String formatRelativeTime(DateTime date) {
  final now = DateTime.now();
  final diff = now.difference(date);

  if (diff.inMinutes < 1) {
    return 'Только что';
  } else if (diff.inMinutes < 60) {
    return '${diff.inMinutes} мин назад';
  } else if (diff.inHours < 24) {
    return '${diff.inHours} ч назад';
  } else if (diff.inDays < 7) {
    return '${diff.inDays} дн назад';
  } else {
    return formatDate(date, pattern: 'dd.MM.yyyy');
  }
}

/// Parse ISO 8601 date string
DateTime? parseDateTime(String? isoString) {
  if (isoString == null || isoString.isEmpty) return null;
  return DateTime.tryParse(isoString);
}
