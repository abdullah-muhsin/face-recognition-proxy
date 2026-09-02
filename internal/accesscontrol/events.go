// Package accesscontrol defines the declared AccessControllerEvent read model.
// It is deliberately separate from PushSDK envelope validation: a source event
// remains valid and archived even when it is not this exact JSON event form.
package accesscontrol

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"time"
)

const (
	SchemaVersion  = 1
	MajorAlarm     = 1
	MajorException = 2
	MajorOperation = 3
	MajorEvent     = 5
)

func MajorEventTypeForCategory(category string) (int, bool) {
	switch category {
	case "alarm":
		return MajorAlarm, true
	case "exception":
		return MajorException, true
	case "operation":
		return MajorOperation, true
	case "event":
		return MajorEvent, true
	default:
		return 0, false
	}
}

func CategoryForMajorEventType(major int) (string, bool) {
	switch major {
	case MajorAlarm:
		return "alarm", true
	case MajorException:
		return "exception", true
	case MajorOperation:
		return "operation", true
	case MajorEvent:
		return "event", true
	default:
		return "", false
	}
}

type Projection struct {
	MajorEventType   int
	SubEventType     int
	EventDescription *string
	OccurredAt       *time.Time
	EmployeeNumber   *string
	EmployeeName     *string
	CardNumber       *string
	CardReaderNumber *int
	DoorNumber       *int
	SourceIPAddress  *string
}

type accessControllerEvent struct {
	MajorEventType   *int    `json:"majorEventType"`
	SubEventType     *int    `json:"subEventType"`
	CardNo           *string `json:"cardNo"`
	Name             *string `json:"name"`
	EmployeeNoString *string `json:"employeeNoString"`
	CardReaderNo     *int    `json:"cardReaderNo"`
	DoorNo           *int    `json:"doorNo"`
}

type eventDocument struct {
	EventType             string                 `json:"eventType"`
	EventDescription      *string                `json:"eventDescription"`
	DateTime              *string                `json:"dateTime"`
	IPAddress             *string                `json:"ipAddress"`
	AccessControllerEvent *accessControllerEvent `json:"AccessControllerEvent"`
}

// Extract returns a projection only for the documented JSON AccessController
// event form. It makes no inference for other event types, formats, missing
// category codes, or malformed declared timestamps.
func Extract(dataFormat, payloadBase64 string) (Projection, bool) {
	if dataFormat != "jsonData" {
		return Projection{}, false
	}
	payload, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return Projection{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var document eventDocument
	if err := decoder.Decode(&document); err != nil {
		return Projection{}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Projection{}, false
	}
	access := document.AccessControllerEvent
	if document.EventType != "AccessControllerEvent" || access == nil || access.MajorEventType == nil || access.SubEventType == nil {
		return Projection{}, false
	}
	if *access.MajorEventType < 1 || *access.MajorEventType > 5 || *access.SubEventType < 0 {
		return Projection{}, false
	}

	var occurredAt *time.Time
	if document.DateTime != nil {
		parsed, err := time.Parse(time.RFC3339, *document.DateTime)
		if err != nil {
			return Projection{}, false
		}
		occurredAt = &parsed
	}
	return Projection{
		MajorEventType:   *access.MajorEventType,
		SubEventType:     *access.SubEventType,
		EventDescription: document.EventDescription,
		OccurredAt:       occurredAt,
		EmployeeNumber:   access.EmployeeNoString,
		EmployeeName:     access.Name,
		CardNumber:       access.CardNo,
		CardReaderNumber: access.CardReaderNo,
		DoorNumber:       access.DoorNo,
		SourceIPAddress:  document.IPAddress,
	}, true
}
