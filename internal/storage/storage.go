// Package storage holds the interfaces the domain uses to reach persistent
// storage. Implementations live in subpackages (sqlite today).
package storage

import "context"

// TxRunner runs fn in one transaction; an error from fn rolls it back.
// Repositories called with the ctx passed to fn take part in the transaction.
type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
