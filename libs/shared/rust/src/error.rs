//! Error handling for Opus Casino platform

use thiserror::Error;
use serde::{Serialize, Deserialize};

/// Application error type
#[derive(Debug, Error)]
pub enum AppError {
    #[error("Validation error: {0}")]
    ValidationError(String),
    
    #[error("Authentication error: {0}")]
    AuthError(String),
    
    #[error("Authorization error: {0}")]
    AuthzError(String),
    
    #[error("Not found: {0}")]
    NotFound(String),
    
    #[error("Already exists: {0}")]
    AlreadyExists(String),
    
    #[error("Invalid argument: {0}")]
    InvalidArgument(String),
    
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    
    #[error("Rate limit exceeded: {0}")]
    RateLimitExceeded(String),
    
    #[error("Service unavailable: {0}")]
    ServiceUnavailable(String),
    
    #[error("Internal error: {0}")]
    InternalError(String),
    
    #[error("Database error: {0}")]
    DatabaseError(#[from] sqlx::Error),
    
    #[error("Redis error: {0}")]
    RedisError(#[from] redis::RedisError),
    
    #[error("Serialization error: {0}")]
    SerializationError(#[from] serde_json::Error),
    
    #[error("Parse error: {0}")]
    ParseError(String),
    
    #[error("Business rule violation: {0}")]
    BusinessRuleViolation(String),
}

/// Application result type alias
pub type AppResult<T> = Result<T, AppError>;

/// Error details for API responses
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorDetails {
    pub error_code: String,
    pub error_message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<std::collections::HashMap<String, String>>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub field_errors: Vec<FieldError>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub trace_id: Option<String>,
}

/// Field validation error
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FieldError {
    pub field: String,
    pub error_code: String,
    pub error_message: String,
}

impl ErrorDetails {
    pub fn new(error_code: &str, error_message: &str) -> Self {
        Self {
            error_code: error_code.to_string(),
            error_message: error_message.to_string(),
            metadata: None,
            field_errors: Vec::new(),
            trace_id: None,
        }
    }
    
    pub fn with_metadata(mut self, metadata: std::collections::HashMap<String, String>) -> Self {
        self.metadata = Some(metadata);
        self
    }
    
    pub fn with_field_error(mut self, field_error: FieldError) -> Self {
        self.field_errors.push(field_error);
        self
    }
    
    pub fn with_trace_id(mut self, trace_id: &str) -> Self {
        self.trace_id = Some(trace_id.to_string());
        self
    }
}

impl From<AppError> for ErrorDetails {
    fn from(err: AppError) -> Self {
        let error_code = match &err {
            AppError::ValidationError(_) => "VALIDATION_ERROR",
            AppError::AuthError(_) => "AUTH_ERROR",
            AppError::AuthzError(_) => "AUTHZ_ERROR",
            AppError::NotFound(_) => "NOT_FOUND",
            AppError::AlreadyExists(_) => "ALREADY_EXISTS",
            AppError::InvalidArgument(_) => "INVALID_ARGUMENT",
            AppError::InsufficientBalance(_) => "INSUFFICIENT_BALANCE",
            AppError::RateLimitExceeded(_) => "RATE_LIMIT_EXCEEDED",
            AppError::ServiceUnavailable(_) => "SERVICE_UNAVAILABLE",
            AppError::InternalError(_) => "INTERNAL_ERROR",
            AppError::DatabaseError(_) => "DATABASE_ERROR",
            AppError::RedisError(_) => "REDIS_ERROR",
            AppError::SerializationError(_) => "SERIALIZATION_ERROR",
            AppError::ParseError(_) => "PARSE_ERROR",
            AppError::BusinessRuleViolation(_) => "BUSINESS_RULE_VIOLATION",
        };
        
        ErrorDetails::new(error_code, &err.to_string())
    }
}

/// Field error builder
pub struct FieldErrorBuilder {
    field: String,
    error_code: String,
    error_message: String,
}

impl FieldErrorBuilder {
    pub fn new(field: &str, error_code: &str, error_message: &str) -> Self {
        Self {
            field: field.to_string(),
            error_code: error_code.to_string(),
            error_message: error_message.to_string(),
        }
    }
    
    pub fn build(self) -> FieldError {
        FieldError {
            field: self.field,
            error_code: self.error_code,
            error_message: self.error_message,
        }
    }
}

/// Convenience macros for creating errors
#[macro_export]
macro_rules! validation_error {
    ($msg:expr) => {
        $crate::error::AppError::ValidationError($msg.to_string())
    };
    ($fmt:expr, $($arg:tt)*) => {
        $crate::error::AppError::ValidationError(format!($fmt, $($arg)*))
    };
}

#[macro_export]
macro_rules! not_found {
    ($entity:expr, $id:expr) => {
        $crate::error::AppError::NotFound(format!("{} not found: {}", $entity, $id))
    };
}

#[macro_export]
macro_rules! invalid_argument {
    ($msg:expr) => {
        $crate::error::AppError::InvalidArgument($msg.to_string())
    };
}

#[macro_export]
macro_rules! insufficient_balance {
    ($msg:expr) => {
        $crate::error::AppError::InsufficientBalance($msg.to_string())
    };
}
