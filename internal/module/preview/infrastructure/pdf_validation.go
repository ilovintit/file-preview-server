package infrastructure

import (
	"bytes"
	"context"
	"sync"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var pdfConfigurationOnce sync.Once

type pdfReader struct {
	*bytes.Reader
	ctx context.Context
}

func (r pdfReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
func (r pdfReader) Seek(offset int64, whence int) (int64, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Seek(offset, whence)
}

func validatePDF(ctx context.Context, data []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = entity.ErrInvalid
		}
	}()
	pdfConfigurationOnce.Do(pdfapi.DisableConfigDir)
	config := model.NewDefaultConfiguration()
	config.Optimize = false
	config.Offline = true
	config.ValidateLinks = false
	if validationErr := pdfapi.Validate(pdfReader{bytes.NewReader(data), ctx}, config); validationErr != nil {
		if ctx.Err() != nil {
			return entity.ErrUnavailable
		}
		return entity.ErrInvalid
	}
	return nil
}
