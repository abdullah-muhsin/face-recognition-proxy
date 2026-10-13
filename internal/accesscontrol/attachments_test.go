package accesscontrol

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"testing"
)

func TestDeliveryPictureExactBytesAndUnavailability(t *testing.T) {
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewGray(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	metadata := func(count int) string {
		return fmt.Sprintf(`{"eventType":"AccessControllerEvent","eventState":"active","dateTime":"2026-10-13T08:00:00+03:00","AccessControllerEvent":{"majorEventType":5,"subEventType":75,"employeeNoString":"007","picturesNumber":%d}}`, count)
	}
	multipart := func(count int, body []byte, extra string) string {
		return base64.StdEncoding.EncodeToString([]byte("Content-Type: multipart/form-data; boundary=test\r\n\r\n--test\r\nContent-Disposition: form-data; name=\"AccessControllerEvent\"\r\nContent-Type: application/json\r\n\r\n" + metadata(count) + "\r\n--test\r\nContent-Disposition: form-data; name=\"Picture\"; filename=\"Picture.jpg\"\r\nContent-Type: image/jpeg\r\n\r\n" + string(body) + extra + "\r\n--test--\r\n"))
	}
	a, status, reason := DeliveryPicture("boundaryData", multipart(1, jpegBytes.Bytes(), ""))
	if status != "available" || reason != nil || a == nil || a.DataBase64 != base64.StdEncoding.EncodeToString(jpegBytes.Bytes()) || a.SHA256 != fmt.Sprintf("%x", sha256.Sum256(jpegBytes.Bytes())) {
		t.Fatal("exact JPEG was not preserved")
	}
	cases := []struct{ format, payload, status, reason string }{
		{"jsonData", base64.StdEncoding.EncodeToString([]byte(metadata(0))), "not_provided", ""},
		{"jsonData", base64.StdEncoding.EncodeToString([]byte(metadata(1))), "unavailable", "missing_picture"},
		{"boundaryData", multipart(1, []byte("invalid"), ""), "unavailable", "invalid_jpeg"},
		{"boundaryData", multipart(2, jpegBytes.Bytes(), ""), "unavailable", "unsupported_picture_count"},
		{"boundaryData", multipart(0, jpegBytes.Bytes(), ""), "unavailable", "invalid_picture"},
		{"boundaryData", multipart(1, bytes.Repeat([]byte{1}, MaxPictureBytes+1), ""), "unavailable", "picture_too_large"},
		{"boundaryData", multipart(1, jpegBytes.Bytes(), "\r\n--test\r\nContent-Disposition: form-data; name=\"Picture\"; filename=\"second.jpg\"\r\nContent-Type: image/jpeg\r\n\r\nsecond"), "unavailable", "invalid_picture"},
	}
	for _, c := range cases {
		a, status, reason := DeliveryPicture(c.format, c.payload)
		got := ""
		if reason != nil {
			got = *reason
		}
		if a != nil || status != c.status || got != c.reason {
			t.Fatalf("expected %s/%s, got %s/%s", c.status, c.reason, status, got)
		}
	}
}
