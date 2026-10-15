package service

// Real channel providers live in internal/client (SendGridSender,
// TwilioSender, FirebaseSender, gateway publisher) and are wired in main.go
// via wireChannelSenders + the Set* hooks. This file is intentionally empty:
// an earlier draft duplicated them here and was removed to keep a single
// implementation per provider (CONVENTIONS: no duplicated services).
