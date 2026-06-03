//go:build e2e
// +build e2e

package e2e

import "time"

// mqSyncedEventuallyTimeout is the default Synced/Ready wait for MQ CR specs (not QMC rotation).
const mqSyncedEventuallyTimeout = 3 * time.Minute

// qmcRotationEventuallyTimeout covers secret rotation and QMC recreate paths.
const qmcRotationEventuallyTimeout = 3 * time.Minute
