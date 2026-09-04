package subscription

import "errors"

// ErrNoActiveSubscription is returned by FindActiveByOrg when an
// organization has no row with status = 'active'. Every organization
// gets one at registration (auth.Register → CreateFree) and today
// nothing ever transitions it away from active, so this path is
// untested by any real data yet — but the query filters on status
// explicitly, so it must have a defined answer rather than an
// unchecked sql.ErrNoRows leaking out. ResolvePlan treats it as "every
// channel closed", never as a 500 (TD §1.1, kriteria #8 — fail closed).
var ErrNoActiveSubscription = errors.New("subscription: no active subscription")
