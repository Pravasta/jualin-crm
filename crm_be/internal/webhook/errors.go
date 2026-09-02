package webhook

import "errors"

// ErrDeliveryNotRetryable is returned when a manual retry is requested
// for a delivery whose status is not 'failed' (kriteria #11). The
// usecase maps it to 409 delivery_not_retryable. A dedicated sentinel
// rather than a bare DomainError because both the pre-check in
// Usecase.RetryDelivery and the WHERE status='failed' guard in the
// repository need to recognize the same condition.
var ErrDeliveryNotRetryable = errors.New("webhook delivery not retryable")
