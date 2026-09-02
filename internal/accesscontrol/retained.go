package accesscontrol

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	RetainedEventSearchURL   = "/ISAPI/AccessControl/AcsEvent?format=json"
	RetainedEventPageSize    = 30
	retainedHistoryStartYear = 2000
)

// RetainedEventSearchRequest is the exact documented AcsEvent condition used
// to reconcile the terminal's retained access-event archive. The terminal
// accepts major/minor zero as its explicit all-category selector.
type RetainedEventSearchRequest struct {
	SearchID             string `json:"searchID"`
	SearchResultPosition int    `json:"searchResultPosition"`
	MaxResults           int    `json:"maxResults"`
	Major                int    `json:"major"`
	Minor                int    `json:"minor"`
	StartTime            string `json:"startTime"`
	EndTime              string `json:"endTime"`
	TimeReverseOrder     bool   `json:"timeReverseOrder"`
}

type retainedEventSearchEnvelope struct {
	AcsEventCond RetainedEventSearchRequest `json:"AcsEventCond"`
}

// NewRetainedEventSearchRequest builds an all-category request bounded by the
// terminal's own reported local clock. DS-K1T342MFWX-E1 accepts 2000-01-01 as
// the earliest supported search time; the search is never given a gateway- or
// browser-derived end time.
func NewRetainedEventSearchRequest(searchID string, position int, terminalNow time.Time) ([]byte, error) {
	if searchID == "" || len(searchID) > 64 {
		return nil, errors.New("retained event search ID must contain 1..64 bytes")
	}
	if position < 0 || position > 150000 {
		return nil, errors.New("retained event search position must be 0..150000")
	}
	if terminalNow.IsZero() {
		return nil, errors.New("terminal time is required")
	}
	start := time.Date(retainedHistoryStartYear, time.January, 1, 0, 0, 0, 0, terminalNow.Location())
	if !terminalNow.After(start) {
		return nil, errors.New("terminal time precedes the supported retained-event range")
	}
	return json.Marshal(retainedEventSearchEnvelope{AcsEventCond: RetainedEventSearchRequest{
		SearchID:             searchID,
		SearchResultPosition: position,
		MaxResults:           RetainedEventPageSize,
		Major:                0,
		Minor:                0,
		StartTime:            start.Format(time.RFC3339),
		EndTime:              terminalNow.Format(time.RFC3339),
		TimeReverseOrder:     true,
	}})
}

// ParseTerminalTime accepts only the documented System/time XML document. A
// terminal may return XML for this endpoint even when its HTTP response header
// announces application/json; the retained source bytes remain in the command
// audit and this parser makes no content-type substitution.
func ParseTerminalTime(source []byte) (time.Time, error) {
	var document struct {
		XMLName   xml.Name `xml:"Time"`
		LocalTime string   `xml:"localTime"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(source))
	if err := decoder.Decode(&document); err != nil {
		return time.Time{}, fmt.Errorf("decode System/time XML: %w", err)
	}
	if document.XMLName.Space != "http://www.isapi.org/ver20/XMLSchema" || document.XMLName.Local != "Time" {
		return time.Time{}, errors.New("System/time XML root is not the documented Time element")
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("read after System/time XML: %w", err)
		}
		characters, isCharacters := token.(xml.CharData)
		if !isCharacters || strings.TrimSpace(string(characters)) != "" {
			return time.Time{}, errors.New("System/time XML contains trailing content")
		}
	}
	if document.LocalTime == "" {
		return time.Time{}, errors.New("System/time XML omits localTime")
	}
	parsed, err := time.Parse(time.RFC3339, document.LocalTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse terminal localTime: %w", err)
	}
	return parsed, nil
}

// RetainedEventPage contains the raw InfoList records exactly as the terminal
// supplied them. Projection is performed per record so each independently
// retained record has an auditable source byte sequence.
type RetainedEventPage struct {
	SearchID       string
	ResponseStatus string
	NumOfMatches   int
	TotalMatches   int
	InfoList       []json.RawMessage
}

type retainedEventPageDocument struct {
	AcsEvent *struct {
		SearchID       *string           `json:"searchID"`
		ResponseStatus *string           `json:"responseStatusStrg"`
		NumOfMatches   *int              `json:"numOfMatches"`
		TotalMatches   *int              `json:"totalMatches"`
		InfoList       []json.RawMessage `json:"InfoList"`
	} `json:"AcsEvent"`
}

// ParseRetainedEventPage accepts the documented AcsEvent result form and
// rejects a response whose pagination state cannot be represented exactly.
func ParseRetainedEventPage(source []byte) (RetainedEventPage, error) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	var document retainedEventPageDocument
	if err := decoder.Decode(&document); err != nil {
		return RetainedEventPage{}, fmt.Errorf("decode AcsEvent result: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return RetainedEventPage{}, err
	}
	if document.AcsEvent == nil || document.AcsEvent.SearchID == nil || document.AcsEvent.ResponseStatus == nil || document.AcsEvent.NumOfMatches == nil || document.AcsEvent.TotalMatches == nil {
		return RetainedEventPage{}, errors.New("AcsEvent result omits a required pagination field")
	}
	page := RetainedEventPage{
		SearchID:       *document.AcsEvent.SearchID,
		ResponseStatus: *document.AcsEvent.ResponseStatus,
		NumOfMatches:   *document.AcsEvent.NumOfMatches,
		TotalMatches:   *document.AcsEvent.TotalMatches,
		InfoList:       document.AcsEvent.InfoList,
	}
	if page.SearchID == "" || len(page.SearchID) > 64 {
		return RetainedEventPage{}, errors.New("AcsEvent result has an invalid searchID")
	}
	if page.NumOfMatches < 0 || page.NumOfMatches > RetainedEventPageSize || page.TotalMatches < 0 || page.TotalMatches > 150000 {
		return RetainedEventPage{}, errors.New("AcsEvent result has an out-of-range pagination count")
	}
	switch page.ResponseStatus {
	case "MORE", "OK":
		if page.NumOfMatches != len(page.InfoList) {
			return RetainedEventPage{}, errors.New("AcsEvent result numOfMatches does not equal InfoList length")
		}
	case "NO MATCH":
		if page.NumOfMatches != 0 || page.TotalMatches != 0 || len(page.InfoList) != 0 {
			return RetainedEventPage{}, errors.New("AcsEvent NO MATCH result has records or a non-zero count")
		}
	default:
		return RetainedEventPage{}, fmt.Errorf("AcsEvent result has unsupported responseStatusStrg %q", page.ResponseStatus)
	}
	return page, nil
}

// ExtractRetainedEvent projects one documented AcsEvent InfoList record. It
// does not manufacture PushSDK envelope fields that the retained ISAPI record
// did not include.
func ExtractRetainedEvent(source json.RawMessage) (Projection, error) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	var record struct {
		Major             *int      `json:"major"`
		Minor             *int      `json:"minor"`
		Time              *string   `json:"time"`
		CardNo            *string   `json:"cardNo"`
		Name              *string   `json:"name"`
		EmployeeNoString  *string   `json:"employeeNoString"`
		CardReaderNo      *int      `json:"cardReaderNo"`
		DoorNo            *int      `json:"doorNo"`
		SerialNo          *int64    `json:"serialNo"`
		UserType          *string   `json:"userType"`
		CurrentVerifyMode *string   `json:"currentVerifyMode"`
		Mask              *string   `json:"mask"`
		FaceRect          *faceRect `json:"FaceRect"`
	}
	if err := decoder.Decode(&record); err != nil {
		return Projection{}, fmt.Errorf("decode retained AcsEvent record: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Projection{}, err
	}
	if record.Major == nil || record.Minor == nil || record.Time == nil {
		return Projection{}, errors.New("retained AcsEvent record omits major, minor, or time")
	}
	if *record.Major < 1 || *record.Major > 5 || *record.Minor < 0 {
		return Projection{}, errors.New("retained AcsEvent record has invalid major or minor code")
	}
	occurredAt, err := time.Parse(time.RFC3339, *record.Time)
	if err != nil {
		return Projection{}, fmt.Errorf("parse retained AcsEvent time: %w", err)
	}
	return Projection{
		MajorEventType:    *record.Major,
		SubEventType:      *record.Minor,
		OccurredAt:        &occurredAt,
		EmployeeNumber:    record.EmployeeNoString,
		EmployeeName:      record.Name,
		CardNumber:        record.CardNo,
		CardReaderNumber:  record.CardReaderNo,
		DoorNumber:        record.DoorNo,
		EventSerialNumber: record.SerialNo,
		UserType:          record.UserType,
		CurrentVerifyMode: record.CurrentVerifyMode,
		Mask:              record.Mask,
		FaceRect:          projectFaceRect(record.FaceRect),
	}, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err != nil {
			return fmt.Errorf("read after JSON document: %w", err)
		}
		return errors.New("JSON document contains trailing value")
	}
	return nil
}
