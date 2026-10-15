import 'package:injectable/injectable.dart';
import 'package:local_auth/local_auth.dart';

/// Biometric gate for sensitive wallet actions (withdraw).
///
/// Fail-secure: unavailable biometrics, user cancellation, or any error
/// returns false and the caller must abort the operation. Uses only
/// `local_auth`, already present in pubspec — no new packages.
@injectable
class WithdrawBiometricGate {
  final LocalAuthentication _auth;

  WithdrawBiometricGate({LocalAuthentication? auth})
      : _auth = auth ?? LocalAuthentication();

  /// Returns true only when the device owner authenticates.
  Future<bool> unlockBeforeWithdraw() async {
    try {
      final canCheck = await _auth.canCheckBiometrics;
      final supported = canCheck || await _auth.isDeviceSupported();
      if (!supported) {
        return false;
      }
      return await _auth.authenticate(
        localizedReason: 'Confirm your identity to withdraw funds.',
        options: const AuthenticationOptions(
          stickyAuth: true,
          biometricOnly: true,
        ),
      );
    } catch (_) {
      return false;
    }
  }
}
