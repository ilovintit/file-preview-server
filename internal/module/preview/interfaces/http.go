package interfaces

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/application"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

type Controller struct{ service *application.Service }

func New(s *application.Service) *Controller { return &Controller{service: s} }

var (
	codeInvalid      = gcode.New(42201, "参数不合法", http.StatusUnprocessableEntity)
	codeUnauthorized = gcode.New(40101, "请求未授权", http.StatusUnauthorized)
	codeUnavailable  = gcode.New(50301, "服务暂不可用", http.StatusServiceUnavailable)
	codeNotFound     = gcode.New(40401, "预览链接已失效", http.StatusNotFound)
)

func (c *Controller) Error(w http.ResponseWriter, r *http.Request, err error) {
	code := codeUnavailable
	switch {
	case errors.Is(err, entity.ErrInvalid):
		code = codeInvalid
	case errors.Is(err, entity.ErrUnauthorized):
		code = codeUnauthorized
	case errors.Is(err, entity.ErrNotFound):
		code = codeNotFound
	}
	gc := gerror.Code(gerror.NewCode(code))
	c.write(w, r, gc.Detail().(int), gc.Code(), gc.Message(), struct{}{})
}
func (c *Controller) write(w http.ResponseWriter, _ *http.Request, status, code int, message string, data any) {
	var id [16]byte
	_, _ = rand.Read(id[:])
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		TraceID string `json:"traceId"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}{hex.EncodeToString(id[:]), code, message, data})
}

// readJSON rejects ambiguous canonical payloads before binding transport DTOs.
func readJSON(r *http.Request, target any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return entity.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	if err != nil || len(raw) > 65536 || !utf8.Valid(raw) {
		return entity.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = scanJSON(d, 0); err != nil {
		return entity.ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return entity.ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return entity.ErrInvalid
	}
	return nil
}

func scanJSON(d *json.Decoder, depth int) error {
	if depth > 32 {
		return entity.ErrInvalid
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return entity.ErrInvalid
			}
			seen[s] = true
			if err = scanJSON(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err = scanJSON(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return entity.ErrInvalid
	}
	_, err = d.Token()
	return err
}

func (c *Controller) Issue(w http.ResponseWriter, r *http.Request, caller string) {
	var input entity.IssueInput
	if err := readJSON(r, &input); err != nil {
		c.Error(w, r, err)
		return
	}
	value, err := c.service.Issue(r.Context(), caller, input)
	if err != nil {
		c.Error(w, r, err)
		return
	}
	c.write(w, r, 200, 0, "成功", value)
}
func (c *Controller) Query(w http.ResponseWriter, r *http.Request, _ string) {
	var input map[string]json.RawMessage
	if err := readJSON(r, &input); err != nil || input == nil {
		c.Error(w, r, entity.ErrInvalid)
		return
	}
	limit, offset := 20, 0
	for key, raw := range input {
		if key != "limit" && key != "offset" {
			c.Error(w, r, entity.ErrInvalid)
			return
		}
		v, err := strconv.Atoi(string(raw))
		if err != nil {
			c.Error(w, r, entity.ErrInvalid)
			return
		}
		if key == "limit" {
			limit = v
		} else {
			offset = v
		}
	}
	value, err := c.service.Query(r.Context(), limit, offset)
	if err != nil {
		c.Error(w, r, err)
		return
	}
	c.write(w, r, 200, 0, "成功", value)
}
func (c *Controller) Revoke(w http.ResponseWriter, r *http.Request, _ string) {
	var input map[string]json.RawMessage
	if err := readJSON(r, &input); err != nil || input == nil || len(input) > 0 {
		c.Error(w, r, entity.ErrInvalid)
		return
	}
	token := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/tokens/"), "/revoke")
	if err := c.service.Revoke(r.Context(), token); err != nil {
		c.Error(w, r, err)
		return
	}
	c.write(w, r, 200, 0, "成功", struct{}{})
}
func (c *Controller) Preview(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/v/")
	location, err := c.service.PreparePreview(r.Context(), token)
	if err != nil {
		c.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.Redirect(w, r, location, http.StatusFound)
}
func (c *Controller) Live(w http.ResponseWriter, r *http.Request) {
	c.write(w, r, 200, 0, "存活", struct{}{})
}
func (c *Controller) Ready(w http.ResponseWriter, r *http.Request) {
	if err := c.service.Ready(r.Context()); err != nil {
		c.Error(w, r, err)
		return
	}
	c.write(w, r, 200, 0, "就绪", struct{}{})
}
