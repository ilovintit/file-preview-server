package repository

import (
	"context"
	"github.com/ilovintit/file-preview-server/internal/module/preview/domain/entity"
)

type TokenStore interface {
	Create(context.Context, entity.Grant) (bool, error)
	List(context.Context) ([]entity.Grant, error)
	Get(context.Context, string) (*entity.Grant, error)
	Revoke(context.Context, string) error
}

type PreviewPreparer interface {
	Prepare(context.Context, entity.Grant) (string, error)
}
