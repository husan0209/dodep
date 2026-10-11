use std::sync::Arc;

use axum::{
    extract::State,
    http::{header, HeaderMap, Request, StatusCode},
    middleware::Next,
    response::Response,
};
// Import the specific items we need: `jwt_simple::prelude::*` also re-exports
// `ct_codecs::Decoder`, which shadows the `base64::Engine::decode` method.
use base64::Engine as _;
// Import the specific items we need: the glob prelude also re-exports
// `ct_codecs::Decoder`, which shadows `base64::Engine::decode`.
use jwt_simple::{
    algorithms::{Ed25519PublicKey, EdDSAPublicKeyLike},
    common::VerificationOptions,
};
use serde::Deserialize;

use crate::state::AppState;

#[derive(Clone, Debug)]
pub struct AuthUser {
    pub id: i64,
    pub roles: Vec<String>,
}

#[derive(Clone)]
struct AuthKey {
    key: Arc<Ed25519PublicKey>,
}

#[derive(Debug, Clone, Deserialize)]
struct CustomClaims {
    #[serde(default)]
    token_type: String,
    #[serde(default)]
    roles: String,
}

fn bearer_token(headers: &HeaderMap) -> Option<&str> {
    let v = headers.get(header::AUTHORIZATION)?.to_str().ok()?;
    let (scheme, token) = v.split_once(' ')?;
    if scheme.eq_ignore_ascii_case("bearer") {
        Some(token.trim())
    } else {
        None
    }
}

fn parse_roles(raw: Option<&str>) -> Vec<String> {
    raw.unwrap_or("")
        .split(',')
        .map(|s| s.trim())
        .filter(|s| !s.is_empty())
        .map(|s| s.to_string())
        .collect()
}

pub async fn require_auth(
    State(state): State<AppState>,
    headers: HeaderMap,
    mut req: Request<axum::body::Body>,
    next: Next,
) -> Result<Response, StatusCode> {
    let public_key_b64 = state.config().auth_ed25519_public_key.trim();
    if public_key_b64.is_empty() {
        // Auth not configured: refuse protected endpoints.
        return Err(StatusCode::SERVICE_UNAVAILABLE);
    }

    let token = bearer_token(&headers).ok_or(StatusCode::UNAUTHORIZED)?;

    let auth_key = {
        // Decode key per-request for now (MVP). If this becomes hot, cache in state.
        let raw = base64::engine::general_purpose::STANDARD
            .decode(public_key_b64)
            .map_err(|_| StatusCode::SERVICE_UNAVAILABLE)?;
        let pk = Ed25519PublicKey::from_bytes(&raw).map_err(|_| StatusCode::SERVICE_UNAVAILABLE)?;
        AuthKey { key: Arc::new(pk) }
    };

    // Enforce issuer to match the Go auth service.
    let options = VerificationOptions {
        allowed_issuers: Some(["opus-casino-auth".to_string()].into_iter().collect()),
        ..Default::default()
    };

    let claims = auth_key
        .key
        .verify_token::<CustomClaims>(token, Some(options))
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    // Go auth service sets subject = user_id (string-encoded bigint).
    let user_id: i64 = claims
        .subject
        .as_deref()
        .ok_or(StatusCode::UNAUTHORIZED)?
        .parse()
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    // Roles are optional in MVP: read from "roles" (comma-separated) if present.
    if !claims.custom.token_type.is_empty() && claims.custom.token_type != "access" {
        return Err(StatusCode::UNAUTHORIZED);
    }
    let roles = parse_roles(Some(claims.custom.roles.as_str()));

    req.extensions_mut().insert(AuthUser { id: user_id, roles });
    Ok(next.run(req).await)
}

pub fn require_admin(user: &AuthUser) -> Result<(), StatusCode> {
    if user
        .roles
        .iter()
        .any(|r| r == "admin" || r == "super_admin")
    {
        Ok(())
    } else {
        Err(StatusCode::FORBIDDEN)
    }
}
