package accesscontrol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRetainedEventSearchUsesTerminalTimeAndDocumentedAllSelector(t *testing.T) {
	terminalNow, err := time.Parse(time.RFC3339, "2026-09-03T00:20:34+08:00")
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewRetainedEventSearchRequest("8f6076c0-a8fd-4e0d-8e40-a267267de1e4", 30, terminalNow)
	if err != nil {
		t.Fatalf("NewRetainedEventSearchRequest() error = %v", err)
	}
	var document retainedEventSearchEnvelope
	if err := json.Unmarshal(source, &document); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	condition := document.AcsEventCond
	if condition.SearchResultPosition != 30 || condition.MaxResults != RetainedEventPageSize || condition.Major != 0 || condition.Minor != 0 || !condition.TimeReverseOrder {
		t.Fatalf("condition = %#v", condition)
	}
	if condition.StartTime != "2000-01-01T00:00:00+08:00" || condition.EndTime != "2026-09-03T00:20:34+08:00" {
		t.Fatalf("search bounds = %q through %q", condition.StartTime, condition.EndTime)
	}
}

func TestParseTerminalTimeRequiresTheDocumentedISAPIRoot(t *testing.T) {
	source := []byte("<Time version=\"2.0\" xmlns=\"http://www.isapi.org/ver20/XMLSchema\"><localTime>2026-09-03T00:20:34+08:00</localTime><IANA>Asia/Shanghai</IANA></Time>\n")
	parsed, err := ParseTerminalTime(source)
	if err != nil {
		t.Fatalf("ParseTerminalTime() error = %v", err)
	}
	if got := parsed.Format(time.RFC3339); got != "2026-09-03T00:20:34+08:00" {
		t.Fatalf("localTime = %q", got)
	}
	if _, err := ParseTerminalTime([]byte(`<time><localTime>2026-09-03T00:20:34+08:00</localTime></time>`)); err == nil {
		t.Fatal("ParseTerminalTime() accepted an undocumented root")
	}
}

func TestParseRetainedEventPagePreservesEveryInfoRecord(t *testing.T) {
	source := []byte(`{"AcsEvent":{"searchID":"gateway-search","responseStatusStrg":"OK","numOfMatches":1,"totalMatches":1,"InfoList":[ { "major":5, "minor":75, "time":"2026-09-02T13:32:56+08:00", "serialNo":179, "name":"test1", "FaceRect":{"height":0.322,"width":0.183,"x":0.471,"y":0.492} } ]}}`)
	page, err := ParseRetainedEventPage(source)
	if err != nil {
		t.Fatalf("ParseRetainedEventPage() error = %v", err)
	}
	if page.ResponseStatus != "OK" || page.NumOfMatches != 1 || page.TotalMatches != 1 || len(page.InfoList) != 1 {
		t.Fatalf("page = %#v", page)
	}
	if !strings.Contains(string(page.InfoList[0]), `"major":5`) || !strings.Contains(string(page.InfoList[0]), `"FaceRect"`) {
		t.Fatalf("raw InfoList record was not retained: %s", page.InfoList[0])
	}
	projection, err := ExtractRetainedEvent(page.InfoList[0])
	if err != nil {
		t.Fatalf("ExtractRetainedEvent() error = %v", err)
	}
	if projection.MajorEventType != 5 || projection.SubEventType != 75 || projection.EventSerialNumber == nil || *projection.EventSerialNumber != 179 {
		t.Fatalf("projection = %#v", projection)
	}
	if projection.FaceRect == nil || projection.FaceRect.Height == nil || *projection.FaceRect.Height != "0.322" {
		t.Fatalf("face rectangle = %#v", projection.FaceRect)
	}
}

func TestParseRetainedEventPageAcceptsOnlyCoherentNoMatch(t *testing.T) {
	page, err := ParseRetainedEventPage([]byte(`{"AcsEvent":{"searchID":"gateway-search","responseStatusStrg":"NO MATCH","numOfMatches":0,"totalMatches":0}}`))
	if err != nil {
		t.Fatalf("ParseRetainedEventPage() error = %v", err)
	}
	if page.ResponseStatus != "NO MATCH" || len(page.InfoList) != 0 {
		t.Fatalf("page = %#v", page)
	}
	if _, err := ParseRetainedEventPage([]byte(`{"AcsEvent":{"searchID":"gateway-search","responseStatusStrg":"NO MATCH","numOfMatches":1,"totalMatches":1,"InfoList":[{}]}}`)); err == nil {
		t.Fatal("ParseRetainedEventPage() accepted an incoherent NO MATCH response")
	}
}
