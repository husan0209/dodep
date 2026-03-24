// Auth middleware placeholder — will be implemented with JWT validation
// For now, user_id is extracted from path parameter

pub struct AuthUser {
    pub id: i64,
    pub roles: Vec<String>,
}
