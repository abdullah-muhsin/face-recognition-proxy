package pushsdk

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestParseEventBatchPreservesTheExactSourcePayload(t *testing.T) {
	payload := `{"eventType":"AccessControllerEvent","eventState":"active","AccessControllerEvent":{"employeeNoString":"EMP-42"}}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	body := []byte(fmt.Sprintf(`{"eventNum":1,"eventList":[{"UUID":"event-1","dataFormat":"jsonData","data":%q}]}`, encoded))

	events, err := ParseEventBatch("DEVICE-1", body)
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	event := events[0]
	if event.Record.TerminalSerialNumber != "DEVICE-1" || event.Record.VendorEventID != "event-1" || event.Record.DataFormat != "jsonData" {
		t.Fatalf("unexpected event metadata: %#v", event.Record)
	}
	if event.Record.PayloadBase64 != encoded {
		t.Fatalf("payload was changed: got %q, want %q", event.Record.PayloadBase64, encoded)
	}
}

func TestParseEventBatchAcceptsAnEventWithoutAccessIdentityFields(t *testing.T) {
	payload := `{"eventType":"AccessControllerEvent","eventState":"active","eventDescription":"Door closed","AccessControllerEvent":{"subEventType":3}}`
	events, err := ParseEventBatch("DEVICE-1", envelope("jsonData", payload))
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if len(events) != 1 || events[0].Record.PayloadBase64 == "" {
		t.Fatalf("expected raw device event, got %#v", events)
	}
}

func TestParseEventBatchAcceptsVendorPayloadWithoutInterpretingIt(t *testing.T) {
	payload := `{"unrecognisedVendorField":[1,2,3]}`
	events, err := ParseEventBatch("DEVICE-1", envelope("jsonData", payload))
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
}

func TestParseEventBatchRetainsNoDataItems(t *testing.T) {
	events, err := ParseEventBatch("DEVICE-1", envelope("noData", ""))
	if err != nil {
		t.Fatalf("ParseEventBatch() error = %v", err)
	}
	if len(events) != 1 || events[0].Record.DataFormat != "noData" || events[0].Record.PayloadBase64 != "" {
		t.Fatalf("unexpected no-data event: %#v", events)
	}
}

func TestParseEventBatchRetainsBoundaryPayloadWithoutParsingIt(t *testing.T) {
	payload := "Content-Type: multipart/form-data; boundary=abc123\r\n\r\n--abc123\r\n{\"eventType\":\"AccessControllerEvent\"}\r\n--abc123--\r\n"
	events, err := ParseEventBatch("DEVICE-1", envelope("boundaryData", payload))
	if err != nil {
		t.Fatalf("raw boundary payload should be retained: %v", err)
	}
	if len(events) != 1 || events[0].Record.PayloadBase64 == "" {
		t.Fatalf("unexpected boundary event: %#v", events)
	}
}

func TestParseEventBatchRejectsInvalidEnvelopeData(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"eventNum":0}`),
		[]byte(`{"eventList":[]}`),
		[]byte(`{"eventNum":1,"eventList":[{"UUID":"vendor-event-1","dataFormat":"noData"}]}`),
		[]byte(`{"eventNum":1,"eventList":[{"UUID":"vendor-event-1","dataFormat":"jsonData","data":"not-base64"}]}`),
	} {
		if _, err := ParseEventBatch("DEVICE-1", body); err == nil {
			t.Fatalf("invalid event envelope was accepted: %s", body)
		}
	}
}

func envelope(dataFormat, data string) []byte {
	encoded := base64.StdEncoding.EncodeToString([]byte(data))
	return []byte(fmt.Sprintf(`{"eventNum":1,"eventList":[{"UUID":"vendor-event-1","dataFormat":%q,"data":%q}]}`, dataFormat, encoded))
}
