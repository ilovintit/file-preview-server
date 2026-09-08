package infrastructure

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"github.com/richardlehane/mscfb"
)

// Includes the fixed converter/font bundle and all rendering options. Change this
// with any image/font/option change; input extensions cannot share interpretations.
const officeOutputVersion = "office-0ec4b0a125c55ff1dfaed071cab30e0f37cc4326f7d53d903ee5b24e4f27fd85-noexternal-v1"

func coreOffice(filename string) bool {
	switch strings.ToLower(path.Ext(filename)) {
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return true
	}
	return false
}

func outputVersion(filename string) string {
	if coreOffice(filename) {
		return officeOutputVersion + "-" + strings.TrimPrefix(strings.ToLower(path.Ext(filename)), ".")
	}
	return rawOutputVersion
}

func outputMIME(filename, contentType string) bool {
	if coreOffice(filename) {
		return contentType == "application/pdf"
	}
	return rawMIME(path.Ext(filename), contentType)
}

type officeConverter struct {
	url    string
	client *http.Client
}

func newOfficeConverter(endpoint string) *officeConverter {
	if endpoint == "" {
		return nil
	}
	return &officeConverter{url: strings.TrimRight(endpoint, "/") + "/forms/libreoffice/convert", client: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *officeConverter) convert(ctx context.Context, body []byte, filename string) ([]byte, error) {
	if c == nil {
		return nil, entity.ErrUnavailable
	}
	// One bounded in-memory input, no source URL or arbitrary caller form fields.
	// Pipe multipart framing to avoid copying the full 32 MiB input again.
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	done := make(chan struct{})
	go func() {
		defer close(done)
		part, err := form.CreateFormFile("files", "source"+strings.ToLower(path.Ext(filename)))
		if err == nil {
			_, err = part.Write(body)
		}
		if err == nil {
			err = form.WriteField("updateIndexes", "false")
		}
		if err == nil {
			err = form.WriteField("exportFormFields", "false")
		}
		if err == nil {
			err = form.WriteField("addOriginalDocumentAsStream", "false")
		}
		if err == nil {
			err = form.Close()
		}
		_ = writer.CloseWithError(err)
	}()
	defer func() { _ = reader.Close(); <-done }()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, reader)
	if err != nil {
		return nil, entity.ErrUnavailable
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	response, err := c.client.Do(req)
	if err != nil {
		return nil, entity.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 400 || response.StatusCode == 422 {
		return nil, entity.ErrInvalid
	}
	if response.StatusCode != 200 {
		return nil, entity.ErrUnavailable
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || contentType != "application/pdf" {
		return nil, entity.ErrUnavailable
	}
	output, err := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
	if err != nil || len(output) > 32<<20 {
		return nil, entity.ErrUnavailable
	}
	if err := validatePDF(ctx, output); err != nil {
		return nil, entity.ErrUnavailable
	}
	return output, nil
}

type officeReaderAt struct {
	ctx context.Context
	*bytes.Reader
	reads int
}

type officeStreamReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r officeStreamReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *officeReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	r.reads++
	if r.ctx.Err() != nil || r.reads > 262144 {
		return 0, entity.ErrInvalid
	}
	return r.Reader.ReadAt(p, offset)
}

func validateOffice(ctx context.Context, data []byte, ext string) (err error) {
	defer func() {
		if recover() != nil {
			err = entity.ErrInvalid
		}
	}()
	if ctx.Err() != nil {
		return entity.ErrUnavailable
	}
	if ext == ".doc" || ext == ".xls" || ext == ".ppt" {
		// Bound allocations driven by untrusted CFB header counts before the
		// parser allocates DIFAT, directory and mini-FAT slices.
		if len(data) < 512 {
			return entity.ErrInvalid
		}
		shift := binary.LittleEndian.Uint16(data[30:32])
		if shift != 9 && shift != 12 {
			return entity.ErrInvalid
		}
		sectors := uint64(len(data)) / (uint64(1) << shift)
		for _, offset := range []int{40, 44, 64, 72} {
			if uint64(binary.LittleEndian.Uint32(data[offset:offset+4])) > sectors {
				return entity.ErrInvalid
			}
		}
		difatCapacity := uint64(binary.LittleEndian.Uint32(data[72:76]))*((uint64(1)<<shift)/4-1) + 109
		if difatCapacity > sectors+((uint64(1)<<shift)/4)+109 {
			return entity.ErrInvalid
		}
		compound, err := mscfb.New(&officeReaderAt{ctx: ctx, Reader: bytes.NewReader(data)})
		if ctx.Err() != nil {
			return entity.ErrUnavailable
		}
		if err != nil || len(compound.File) > 4096 {
			return entity.ErrInvalid
		}
		found := false
		for _, entry := range compound.File {
			if ctx.Err() != nil {
				return entity.ErrUnavailable
			}
			if entry.FileInfo().IsDir() || len(entry.Path) != 0 {
				continue
			}
			if entry.Name == "EncryptedPackage" || entry.Name == "EncryptionInfo" {
				return entity.ErrInvalid
			}
			if (ext == ".doc" && entry.Name == "WordDocument") || (ext == ".xls" && (entry.Name == "Workbook" || entry.Name == "Book")) || (ext == ".ppt" && entry.Name == "PowerPoint Document") {
				if entry.Size <= 0 || entry.Size > int64(len(data)) {
					return entity.ErrInvalid
				}
				found = true
			}
		}
		if !found {
			return entity.ErrInvalid
		}
		return nil
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) > 4096 {
		return entity.ErrInvalid
	}
	wanted := map[string]string{".docx": "word/document.xml", ".xlsx": "xl/workbook.xml", ".pptx": "ppt/presentation.xml"}[ext]
	var total uint64
	seen := make(map[string]bool)
	for _, entry := range archive.File {
		if ctx.Err() != nil {
			return entity.ErrUnavailable
		}
		if seen[entry.Name] || path.Clean(entry.Name) != strings.TrimSuffix(entry.Name, "/") || strings.HasPrefix(entry.Name, "/") || strings.HasPrefix(entry.Name, "../") || strings.Contains(entry.Name, "\\") || entry.UncompressedSize64 > 64<<20 {
			return entity.ErrInvalid
		}
		seen[entry.Name] = true
		total += entry.UncompressedSize64
		if total > 64<<20 {
			return entity.ErrInvalid
		}
		stream, err := entry.Open()
		if err != nil {
			return entity.ErrInvalid
		}
		_, err = io.Copy(io.Discard, io.LimitReader(officeStreamReader{ctx: ctx, reader: stream}, 64<<20+1))
		_ = stream.Close()
		if ctx.Err() != nil {
			return entity.ErrUnavailable
		}
		if err != nil {
			return entity.ErrInvalid
		}
	}
	if wanted == "" || !seen[wanted] || !seen["[Content_Types].xml"] {
		return entity.ErrInvalid
	}
	return nil
}
