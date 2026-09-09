package entity

import "strings"

// SupportedExtension is the permanent product boundary; converters cannot expand it.
func SupportedExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".wps", ".et", ".dps":
		return true
	default:
		return false
	}
}
