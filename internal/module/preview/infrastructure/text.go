package infrastructure

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"github.com/richardlehane/mscfb"
)

func textFormat(filename string) bool {
	switch strings.ToLower(path.Ext(filename)) {
	case ".docm", ".dot", ".dotm", ".dotx", ".odt", ".fodt", ".ott", ".rtf", ".txt", ".wps", ".wpd", ".pages", ".abw", ".zabw", ".lwp", ".mw", ".mcw", ".hwp", ".sxw", ".stw", ".sgl", ".vor", ".602", ".bib", ".xml", ".cwk", ".psw", ".uof":
		return true
	}
	return false
}

func prepareText(ctx context.Context, data []byte, filename string) (_ []byte, _ string, err error) {
	defer func() {
		if recover() != nil {
			err = entity.ErrInvalid
		}
	}()
	if ctx.Err() != nil {
		return nil, "", entity.ErrUnavailable
	}
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".txt", ".bib":
		decoded, err := decodeText(data)
		if err != nil {
			return nil, "", err
		}
		if ext == ".bib" && !validBibliography(decoded) {
			return nil, "", entity.ErrInvalid
		}
		wrapped, err := wrapText(ctx, decoded)
		return wrapped, "source.fodt", err
	case ".dot":
		err = validateOffice(ctx, data, ".doc")
	case ".docm", ".dotm", ".dotx", ".odt", ".ott", ".sxw", ".stw", ".pages":
		err = validateTextArchive(ctx, data, ext)
	case ".fodt", ".xml", ".uof", ".abw":
		var root xml.Name
		root, err = validateTextXML(ctx, data)
		if err == nil {
			valid := false
			switch ext {
			case ".fodt":
				valid = root.Local == "document" && root.Space == "urn:oasis:names:tc:opendocument:xmlns:office:1.0"
			case ".xml":
				valid = root.Local == "wordDocument" && root.Space == "http://schemas.microsoft.com/office/word/2003/wordml"
			case ".uof":
				valid = root.Local == "UOF" && root.Space == "http://schemas.uof.org/cn/2003/uof"
			case ".abw":
				valid = root.Local == "abiword" && (root.Space == "http://www.abisource.com/awml.dtd" || root.Space == "")
			}
			if !valid {
				err = entity.ErrInvalid
			}
		}
	case ".zabw":
		var reader *gzip.Reader
		reader, err = gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, "", entity.ErrInvalid
		}
		var expanded []byte
		expanded, err = io.ReadAll(io.LimitReader(officeStreamReader{ctx: ctx, reader: reader}, 64<<20+1))
		_ = reader.Close()
		if ctx.Err() != nil {
			return nil, "", entity.ErrUnavailable
		}
		if err != nil || len(expanded) > 64<<20 {
			return nil, "", entity.ErrInvalid
		}
		_, _, err = prepareText(ctx, expanded, "source.abw")
	case ".rtf":
		err = validateRTF(ctx, data)
	case ".sgl", ".vor":
		err = validateStarWriter(ctx, data, ext)
	case ".wps":
		if bytes.HasPrefix(data, []byte{1, 0xfe}) && len(data) >= 64 {
			break
		}
		err = validateCompoundText(ctx, data, []string{"MN0", "CONTENTS", "Contents"}, nil)
	case ".wpd":
		if len(data) < 64 || !bytes.HasPrefix(data, []byte{0xff, 'W', 'P', 'C'}) {
			err = entity.ErrInvalid
		}
	case ".cwk":
		if len(data) < 64 || data[0] < 1 || data[0] > 6 || !bytes.Equal(data[4:8], []byte("BOBO")) {
			err = entity.ErrInvalid
		}
	case ".mw", ".mcw":
		if len(data) < 64 || (binary.BigEndian.Uint16(data[:2]) != 4 && binary.BigEndian.Uint16(data[:2]) != 6) {
			err = entity.ErrInvalid
		}
	case ".psw":
		if len(data) < 64 || !bytes.HasPrefix(data, []byte(`{\pwi`)) {
			err = entity.ErrInvalid
		}
	case ".hwp":
		if len(data) < 256 || !bytes.HasPrefix(data, []byte("HWP Document File V3.00")) {
			err = entity.ErrInvalid
		}
	case ".lwp":
		if len(data) < 64 || !bytes.HasPrefix(data, []byte("WordPro\x00")) {
			err = entity.ErrInvalid
		}
	case ".602":
		if len(data) < 12 || !bytes.HasPrefix(data, []byte("@CT ")) {
			err = entity.ErrInvalid
		}
	default:
		err = entity.ErrInvalid
	}
	if err != nil {
		return nil, "", err
	}
	if ctx.Err() != nil {
		return nil, "", entity.ErrUnavailable
	}
	return data, filename, nil
}

func decodeText(data []byte) (string, error) {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		data = data[3:]
	}
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		little := data[0] == 0xff
		data = data[2:]
		if len(data)%2 != 0 {
			return "", entity.ErrInvalid
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			if little {
				units[i] = binary.LittleEndian.Uint16(data[2*i:])
			} else {
				units[i] = binary.BigEndian.Uint16(data[2*i:])
			}
		}
		data = []byte(string(utf16.Decode(units)))
	}
	if !utf8.Valid(data) {
		return "", entity.ErrInvalid
	}
	for _, r := range string(data) {
		if r == utf8.RuneError || (r < 32 && r != '\t' && r != '\r' && r != '\n') {
			return "", entity.ErrInvalid
		}
	}
	return strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n"), nil
}

func validBibliography(text string) bool {
	depth, entries := 0, 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "%") {
			continue
		}
		if strings.HasPrefix(line, "@") && (strings.Contains(line, "{") || strings.Contains(line, "(")) {
			entries++
		}
		for i := 0; i < len(line); i++ {
			if line[i] == '\\' {
				i++
				continue
			}
			switch line[i] {
			case '{', '(':
				depth++
			case '}', ')':
				depth--
				if depth < 0 {
					return false
				}
			}
		}
	}
	return entries > 0 && depth == 0
}

func wrapText(ctx context.Context, text string) ([]byte, error) {
	if int64(len(text))+int64(strings.Count(text, "&"))*4+int64(strings.Count(text, "<"))*3+int64(strings.Count(text, ">"))*3+int64(strings.Count(text, "\n"))*30 > 31<<20 {
		return nil, entity.ErrInvalid
	}
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" office:version="1.2" office:mimetype="application/vnd.oasis.opendocument.text"><office:body><office:text>`)
	for _, line := range strings.Split(text, "\n") {
		if ctx.Err() != nil {
			return nil, entity.ErrUnavailable
		}
		body.WriteString("<text:p>")
		if err := xml.EscapeText(&body, []byte(line)); err != nil {
			return nil, entity.ErrInvalid
		}
		body.WriteString("</text:p>")
	}
	body.WriteString("</office:text></office:body></office:document>")
	if body.Len() > 32<<20 {
		return nil, entity.ErrInvalid
	}
	return body.Bytes(), nil
}

func validateTextXML(ctx context.Context, data []byte, zipPath ...string) (xml.Name, error) {
	decoder := xml.NewDecoder(officeStreamReader{ctx: ctx, reader: bytes.NewReader(data)})
	var root xml.Name
	depth, roots, tokens := 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if ctx.Err() != nil {
			return root, entity.ErrUnavailable
		}
		if err != nil {
			return root, entity.ErrInvalid
		}
		tokens++
		if tokens > 2000000 {
			return root, entity.ErrInvalid
		}
		switch value := token.(type) {
		case xml.Directive:
			if strings.Contains(strings.ToUpper(string(value)), "DOCTYPE") {
				return root, entity.ErrInvalid
			}
		case xml.ProcInst:
			if strings.EqualFold(value.Target, "xml-stylesheet") {
				return root, entity.ErrInvalid
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return root, entity.ErrInvalid
			}
		case xml.StartElement:
			if depth == 0 {
				root = value.Name
				roots++
			}
			depth++
			if depth > 64 {
				return root, entity.ErrInvalid
			}
			external := false
			for _, attribute := range value.Attr {
				if attribute.Name.Local == "TargetMode" && strings.EqualFold(attribute.Value, "External") {
					external = true
				}
			}
			for _, attribute := range value.Attr {
				name := strings.ToLower(attribute.Name.Local)
				if name != "href" && name != "src" && name != "target" {
					continue
				}
				ref := strings.ToLower(strings.TrimSpace(attribute.Value))
				if name == "target" && value.Name.Local == "Relationship" && !external && len(zipPath) > 0 {
					if strings.Contains(ref, ":") || strings.Contains(ref, "\\") {
						return root, entity.ErrInvalid
					}
					resolved := path.Clean(path.Join(path.Dir(path.Dir(zipPath[0])), ref))
					if strings.HasPrefix(ref, "/") {
						resolved = strings.TrimPrefix(path.Clean(ref), "/")
					}
					if resolved == ".." || strings.HasPrefix(resolved, "../") {
						return root, entity.ErrInvalid
					}
					continue
				}
				if strings.HasPrefix(ref, "file:") || strings.HasPrefix(ref, "\\") || strings.HasPrefix(ref, "/") || strings.Contains(ref, "../") {
					return root, entity.ErrInvalid
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if roots != 1 || depth != 0 {
		return root, entity.ErrInvalid
	}
	return root, nil
}

func validateTextArchive(ctx context.Context, data []byte, ext string) error {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) > 4096 {
		return entity.ErrInvalid
	}
	content := map[string][]byte{}
	seen := map[string]bool{}
	var total uint64
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
		payload, err := io.ReadAll(io.LimitReader(officeStreamReader{ctx: ctx, reader: stream}, 64<<20+1))
		_ = stream.Close()
		if ctx.Err() != nil {
			return entity.ErrUnavailable
		}
		if err != nil || len(payload) > 64<<20 {
			return entity.ErrInvalid
		}
		if strings.HasSuffix(entry.Name, ".xml") || strings.HasSuffix(entry.Name, ".rels") {
			if _, err := validateTextXML(ctx, payload, entry.Name); err != nil {
				return err
			}
			content[entry.Name] = payload
		} else if entry.Name == "mimetype" {
			content[entry.Name] = payload
		}
	}
	switch ext {
	case ".docm", ".dotm", ".dotx":
		return validateOOXMLTextType(content, ext)
	case ".pages":
		if !seen["index.xml"] && !seen["index.xml.gz"] && !seen["Index/Document.iwa"] {
			return entity.ErrInvalid
		}
	default:
		mimes := map[string]string{".odt": "application/vnd.oasis.opendocument.text", ".ott": "application/vnd.oasis.opendocument.text-template", ".sxw": "application/vnd.sun.xml.writer", ".stw": "application/vnd.sun.xml.writer.template"}
		if string(content["mimetype"]) != mimes[ext] || len(content["content.xml"]) == 0 {
			return entity.ErrInvalid
		}
	}
	return nil
}

func validateOOXMLTextType(content map[string][]byte, ext string) error {
	wanted := map[string]string{".docm": "application/vnd.ms-word.document.macroEnabled.main+xml", ".dotm": "application/vnd.ms-word.template.macroEnabledTemplate.main+xml", ".dotx": "application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"}[ext]
	if len(content["word/document.xml"]) == 0 || wanted == "" {
		return entity.ErrInvalid
	}
	decoder := xml.NewDecoder(bytes.NewReader(content["[Content_Types].xml"]))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return entity.ErrInvalid
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Override" {
			continue
		}
		part, kind := "", ""
		for _, attribute := range start.Attr {
			if attribute.Name.Local == "PartName" {
				part = attribute.Value
			}
			if attribute.Name.Local == "ContentType" {
				kind = attribute.Value
			}
		}
		if part == "/word/document.xml" && kind == wanted {
			return nil
		}
	}
	return entity.ErrInvalid
}

func validateCompoundText(ctx context.Context, data []byte, names []string, check func(*mscfb.Reader, *mscfb.File) bool) error {
	compound, err := boundedCompound(ctx, data)
	if err != nil {
		return err
	}
	for _, entry := range compound.File {
		if entry.FileInfo().IsDir() || len(entry.Path) != 0 {
			continue
		}
		if entry.Name == "EncryptedPackage" || entry.Name == "EncryptionInfo" {
			return entity.ErrInvalid
		}
		for _, name := range names {
			if entry.Name == name && entry.Size > 0 && entry.Size <= int64(len(data)) {
				if check == nil || check(compound, entry) {
					return nil
				}
			}
		}
	}
	return entity.ErrInvalid
}

func validateStarWriter(ctx context.Context, data []byte, ext string) error {
	return validateCompoundText(ctx, data, []string{"StarWriterDocument"}, func(compound *mscfb.Reader, entry *mscfb.File) bool {
		var header [16]byte
		if _, err := io.ReadFull(entry, header[:]); err != nil {
			return false
		}
		if !bytes.HasPrefix(header[:], []byte("SW3HDR")) && !bytes.HasPrefix(header[:], []byte("SW4HDR")) && !bytes.HasPrefix(header[:], []byte("SW5HDR")) {
			return false
		}
		master := strings.EqualFold(compound.ID(), "c20cf9d3-85ae-11d1-aab4-006097da561a")
		return (ext == ".sgl" && master) || (ext == ".vor" && !master)
	})
}

func validateRTF(ctx context.Context, data []byte) error {
	if !bytes.HasPrefix(data, []byte(`{\rtf`)) {
		return entity.ErrInvalid
	}
	lower := bytes.ToLower(data)
	if bytes.Contains(lower, []byte(`\object`)) || (bytes.Contains(lower, []byte(`\fldinst`)) && (bytes.Contains(lower, []byte("includetext")) || bytes.Contains(lower, []byte("includepicture")))) {
		return entity.ErrInvalid
	}
	depth := 0
	for i := 0; i < len(data); i++ {
		if i%4096 == 0 && ctx.Err() != nil {
			return entity.ErrUnavailable
		}
		switch data[i] {
		case '{':
			depth++
			if depth > 256 {
				return entity.ErrInvalid
			}
		case '}':
			depth--
			if depth < 0 {
				return entity.ErrInvalid
			}
		case '\\':
			if i+4 < len(data) && string(data[i+1:i+4]) == "bin" {
				start := i + 4
				end := start
				for end < len(data) && data[end] >= '0' && data[end] <= '9' {
					end++
				}
				if end > start {
					n, err := strconv.Atoi(string(data[start:end]))
					if err != nil || n > 32<<20 {
						return entity.ErrInvalid
					}
					if end < len(data) && data[end] == ' ' {
						end++
					}
					if end+n > len(data) {
						return entity.ErrInvalid
					}
					i = end + n - 1
					continue
				}
			}
			i++
		}
	}
	if depth != 0 {
		return entity.ErrInvalid
	}
	return nil
}
