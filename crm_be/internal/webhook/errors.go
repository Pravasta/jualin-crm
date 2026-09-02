package webhook

import "errors"

// ErrDeliveryNotRetryable is returned when a manual retry is requested
// for a delivery whose status is not 'failed' (kriteria #11). The
// usecase maps it to 409 delivery_not_retryable. A dedicated sentinel
// rather than a bare DomainError because both the pre-check in
// Usecase.RetryDelivery and the WHERE status='failed' guard in the
// repository need to recognize the same condition.
var ErrDeliveryNotRetryable = errors.New("webhook delivery not retryable")

// ErrDeliveryNotClaimed is returned by MarkResult when the row is no
// longer 'delivering' — the reaper returned it to the queue while this
// worker was mid-flight. A lost race, not a failure: the delivery will be
// retried by whoever owns it now, which is exactly the at-least-once
// behaviour TD §4.2 accepts.
var ErrDeliveryNotClaimed = errors.New("webhook: delivery no longer claimed")
