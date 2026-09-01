package pushsdk

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestParseEventBatchPersistsExactAttendanceFields(t *testing.T) {
	payload := `{"eventType":"AccessControllerEvent","eventState":"active","dateTime":"2026-09-01T19:41:00+03:00","AccessControllerEvent":{"employeeNoString":"EMP-42","name":"Ava","currentVerifyMode":"face","attendanceStatus":"checkIn","statusValue":1}}`
	body := envelope("jsonData", payload)
	events, err := ParseEventBatch("DEVICE-1", body)
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if len(events) != 1 || events[0].Attendance == nil {
		t.Fatalf("expected one attendance event, got %#v", events)
	}
	record := events[0].Attendance
	if record.TerminalSerialNumber != "DEVICE-1" || record.EmployeeNumber != "EMP-42" || record.VerificationMethod != "face" || record.SourceFormat != "jsonData" {
		t.Fatalf("unexpected record: %#v", record)
	}
	if record.AttendanceStatus == nil || *record.AttendanceStatus != "checkIn" || record.StatusValue == nil || *record.StatusValue != 1 {
		t.Fatalf("optional fields changed: %#v", record)
	}
	want := time.Date(2026, 9, 1, 16, 41, 0, 0, time.UTC)
	if !record.OccurredAt.Equal(want) {
		t.Fatalf("occurredAt = %s, want %s", record.OccurredAt, want)
	}
}

func TestParseEventBatchDoesNotInventAttendanceStatus(t *testing.T) {
	payload := `{"eventType":"AccessControllerEvent","eventState":"active","dateTime":"2026-09-01T19:41:00Z","AccessControllerEvent":{"employeeNoString":"EMP-42","currentVerifyMode":"face"}}`
	events, err := ParseEventBatch("DEVICE-1", envelope("jsonData", payload))
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if events[0].Attendance.AttendanceStatus != nil || events[0].Attendance.StatusValue != nil {
		t.Fatalf("missing vendor fields must remain null: %#v", events[0].Attendance)
	}
}

func TestParseEventBatchAcceptsOnlyDocumentedDirectMultipart(t *testing.T) {
	metadata := `{"eventType":"AccessControllerEvent","eventState":"active","dateTime":"2026-09-01T19:41:00Z","AccessControllerEvent":{"employeeNoString":"EMP-42","currentVerifyMode":"face"}}`
	multipart := "Content-Type: multipart/form-data; boundary=abc123\r\n\r\n--abc123\r\nContent-Type: application/json\r\n\r\n" + metadata + "\r\n--abc123--\r\n"
	events, err := ParseEventBatch("DEVICE-1", envelope("boundaryData", multipart))
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if events[0].Format != "boundaryData" || events[0].Attendance == nil {
		t.Fatalf("unexpected event %#v", events[0])
	}
}

func TestParseEventBatchRejectsHeaderlessMultipartMetadata(t *testing.T) {
	payload := "Content-Type: multipart/form-data; boundary=abc123\r\n\r\n--abc123\r\n{\"eventType\":\"AccessControllerEvent\"}\r\n--abc123--\r\n"
	_, err := ParseEventBatch("DEVICE-1", envelope("boundaryData", payload))
	if err == nil {
		t.Fatal("expected strict multipart error")
	}
	protocol, ok := err.(*ProtocolError)
	if !ok || protocol.Status != 400 {
		t.Fatalf("error = %#v, want HTTP 400 protocol error", err)
	}
}

func TestParseEventBatchRejectsMissingEmployee(t *testing.T) {
	payload := `{"eventType":"AccessControllerEvent","eventState":"active","dateTime":"2026-09-01T19:41:00Z","AccessControllerEvent":{"currentVerifyMode":"face"}}`
	_, err := ParseEventBatch("DEVICE-1", envelope("jsonData", payload))
	if err == nil {
		t.Fatal("expected missing employee error")
	}
	protocol, ok := err.(*ProtocolError)
	if !ok || protocol.Status != 422 {
		t.Fatalf("error = %#v, want HTTP 422 protocol error", err)
	}
}

func TestParseEventBatchRequiresTheEntireDocumentedEnvelope(t *testing.T) {
	if _, err := ParseEventBatch("DEVICE-1", []byte(`{"eventNum":0}`)); err == nil {
		t.Fatal("eventList must not be inferred when it is absent")
	}
	if _, err := ParseEventBatch("DEVICE-1", []byte(`{"eventList":[]}`)); err == nil {
		t.Fatal("eventNum must not be inferred when it is absent")
	}
	if _, err := ParseEventBatch("DEVICE-1", []byte(`{"eventNum":1,"eventList":[{"UUID":"vendor-event-1","dataFormat":"noData"}]}`)); err == nil {
		t.Fatal("event data must not be inferred when it is absent")
	}
}

func envelope(dataFormat, data string) []byte {
	encoded := base64.StdEncoding.EncodeToString([]byte(data))
	return []byte(fmt.Sprintf(`{"eventNum":1,"eventList":[{"UUID":"vendor-event-1","dataFormat":%q,"data":%q}]}`, dataFormat, encoded))
}
