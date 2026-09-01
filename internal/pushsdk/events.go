package pushsdk

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itplus/pushsdk-gateway/internal/store"
)

var (
	vendorEventID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)
	rfc3339Second = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`)
	boundaryValue = regexp.MustCompile(`^[0-9A-Za-z'()+_,./:=?-]{1,70}$`)
)

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
	UUID       string
	Format     string
	Attendance *store.NewAttendanceRecord
	Ignored    bool
	Reason     string
}

type eventEnvelope struct {
	EventNum  int         `json:"eventNum"`
	EventList []eventItem `json:"eventList"`
}
type eventItem struct {
	UUID       string `json:"UUID"`
	DataFormat string `json:"dataFormat"`
	Data       string `json:"data"`
}

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
	if envelope.EventNum < 0 || envelope.EventNum > 20 {
		return nil, badRequest("eventNum must be between 0 and 20")
	}
	if len(envelope.EventList) != envelope.EventNum {
		return nil, badRequest("eventNum must equal eventList length")
	}
	seen := make(map[string]struct{}, len(envelope.EventList))
	parsed := make([]ParsedEvent, 0, len(envelope.EventList))
	for _, item := range envelope.EventList {
		if !vendorEventID.MatchString(item.UUID) {
			return nil, badRequest("event UUID is invalid")
		}
		if _, exists := seen[item.UUID]; exists {
			return nil, badRequest("eventList contains a duplicate UUID")
		}
		seen[item.UUID] = struct{}{}
		if item.DataFormat != "jsonData" && item.DataFormat != "xmlData" && item.DataFormat != "boundaryData" && item.DataFormat != "noData" {
			return nil, badRequest("event %s has an unsupported dataFormat", item.UUID)
		}
		payload, err := base64.StdEncoding.DecodeString(item.Data)
		if err != nil {
			return nil, badRequest("event %s data is not base64: %v", item.UUID, err)
		}
		var event ParsedEvent
		switch item.DataFormat {
		case "jsonData":
			event, err = parseJSONEvent(terminalSerial, item.UUID, payload, "jsonData")
		case "xmlData":
			event, err = parseXMLEvent(terminalSerial, item.UUID, payload, "xmlData")
		case "boundaryData":
			event, err = parseBoundaryEvent(terminalSerial, item.UUID, payload)
		case "noData":
			if len(payload) != 0 {
				return nil, unprocessable("event %s noData payload must be empty", item.UUID)
			}
			event = ParsedEvent{UUID: item.UUID, Format: "noData", Ignored: true, Reason: "no_data"}
		}
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, event)
	}
	return parsed, nil
}

type jsonAlert struct {
	EventType             string     `json:"eventType"`
	EventState            string     `json:"eventState"`
	DateTime              string     `json:"dateTime"`
	AccessControllerEvent jsonAccess `json:"AccessControllerEvent"`
}
type jsonAccess struct {
	EmployeeNoString  string  `json:"employeeNoString"`
	Name              *string `json:"name"`
	CurrentVerifyMode string  `json:"currentVerifyMode"`
	AttendanceStatus  *string `json:"attendanceStatus"`
	StatusValue       *int    `json:"statusValue"`
}

func parseJSONEvent(terminal, uuid string, payload []byte, format string) (ParsedEvent, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var alert jsonAlert
	if err := decoder.Decode(&alert); err != nil {
		return ParsedEvent{}, badRequest("event %s %s payload is not valid JSON: %v", uuid, format, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ParsedEvent{}, badRequest("event %s %s payload contains a trailing JSON value", uuid, format)
	}
	return translateAlert(terminal, uuid, format, alert.EventType, alert.EventState, alert.DateTime, alert.AccessControllerEvent.EmployeeNoString,
		alert.AccessControllerEvent.Name, alert.AccessControllerEvent.CurrentVerifyMode, alert.AccessControllerEvent.AttendanceStatus, alert.AccessControllerEvent.StatusValue)
}

type xmlAlert struct {
	XMLName               xml.Name  `xml:"EventNotificationAlert"`
	EventType             string    `xml:"eventType"`
	EventState            string    `xml:"eventState"`
	DateTime              string    `xml:"dateTime"`
	AccessControllerEvent xmlAccess `xml:"AccessControllerEvent"`
}
type xmlAccess struct {
	EmployeeNoString  string  `xml:"employeeNoString"`
	Name              *string `xml:"name"`
	CurrentVerifyMode string  `xml:"currentVerifyMode"`
	AttendanceStatus  *string `xml:"attendanceStatus"`
	StatusValue       *int    `xml:"statusValue"`
}

func parseXMLEvent(terminal, uuid string, payload []byte, format string) (ParsedEvent, error) {
	if !utf8.Valid(payload) {
		return ParsedEvent{}, badRequest("event %s %s payload is not valid UTF-8", uuid, format)
	}
	var alert xmlAlert
	if err := xml.Unmarshal(payload, &alert); err != nil {
		return ParsedEvent{}, badRequest("event %s %s payload is not valid XML: %v", uuid, format, err)
	}
	if alert.XMLName.Local != "EventNotificationAlert" {
		return ParsedEvent{}, unprocessable("event %s XML root must be EventNotificationAlert", uuid)
	}
	return translateAlert(terminal, uuid, format, alert.EventType, alert.EventState, alert.DateTime, alert.AccessControllerEvent.EmployeeNoString,
		alert.AccessControllerEvent.Name, alert.AccessControllerEvent.CurrentVerifyMode, alert.AccessControllerEvent.AttendanceStatus, alert.AccessControllerEvent.StatusValue)
}

func translateAlert(terminal, uuid, format, eventType, eventState, dateTime, employee string, name *string, verify string, attendance *string, statusValue *int) (ParsedEvent, error) {
	if eventType != "AccessControllerEvent" || eventState != "active" {
		return ParsedEvent{UUID: uuid, Format: format, Ignored: true, Reason: "not_active_access_controller_event"}, nil
	}
	if employee == "" || strings.TrimSpace(employee) == "" {
		return ParsedEvent{}, unprocessable("event %s has no employeeNoString", uuid)
	}
	if verify == "" || strings.TrimSpace(verify) == "" {
		return ParsedEvent{}, unprocessable("event %s has no currentVerifyMode", uuid)
	}
	if len(employee) > 80 || len(verify) > 80 {
		return ParsedEvent{}, unprocessable("event %s exceeds a field length limit", uuid)
	}
	if name != nil && len(*name) > 160 {
		return ParsedEvent{}, unprocessable("event %s name exceeds 160 characters", uuid)
	}
	if attendance != nil && len(*attendance) > 80 {
		return ParsedEvent{}, unprocessable("event %s attendanceStatus exceeds 80 characters", uuid)
	}
	occurredAt, err := parseTimestamp(dateTime)
	if err != nil {
		return ParsedEvent{}, unprocessable("event %s dateTime: %v", uuid, err)
	}
	return ParsedEvent{UUID: uuid, Format: format, Attendance: &store.NewAttendanceRecord{
		TerminalSerialNumber: terminal, VendorEventID: uuid, OccurredAt: occurredAt, EmployeeNumber: employee, EmployeeName: name,
		VerificationMethod: verify, AttendanceStatus: attendance, StatusValue: statusValue, SourceFormat: format,
	}}, nil
}

func parseTimestamp(value string) (time.Time, error) {
	if !rfc3339Second.MatchString(value) {
		return time.Time{}, fmt.Errorf("must be RFC 3339 with second precision")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("is invalid")
	}
	return parsed.UTC(), nil
}

type multipartPart struct {
	ContentType string
	Body        []byte
}

// parseBoundaryEvent implements the documented direct multipart form only:
// outer MIME headers, CRLF CRLF, then a complete multipart/form-data body.
// It intentionally does not accept serialized HTTP messages, headerless parts,
// or inferred media types from payload bytes.
func parseBoundaryEvent(terminal, uuid string, payload []byte) (ParsedEvent, error) {
	parts, err := parseDirectMultipart(payload)
	if err != nil {
		return ParsedEvent{}, badRequest("event %s boundaryData: %v", uuid, err)
	}
	var metadata *multipartPart
	jpegCount := 0
	for index := range parts {
		mediaType, _, err := mime.ParseMediaType(parts[index].ContentType)
		if err != nil {
			return ParsedEvent{}, badRequest("event %s boundaryData part has invalid Content-Type", uuid)
		}
		switch mediaType {
		case "application/json", "application/xml":
			if metadata != nil {
				return ParsedEvent{}, unprocessable("event %s boundaryData has more than one metadata part", uuid)
			}
			metadata = &parts[index]
		case "image/jpeg":
			jpegCount++
			if jpegCount > 1 {
				return ParsedEvent{}, unprocessable("event %s boundaryData has more than one JPEG", uuid)
			}
			if len(parts[index].Body) < 4 || len(parts[index].Body) > 5_242_880 || !bytes.HasPrefix(parts[index].Body, []byte{0xff, 0xd8, 0xff}) || !bytes.HasSuffix(parts[index].Body, []byte{0xff, 0xd9}) {
				return ParsedEvent{}, unprocessable("event %s JPEG is invalid", uuid)
			}
		default:
			return ParsedEvent{}, unprocessable("event %s boundaryData has unsupported part media type %q", uuid, mediaType)
		}
	}
	if metadata == nil {
		return ParsedEvent{}, unprocessable("event %s boundaryData has no JSON or XML metadata part", uuid)
	}
	mediaType, _, _ := mime.ParseMediaType(metadata.ContentType)
	if mediaType == "application/json" {
		return parseJSONEvent(terminal, uuid, metadata.Body, "boundaryData")
	}
	return parseXMLEvent(terminal, uuid, metadata.Body, "boundaryData")
}

func parseDirectMultipart(payload []byte) ([]multipartPart, error) {
	headerEnd := bytes.Index(payload, []byte("\r\n\r\n"))
	if headerEnd < 1 {
		return nil, fmt.Errorf("outer headers are missing")
	}
	headers, err := parseHeaders(string(payload[:headerEnd]))
	if err != nil {
		return nil, err
	}
	contentType, found := headers["content-type"]
	if !found {
		return nil, fmt.Errorf("outer Content-Type is missing")
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return nil, fmt.Errorf("outer Content-Type must be multipart/form-data")
	}
	boundary := params["boundary"]
	if !boundaryValue.MatchString(boundary) {
		return nil, fmt.Errorf("outer multipart boundary is invalid")
	}
	body := payload[headerEnd+4:]
	if length, found := headers["content-length"]; found {
		declared, err := strconv.Atoi(length)
		if err != nil || declared < 0 || declared != len(body) {
			return nil, fmt.Errorf("outer Content-Length does not match payload")
		}
	}
	delimiter := []byte("--" + boundary)
	if !bytes.HasPrefix(body, delimiter) {
		return nil, fmt.Errorf("body must begin with the multipart delimiter")
	}
	position := 0
	parts := []multipartPart{}
	for {
		if !bytes.HasPrefix(body[position:], delimiter) {
			return nil, fmt.Errorf("invalid multipart delimiter")
		}
		position += len(delimiter)
		if bytes.HasPrefix(body[position:], []byte("--")) {
			position += 2
			if position != len(body) && !bytes.Equal(body[position:], []byte("\r\n")) {
				return nil, fmt.Errorf("bytes after terminal multipart delimiter")
			}
			if len(parts) == 0 {
				return nil, fmt.Errorf("multipart has no parts")
			}
			return parts, nil
		}
		if !bytes.HasPrefix(body[position:], []byte("\r\n")) {
			return nil, fmt.Errorf("delimiter must be followed by CRLF")
		}
		position += 2
		headerEnd := bytes.Index(body[position:], []byte("\r\n\r\n"))
		if headerEnd < 1 {
			return nil, fmt.Errorf("part headers are missing")
		}
		partHeaders, err := parseHeaders(string(body[position : position+headerEnd]))
		if err != nil {
			return nil, err
		}
		partType, found := partHeaders["content-type"]
		if !found {
			return nil, fmt.Errorf("part Content-Type is missing")
		}
		position += headerEnd + 4
		next := bytes.Index(body[position:], append([]byte("\r\n"), delimiter...))
		if next < 0 {
			return nil, fmt.Errorf("part has no following delimiter")
		}
		partBody := body[position : position+next]
		if declared, found := partHeaders["content-length"]; found {
			length, err := strconv.Atoi(declared)
			if err != nil || length < 0 || length != len(partBody) {
				return nil, fmt.Errorf("part Content-Length does not match payload")
			}
		}
		parts = append(parts, multipartPart{ContentType: partType, Body: partBody})
		position += next + 2
	}
}

func parseHeaders(raw string) (map[string]string, error) {
	headers := make(map[string]string)
	for _, line := range strings.Split(raw, "\r\n") {
		name, value, found := strings.Cut(line, ":")
		if !found || name == "" || value == "" {
			return nil, fmt.Errorf("invalid header line")
		}
		name = strings.ToLower(name)
		if _, duplicate := headers[name]; duplicate {
			return nil, fmt.Errorf("duplicate header %q", name)
		}
		headers[name] = strings.TrimPrefix(value, " ")
	}
	return headers, nil
}
