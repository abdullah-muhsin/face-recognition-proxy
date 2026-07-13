# Attendance Mode And Verification TTS

## Covered Endpoints

- `GET /ISAPI/AccessControl/Configuration/attendanceMode/capabilities?format=json`
- `GET /ISAPI/AccessControl/Configuration/attendanceMode?format=json`
- `GET /ISAPI/AccessControl/Verification/ttsText/capabilities?format=json`
- `GET /ISAPI/AccessControl/Verification/ttsText?format=json`
- `GET /ISAPI/AccessControl/Verification/ttsText/holidayPlan/capabilities?format=json`
- `GET /ISAPI/AccessControl/Verification/ttsText/holidayPlan/1?format=json`
- `POST /ISAPI/AccessControl/Verification/ttsText/searchHolidayPlan?format=json`

## Attendance Mode

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Configuration/attendanceMode?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "AttendanceMode": {
    "mode": "auto",
    "reqAttendanceStatus": true
  }
}
```

### Capability Response

```json
{
  "AttendanceMode": {
    "mode": {
      "@opt": [
        "disable",
        "manual",
        "auto",
        "manualAndAuto"
      ]
    },
    "reqAttendanceStatus": {
      "@opt": [
        true,
        false
      ]
    }
  }
}
```

## Verification TTS

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Verification/ttsText?format=json"
```

### Observed Response

```json
{
  "TTSText": {
    "enable": false,
    "prefix": "none",
    "Success": [],
    "Failure": []
  }
}
```

### Capability Response

```json
{
  "TTSTextCap": {
    "enable": [
      true,
      false
    ],
    "prefix": [
      "name",
      "none"
    ],
    "Success": {
      "maxSize": 4,
      "TimeSegment": {
        "beginTime": "00:00:00",
        "endTime": "23:59:59",
        "validUnit": "second"
      },
      "text": {
        "@min": 1,
        "@max": 64
      }
    },
    "Failure": {
      "maxSize": 4,
      "TimeSegment": {
        "beginTime": "00:00:00",
        "endTime": "23:59:59",
        "validUnit": "second"
      },
      "text": {
        "@min": 1,
        "@max": 64
      }
    }
  }
}
```

## Holiday TTS Plan

### Indexed Plan Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Verification/ttsText/holidayPlan/1?format=json"
```

### Observed Response

```json
{
  "enable": false,
  "prefix": "none",
  "beginDate": "2000-01-01",
  "endDate": "2037-12-31",
  "Success": [],
  "Failure": []
}
```

### Search Request

This search endpoint expects a rootless JSON body. Wrapping the body in `SearchCond` or `TTSTextHolidayPlanSearchCond` returned `MessageParametersLack` for `searchID`.

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/Verification/ttsText/searchHolidayPlan?format=json" \
  --data '{
    "searchID": "doc-tts-holiday",
    "searchResultPosition": 0,
    "maxResults": 10
  }'
```

### Search Response

```json
{
  "responseStatus": "NO MATCH",
  "numOfMatches": 0,
  "totalMatches": 0
}
```

## Integration Notes

- The working TTS path is `/ISAPI/AccessControl/Verification/ttsText`; `/ISAPI/AccessControl/TTSText` returned `notSupport`.
- TTS is currently disabled.
- Success and failure text schedules support up to four time segments each.
