package repository

import (
	"context"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
)

type TokenStore interface {
	Create(context.Context, entity.Grant) (bool, error)
	List(context.Context) ([]entity.Grant, error)
	Get(context.Context, string) (*entity.Grant, error)
	Revoke(context.Context, string) error
}
