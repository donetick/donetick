package auth

import "errors"

// MFARequiredError represents an error when MFA verification is required
type MFARequiredError struct {
	message string
}

func (e *MFARequiredError) Error() string {
	return e.message
}

func NewMFARequiredError() *MFARequiredError {
	return &MFARequiredError{
		message: "MFA verification required",
	}
}

// IsMFARequiredError checks if the error is an MFA required error
func IsMFARequiredError(err error) bool {
	var mfaErr *MFARequiredError
	return errors.As(err, &mfaErr)
}
