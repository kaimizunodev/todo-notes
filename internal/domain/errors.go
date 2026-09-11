package domain

// ValidationError menandakan input yang melanggar aturan bisnis
// (dipetakan ke HTTP 422 oleh layer handler).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func NewValidationError(msg string) error { return &ValidationError{Msg: msg} }

// NotFoundError menandakan entity yang dicari tidak ada
// (dipetakan ke HTTP 404 oleh layer handler).
type NotFoundError struct{ Msg string }

func (e *NotFoundError) Error() string { return e.Msg }

func NewNotFoundError(msg string) error { return &NotFoundError{Msg: msg} }
