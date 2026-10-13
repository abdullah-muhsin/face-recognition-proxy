package accesscontrol

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"image/jpeg"
)

const MaxPictureBytes = 5 * 1024 * 1024

type Attachment struct {
	ContentType string `json:"contentType"`
	ByteSize    int    `json:"byteSize"`
	SHA256      string `json:"sha256"`
	DataBase64  string `json:"dataBase64"`
}

// DeliveryPicture classifies missing evidence explicitly; it never substitutes
// a portrait, follows a device URL, or decodes an undeclared image part.
func DeliveryPicture(format, payload string) (*Attachment, string, *string) {
	unavailable := func(reason string) (*Attachment, string, *string) { return nil, "unavailable", &reason }
	projection, ok := Extract(format, payload)
	if !ok {
		return unavailable("invalid_multipart")
	}
	count := projection.PicturesNumber
	if count != nil && (*count < 0 || *count > 1) {
		return unavailable("unsupported_picture_count")
	}
	var raw []byte
	if format == "boundaryData" {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return unavailable("invalid_multipart")
		}
		parts, err := multipartFormParts(decoded)
		if err != nil {
			return unavailable("invalid_multipart")
		}
		found := false
		for _, part := range parts {
			if part.name != "Picture" {
				continue
			}
			if found || part.mediaType != "image/jpeg" || part.fileName == "" {
				return unavailable("invalid_picture")
			}
			found = true
			raw = part.data
		}
		if found && len(raw) == 0 {
			return unavailable("invalid_picture")
		}
	}
	if raw == nil {
		if count != nil && *count > 0 {
			return unavailable("missing_picture")
		}
		return nil, "not_provided", nil
	}
	if count != nil && *count == 0 {
		return unavailable("invalid_picture")
	}
	if len(raw) > MaxPictureBytes {
		return unavailable("picture_too_large")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return unavailable("invalid_jpeg")
	}
	if _, err = jpeg.Decode(bytes.NewReader(raw)); err != nil {
		return unavailable("invalid_jpeg")
	}
	digest := sha256.Sum256(raw)
	return &Attachment{ContentType: "image/jpeg", ByteSize: len(raw), SHA256: hex.EncodeToString(digest[:]), DataBase64: base64.StdEncoding.EncodeToString(raw)}, "available", nil
}
