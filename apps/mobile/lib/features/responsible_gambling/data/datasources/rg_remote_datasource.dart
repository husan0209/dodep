import '../models/rg_limits_model.dart';

/// Remote source for the RG cabinet. Talks to the Go RG service
/// (`/api/v1/rg/*`), which resolves the player from the JWT — no user id
/// is ever sent in the path or body (CONVENTIONS NEVER-7).
abstract class RGRemoteDataSource {
  /// GET /api/v1/rg/limits
  Future<RGLimitsEnvelopeModel> getLimits();

  /// PUT /api/v1/rg/limits
  Future<RGLimitsEnvelopeModel> setLimits(SetRGLimitsBody body);

  /// GET /api/v1/rg/status
  Future<RGStatusModel> getStatus();

  /// POST /api/v1/rg/self-exclusion
  Future<RGExclusionModel> startSelfExclusion(SelfExclusionBody body);

  /// POST /api/v1/rg/self-exclusion/revoke (expired temporary only).
  Future<void> revokeSelfExclusion(RevokeSelfExclusionBody body);

  /// POST /api/v1/rg/timeout
  Future<void> startTimeout(String period);

  /// POST /api/v1/rg/check — the same gate betting-engine/casino use.
  Future<bool> checkAllowed({required String channel, String? amount});
}
