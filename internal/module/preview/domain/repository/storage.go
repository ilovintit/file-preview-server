package repository

import "context"

// ObjectStorage keeps provider SDKs out of the shared preview lifecycle.
// Missing objects return entity.ErrNotFound; dependency errors fail closed.
type ObjectStorage interface {
	Put(context.Context, string, string, []byte) error
	Stat(context.Context, string) error
	Delete(context.Context, string) error
	SignGet(context.Context, string, int64) (location string, expiresAt int64, err error)
	Check(context.Context) error
}

type HealthChecker interface{ Check(context.Context) error }
