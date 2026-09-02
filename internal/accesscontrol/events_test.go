package accesscontrol

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestExtractProjectsDeclaredAccessControllerEvent(t *testing.T) {
	payload := `{"ipAddress":"10.9.96.20","dateTime":"2026-09-02T12:20:07+03:00","eventType":"AccessControllerEvent","eventDescription":"Authenticated via Face","AccessControllerEvent":{"majorEventType":5,"subEventType":75,"cardNo":"66","name":"Kelo","employeeNoString":"77","cardReaderNo":1,"doorNo":2}}`
	projection, ok := Extract("jsonData", base64.StdEncoding.EncodeToString([]byte(payload)))
	if !ok {
		t.Fatal("Extract() did not project declared access event")
	}
	if projection.MajorEventType != 5 || projection.SubEventType != 75 {
		t.Fatalf("event type = %d/%d, want 5/75", projection.MajorEventType, projection.SubEventType)
	}
	if projection.EventDescription == nil || *projection.EventDescription != "Authenticated via Face" {
		t.Fatalf("description = %#v", projection.EventDescription)
	}
	if projection.OccurredAt == nil || !projection.OccurredAt.Equal(time.Date(2026, 9, 2, 9, 20, 7, 0, time.UTC)) {
		t.Fatalf("occurred at = %#v", projection.OccurredAt)
	}
}

func TestExtractProjectsDeclaredBoundaryAccessControllerEvent(t *testing.T) {
	event := `{"ipAddress":"10.203.216.162","dateTime":"2026-09-02T12:54:06+08:00","eventType":"AccessControllerEvent","eventDescription":"Access Controller Event","AccessControllerEvent":{"majorEventType":5,"subEventType":38,"employeeNoString":"1"}}`
	payload := "Content-Type: multipart/form-data; boundary=MIME_boundary\r\n\r\n" +
		"--MIME_boundary\r\n" +
		"Content-Disposition: form-data; name=\"AccessControllerEvent\"\r\n" +
		"Content-Type: application/json; charset=\"UTF-8\"\r\n\r\n" +
		event + "\r\n--MIME_boundary--\r\n"
	projection, ok := Extract("boundaryData", base64.StdEncoding.EncodeToString([]byte(payload)))
	if !ok {
		t.Fatal("Extract() did not project declared boundary event")
	}
	if projection.MajorEventType != MajorEvent || projection.SubEventType != 38 {
		t.Fatalf("event type = %d/%d, want 5/38", projection.MajorEventType, projection.SubEventType)
	}
}

func TestExtractProjectsBoundaryEventWithPicturePart(t *testing.T) {
	event := `{"ipAddress":"10.203.216.162","dateTime":"2026-09-02T13:32:56+08:00","eventType":"AccessControllerEvent","eventDescription":"Access Controller Event","AccessControllerEvent":{"majorEventType":5,"subEventType":75,"name":"test1","cardReaderNo":1,"doorNo":1,"employeeNoString":"2","currentVerifyMode":"faceOrFpOrCardOrPw","FaceRect":{"height":0.322,"width":0.183,"x":0.471,"y":0.492}}}`
	payload := "Content-Type: multipart/form-data; boundary=MIME_boundary\r\n\r\n" +
		"--MIME_boundary\r\n" +
		"Content-Disposition: form-data; name=\"AccessControllerEvent\"\r\n" +
		"Content-Type: application/json; charset=\"UTF-8\"\r\n\r\n" +
		event + "\r\n" +
		"--MIME_boundary\r\n" +
		"Content-Disposition: form-data; name=\"Picture\"; filename=\"Picture.jpg\"\r\n" +
		"Content-Type: image/jpeg\r\n" +
		"Content-ID: pictureImage\r\n\r\n" +
		"\xff\xd8\xff\xe0JPEG\xff\xd9\r\n" +
		"--MIME_boundary--\r\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	projection, ok := Extract("boundaryData", encoded)
	if !ok {
		t.Fatal("Extract() did not project boundary event with picture")
	}
	if projection.MajorEventType != MajorEvent || projection.SubEventType != 75 {
		t.Fatalf("event type = %d/%d, want 5/75", projection.MajorEventType, projection.SubEventType)
	}
	if projection.EmployeeName == nil || *projection.EmployeeName != "test1" {
		t.Fatalf("employee name = %#v", projection.EmployeeName)
	}
	picture, ok := ExtractPicture("boundaryData", encoded)
	if !ok {
		t.Fatal("ExtractPicture() did not return declared picture")
	}
	if picture.ContentType != "image/jpeg" || picture.FileName != "Picture.jpg" || picture.ContentID != "pictureImage" {
		t.Fatalf("picture metadata = %#v", picture)
	}
	if picture.DataBase64 != base64.StdEncoding.EncodeToString([]byte("\xff\xd8\xff\xe0JPEG\xff\xd9")) {
		t.Fatal("picture bytes were changed")
	}
}

func TestExtractDoesNotInferAnUnsupportedOrIncompletePayload(t *testing.T) {
	for _, test := range []struct {
		name       string
		dataFormat string
		payload    string
	}{
		{"other event type", "jsonData", `{"eventType":"CertificateCaptureEvent","AccessControllerEvent":{"majorEventType":5,"subEventType":75}}`},
		{"missing subtype", "jsonData", `{"eventType":"AccessControllerEvent","AccessControllerEvent":{"majorEventType":5}}`},
		{"malformed occurrence time", "jsonData", `{"eventType":"AccessControllerEvent","dateTime":"02-09-2026","AccessControllerEvent":{"majorEventType":5,"subEventType":75}}`},
		{"non-json data", "boundaryData", `{"eventType":"AccessControllerEvent","AccessControllerEvent":{"majorEventType":5,"subEventType":75}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := Extract(test.dataFormat, base64.StdEncoding.EncodeToString([]byte(test.payload))); ok {
				t.Fatal("Extract() projected an unsupported payload")
			}
		})
	}
}

func TestCategoryCodesAreExplicitAndBidirectional(t *testing.T) {
	for category, major := range map[string]int{
		"alarm": MajorAlarm, "exception": MajorException, "operation": MajorOperation, "event": MajorEvent,
	} {
		if actual, ok := MajorEventTypeForCategory(category); !ok || actual != major {
			t.Fatalf("MajorEventTypeForCategory(%q) = %d, %t; want %d, true", category, actual, ok, major)
		}
		if actual, ok := CategoryForMajorEventType(major); !ok || actual != category {
			t.Fatalf("CategoryForMajorEventType(%d) = %q, %t; want %q, true", major, actual, ok, category)
		}
	}
	if _, ok := CategoryForMajorEventType(4); ok {
		t.Fatal("unmapped major type 4 has a category")
	}
}
