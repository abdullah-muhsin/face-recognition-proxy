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
	event := `{"ipAddress":"192.0.2.10","dateTime":"2026-09-02T12:54:06+08:00","eventType":"AccessControllerEvent","eventDescription":"Access Controller Event","AccessControllerEvent":{"majorEventType":5,"subEventType":38,"employeeNoString":"1"}}`
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
	event := `{"ipAddress":"192.0.2.10","macAddress":"88:de:39:5d:b5:e1","channelID":1,"dateTime":"2026-09-02T13:32:56+08:00","activePostCount":1,"eventType":"AccessControllerEvent","eventState":"active","eventDescription":"Access Controller Event","shortSerialNumber":"GN0953967","AccessControllerEvent":{"deviceName":"Access Controller","majorEventType":5,"subEventType":75,"name":"test1","cardReaderNo":1,"doorNo":1,"employeeNoString":"2","serialNo":179,"frontSerialNo":178,"userType":"normal","currentVerifyMode":"faceOrFpOrCardOrPw","currentEvent":true,"mask":"no","picturesNumber":1,"purePwdVerifyEnable":true,"FaceRect":{"height":0.322,"width":0.183,"x":0.471,"y":0.492}}}`
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
	if projection.EventState == nil || *projection.EventState != "active" {
		t.Fatalf("event state = %#v", projection.EventState)
	}
	if projection.SourceMACAddress == nil || *projection.SourceMACAddress != "88:de:39:5d:b5:e1" {
		t.Fatalf("source MAC address = %#v", projection.SourceMACAddress)
	}
	if projection.ChannelID == nil || *projection.ChannelID != 1 || projection.ActivePostCount == nil || *projection.ActivePostCount != 1 {
		t.Fatalf("channel and post count = %#v, %#v", projection.ChannelID, projection.ActivePostCount)
	}
	if projection.ShortSerialNumber == nil || *projection.ShortSerialNumber != "GN0953967" || projection.DeviceName == nil || *projection.DeviceName != "Access Controller" {
		t.Fatalf("terminal context = %#v, %#v", projection.ShortSerialNumber, projection.DeviceName)
	}
	if projection.EventSerialNumber == nil || *projection.EventSerialNumber != 179 || projection.FrontSerialNumber == nil || *projection.FrontSerialNumber != 178 {
		t.Fatalf("event sequence = %#v, %#v", projection.EventSerialNumber, projection.FrontSerialNumber)
	}
	if projection.UserType == nil || *projection.UserType != "normal" || projection.CurrentVerifyMode == nil || *projection.CurrentVerifyMode != "faceOrFpOrCardOrPw" {
		t.Fatalf("verification context = %#v, %#v", projection.UserType, projection.CurrentVerifyMode)
	}
	if projection.CurrentEvent == nil || !*projection.CurrentEvent || projection.Mask == nil || *projection.Mask != "no" || projection.PicturesNumber == nil || *projection.PicturesNumber != 1 || projection.PurePwdVerifyEnable == nil || !*projection.PurePwdVerifyEnable {
		t.Fatalf("event context = %#v", projection)
	}
	if projection.FaceRect == nil || projection.FaceRect.Height == nil || *projection.FaceRect.Height != "0.322" || projection.FaceRect.Width == nil || *projection.FaceRect.Width != "0.183" || projection.FaceRect.X == nil || *projection.FaceRect.X != "0.471" || projection.FaceRect.Y == nil || *projection.FaceRect.Y != "0.492" {
		t.Fatalf("face rectangle = %#v", projection.FaceRect)
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

func TestSubtypeLabelsAreExplicitAndDocumented(t *testing.T) {
	label, known := SubtypeLabel(MajorEvent, 75)
	if !known || label != "Face Authentication Completed" {
		t.Fatalf("SubtypeLabel(5, 75) = %q, %t", label, known)
	}
	if _, known := SubtypeLabel(MajorEvent, 38); known {
		t.Fatal("SubtypeLabel inferred an undocumented subtype")
	}
}
