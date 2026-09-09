package entity

import "testing"

func TestTC_R1_AllowedFormatBoundary(t *testing.T) {
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".wps", ".et", ".dps"} {
		t.Run("allow"+ext, func(t *testing.T) {
			if !SupportedExtension(ext) {
				t.Fatalf("required preview format rejected: %s", ext)
			}
		})
	}
	for _, ext := range []string{"", " ", ".docx.exe", "docx", ".avif", ".txt", ".rtf", ".csv", ".docm", ".xlsm", ".pptm", ".dot", ".odt", ".ods", ".odp", ".pages", ".numbers", ".key", ".svg", ".vsd", ".html", ".epub", ".psd", ".xlw", ".pxl", ".uof", ".psw"} {
		t.Run("reject"+ext, func(t *testing.T) {
			if SupportedExtension(ext) {
				t.Fatalf("permanently excluded format was accepted: %q", ext)
			}
		})
	}
}
