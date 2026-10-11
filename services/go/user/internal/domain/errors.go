package domain

import "errors"

// Sentinel errors for the user domain.
//
// These exist so transport layers can distinguish "the caller asked for
// something that does not exist" (404) from "the datastore failed" (500).
// Before they were introduced, a dropped database connection was reported to
// clients as "user not found", which both leaked infrastructure state and hid
// real incidents. Handlers must use errors.Is, never string comparison.
var (
	// ErrUserNotFound means the requested user does not exist (or is soft
	// deleted). It is a client-visible 404, not a failure.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidUserID means the identifier is structurally impossible
	// (non-positive). users.id is a BIGSERIAL, so 0 and negatives can never
	// match a row; rejecting them before the query keeps a malformed
	// identifier from reaching the database.
	ErrInvalidUserID = errors.New("invalid user id")
)
