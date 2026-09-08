// Generates genuine text-format containers from repository-owned synthetic text.
// This is fixture authoring, not PDF conversion or acceptance testing.
package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const fixtureDir = "internal/module/preview/testdata/text"
const fixtureText = "File preview S05 Text 42"

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func write(extension string, data []byte) {
	must(os.WriteFile(filepath.Join(fixtureDir, "source."+extension), data, 0644))
}

func main() {
	// Macro-enabled template without a VBA project is a valid DOTM package;
	// the package content type must change, not just its filename extension.
	input, err := os.ReadFile(filepath.Join(fixtureDir, "source.dotx"))
	must(err)
	reader, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
	must(err)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	changed := false
	for _, entry := range reader.File {
		stream, err := entry.Open()
		must(err)
		data, err := io.ReadAll(stream)
		must(err)
		must(stream.Close())
		if entry.Name == "[Content_Types].xml" {
			from := []byte("application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml")
			if bytes.Count(data, from) != 1 {
				panic("unexpected DOTX content type")
			}
			data = bytes.Replace(data, from, []byte("application/vnd.ms-word.template.macroEnabledTemplate.main+xml"), 1)
			changed = true
		}
		header := entry.FileHeader
		part, err := writer.CreateHeader(&header)
		must(err)
		_, err = part.Write(data)
		must(err)
	}
	if !changed {
		panic("DOTX content types missing")
	}
	must(writer.Close())
	write("dotm", buffer.Bytes())

	abw := `<?xml version="1.0" encoding="UTF-8"?>
<abiword xmlns="http://www.abisource.com/awml.dtd" fileformat="1.0" version="1.0.0" template="false"><section><p style="Normal">` + fixtureText + `</p><p style="Normal">Résumé 中文</p></section></abiword>`
	write("abw", []byte(abw))
	buffer.Reset()
	compressed := gzip.NewWriter(&buffer)
	_, err = compressed.Write([]byte(abw))
	must(err)
	must(compressed.Close())
	write("zabw", buffer.Bytes())
	write("bib", []byte("@book{preview_s05,\n  author = {Preview Fixture},\n  title = {"+fixtureText+"},\n  year = {2026}\n}\n"))
	write("602", []byte("@CT 0\r\n"+fixtureText+"\r\n\x1a"))
	for _, extension := range []string{"sxw", "stw"} {
		mime := "application/vnd.sun.xml.writer"
		content := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="http://openoffice.org/2000/office" xmlns:text="http://openoffice.org/2000/text" office:class="text" office:version="1.0"><office:script/><office:font-decls/><office:automatic-styles/><office:body><text:p text:style-name="Standard">` + fixtureText + `</text:p><text:p text:style-name="Standard">Résumé 中文</text:p></office:body></office:document-content>`
		styles := `<?xml version="1.0" encoding="UTF-8"?><office:document-styles xmlns:office="http://openoffice.org/2000/office" xmlns:style="http://openoffice.org/2000/style" xmlns:fo="http://www.w3.org/1999/XSL/Format" office:version="1.0"><office:font-decls><style:font-decl style:name="Liberation Serif" fo:font-family="Liberation Serif"/></office:font-decls><office:styles><style:style style:name="Standard" style:family="paragraph" style:class="text"><style:properties style:font-name="Liberation Serif" fo:font-size="12pt"/></style:style></office:styles><office:automatic-styles><style:page-master style:name="pm1"><style:properties fo:page-width="21cm" fo:page-height="29.7cm" fo:margin-top="2cm" fo:margin-bottom="2cm" fo:margin-left="2cm" fo:margin-right="2cm"/></style:page-master></office:automatic-styles><office:master-styles><style:master-page style:name="Standard" style:page-master-name="pm1"/></office:master-styles></office:document-styles>`
		meta := `<?xml version="1.0" encoding="UTF-8"?><office:document-meta xmlns:office="http://openoffice.org/2000/office" xmlns:meta="http://openoffice.org/2000/meta" office:version="1.0"><office:meta><meta:user-defined meta:name="FixtureKind">` + extension + `</meta:user-defined></office:meta></office:document-meta>`
		settings := `<?xml version="1.0" encoding="UTF-8"?><office:document-settings xmlns:office="http://openoffice.org/2000/office" office:version="1.0"><office:settings/></office:document-settings>`
		manifest := `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="http://openoffice.org/2001/manifest"><manifest:file-entry manifest:media-type="` + mime + `" manifest:full-path="/"/><manifest:file-entry manifest:media-type="text/xml" manifest:full-path="content.xml"/><manifest:file-entry manifest:media-type="text/xml" manifest:full-path="styles.xml"/><manifest:file-entry manifest:media-type="text/xml" manifest:full-path="meta.xml"/><manifest:file-entry manifest:media-type="text/xml" manifest:full-path="settings.xml"/></manifest:manifest>`
		buffer.Reset()
		writer = zip.NewWriter(&buffer)
		for _, entry := range []struct{ name, body string }{{"content.xml", content}, {"styles.xml", styles}, {"meta.xml", meta}, {"settings.xml", settings}, {"META-INF/manifest.xml", manifest}} {
			method := uint16(zip.Deflate)
			if entry.name == "mimetype" {
				method = zip.Store
			}
			part, err := writer.CreateHeader(&zip.FileHeader{Name: entry.name, Method: method})
			must(err)
			_, err = io.Copy(part, strings.NewReader(entry.body))
			must(err)
		}
		must(writer.Close())
		write(extension, buffer.Bytes())
	}
	fmt.Println("Generated DOTM, AbiWord/gzip, BibTeX, T602 and OpenOffice XML fixtures; PDF acceptance not run.")
}
