// Package accesscontrol defines the declared AccessControllerEvent read model.
// It is deliberately separate from PushSDK envelope validation: a source event
// remains valid and archived even when it is not this exact JSON event form.
package accesscontrol

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"time"
)

const (
	SchemaVersion  = 2
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

// Picture is an exact JPEG part declared by a terminal multipart event.
type Picture struct {
	ContentType string `json:"contentType"`
	FileName    string `json:"fileName"`
	ContentID   string `json:"contentId,omitempty"`
	DataBase64  string `json:"dataBase64"`
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
	payload, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return Projection{}, false
	}
	if dataFormat == "boundaryData" {
		payload, err = accessControllerEventPart(payload)
		if err != nil {
			return Projection{}, false
		}
	} else if dataFormat != "jsonData" {
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

// accessControllerEventPart accepts only the documented multipart wrapper used
// by the terminal: multipart/form-data with exactly one JSON form part named
// AccessControllerEvent. It does not scan arbitrary boundary bytes for JSON.
func accessControllerEventPart(payload []byte) ([]byte, error) {
	parts, err := multipartFormParts(payload)
	if err != nil {
		return nil, err
	}
	var eventPayload []byte
	for _, part := range parts {
		if part.name != "AccessControllerEvent" {
			continue
		}
		if part.mediaType != "application/json" || eventPayload != nil {
			return nil, io.ErrUnexpectedEOF
		}
		eventPayload = part.data
	}
	if eventPayload == nil {
		return nil, io.ErrUnexpectedEOF
	}
	return eventPayload, nil
}

// ExtractPicture returns only the explicitly declared JPEG Picture form part.
// It never attempts image detection, transcoding, or recovery from arbitrary
// boundary bytes.
func ExtractPicture(dataFormat, payloadBase64 string) (Picture, bool) {
	if dataFormat != "boundaryData" {
		return Picture{}, false
	}
	payload, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return Picture{}, false
	}
	parts, err := multipartFormParts(payload)
	if err != nil {
		return Picture{}, false
	}
	var picture *Picture
	for _, part := range parts {
		if part.name != "Picture" {
			continue
		}
		if part.mediaType != "image/jpeg" || part.fileName == "" || picture != nil {
			return Picture{}, false
		}
		picture = &Picture{
			ContentType: part.mediaType,
			FileName:    part.fileName,
			ContentID:   part.contentID,
			DataBase64:  base64.StdEncoding.EncodeToString(part.data),
		}
	}
	if picture == nil {
		return Picture{}, false
	}
	return *picture, true
}

type multipartFormPart struct {
	name      string
	fileName  string
	contentID string
	mediaType string
	data      []byte
}

func multipartFormParts(payload []byte) ([]multipartFormPart, error) {
	reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(payload)))
	header, err := reader.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	mediaType, parameters, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || parameters["boundary"] == "" {
		return nil, io.ErrUnexpectedEOF
	}
	multipartReader := multipart.NewReader(reader.R, parameters["boundary"])
	parts := []multipartFormPart{}
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		disposition, parameters, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if err != nil || disposition != "form-data" || parameters["name"] == "" {
			return nil, io.ErrUnexpectedEOF
		}
		mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return nil, io.ErrUnexpectedEOF
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		parts = append(parts, multipartFormPart{
			name:      parameters["name"],
			fileName:  parameters["filename"],
			contentID: part.Header.Get("Content-ID"),
			mediaType: mediaType,
			data:      data,
		})
	}
	return parts, nil
}
