use axum::http::StatusCode;

pub async fn liveness() -> StatusCode {
    StatusCode::OK
}

pub async fn readiness() -> StatusCode {
    StatusCode::OK
}
