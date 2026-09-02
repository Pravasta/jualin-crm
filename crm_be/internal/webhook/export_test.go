package webhook

import (
	"context"
	"time"
)

// This file exists only for tests in package webhook_test, which drive the
// worker deterministically: a real ticker and a real clock would make
// backoff assertions approximate ("about an hour from now") when they can
// be exact.
//
// Kept in export_test.go rather than as exported API so nothing in
// production can reach in and replace the worker's clock or step its loop.

// SetWorkerClockForTest pins the worker's notion of now.
func SetWorkerClockForTest(w *Worker, now func() time.Time) {
	w.now = now
}

// RunWorkerTickForTest performs exactly one iteration of the loop body,
// without the ticker Run would otherwise wait on.
func RunWorkerTickForTest(w *Worker, ctx context.Context) {
	w.tick(ctx)
}
