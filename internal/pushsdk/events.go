package pushsdk

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/itplus/pushsdk-gateway/internal/store"
)

var vendorEventID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

type ProtocolError struct {
	Status  int
	Message string
}

func (e *ProtocolError) Error() string { return e.Message }

func badRequest(format string, args ...any) error {
	return &ProtocolError{Status: 400, Message: fmt.Sprintf(format, args...)}
}

func unprocessable(format string, args ...any) error {
	return &ProtocolError{Status: 422, Message: fmt.Sprintf(format, args...)}
}

type ParsedEvent struct {
	UUID   string
	Format string
	Record store.NewDeviceEvent
}

type eventEnvelope struct {
	EventNum  *int         `json:"eventNum"`
	EventList *[]eventItem `json:"eventList"`
}

type eventItem struct {
	UUID       string  `json:"UUID"`
	DataFormat string  `json:"dataFormat"`
	Data       *string `json:"data"`
}

// ParseEventBatch validates the documented PushSDK Event envelope and retains
// every source data value verbatim. The gateway deliberately does not inspect
// or classify the payload beneath eventList: that data belongs to the device,
// is persisted unchanged, and is shown to operators as received.
func ParseEventBatch(terminalSerial string, body []byte) ([]ParsedEvent, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var envelope eventEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return nil, badRequest("event envelope is not valid JSON: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, badRequest("event envelope contains a trailing JSON value")
	}
	if envelope.EventNum == nil || envelope.EventList == nil {
		return nil, badRequest("event envelope must contain eventNum and eventList")
	}
	if *envelope.EventNum < 0 || *envelope.EventNum > 20 {
		return nil, badRequest("eventNum must be between 0 and 20")
	}
	if len(*envelope.EventList) != *envelope.EventNum {
		return nil, badRequest("eventNum must equal eventList length")
	}

	seen := make(map[string]struct{}, len(*envelope.EventList))
	parsed := make([]ParsedEvent, 0, len(*envelope.EventList))
	for _, item := range *envelope.EventList {
		if !vendorEventID.MatchString(item.UUID) {
			return nil, badRequest("event UUID is invalid")
		}
		if _, exists := seen[item.UUID]; exists {
			return nil, badRequest("eventList contains a duplicate UUID")
		}
		seen[item.UUID] = struct{}{}
		if !supportedDataFormat(item.DataFormat) {
			return nil, badRequest("event %s has an unsupported dataFormat", item.UUID)
		}
		if item.Data == nil {
			return nil, badRequest("event %s has no data", item.UUID)
		}
		if _, err := base64.StdEncoding.DecodeString(*item.Data); err != nil {
			return nil, badRequest("event %s data is not base64: %v", item.UUID, err)
		}
		if item.DataFormat == "noData" && *item.Data != "" {
			return nil, badRequest("event %s noData payload must be empty", item.UUID)
		}
		parsed = append(parsed, ParsedEvent{
			UUID:   item.UUID,
			Format: item.DataFormat,
			Record: store.NewDeviceEvent{
				TerminalSerialNumber: terminalSerial,
				VendorEventID:        item.UUID,
				DataFormat:           item.DataFormat,
				PayloadBase64:        *item.Data,
			},
		})
	}
	return parsed, nil
}

func supportedDataFormat(format string) bool {
	return format == "jsonData" || format == "xmlData" || format == "boundaryData" || format == "noData"
}
