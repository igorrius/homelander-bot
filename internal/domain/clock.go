// Package domain holds Homelander's entities, the repository interfaces its
// services need, and the services themselves. It knows nothing about HTTP,
// Telegram or SQL.
package domain

import "time"

// Clock returns the current time; services take one so tests control time.
type Clock func() time.Time

// SystemClock is the real clock in UTC.
func SystemClock() time.Time { return time.Now().UTC() }
