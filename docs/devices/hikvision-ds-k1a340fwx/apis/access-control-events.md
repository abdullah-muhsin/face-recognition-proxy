# Access-Control Event Search

## Covered Endpoints

- `GET /ISAPI/AccessControl/AcsEvent/capabilities?format=json`
- `POST /ISAPI/AccessControl/AcsEvent?format=json`
- `GET /ISAPI/AccessControl/AcsEventTotalNum/capabilities?format=json`
- `POST /ISAPI/AccessControl/AcsEventTotalNum?format=json`

## Event Search Capabilities

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/AcsEvent/capabilities?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "AcsEvent": {
    "AcsEventCond": {
      "searchID": {
        "@min": 1,
        "@max": 64
      },
      "searchResultPosition": {
        "@min": 0,
        "@max": 300000
      },
      "maxResults": {
        "@min": 1,
        "@max": 30
      },
      "major": {
        "@opt": "0,1,2,3,5"
      },
      "minorAlarm": {
        "@opt": "1028,1029,1030,1031,1032,1033,1034,1035,1036"
      },
      "minorException": {
        "@opt": "39,1024,1031,1033,1034,1064,1065"
      },
      "minorOperation": {
        "@opt": "90,93,94,112,113,121,122,123,126,134,135,1024,1025,1026,1027,1028,1029,1030,1031,1034,1035,1036,1038,1049,1050,1071,1072,1073,1074,1075,9728,9729"
      },
      "minorEvent": {
        "@opt": "1,6,7,8,9,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,38,39,49,51,75,76,81,82,104,151,152,155,153,154"
      },
      "startTime": {
        "@min": 1,
        "@max": 32
      },
      "endTime": {
        "@min": 1,
        "@max": 32
      },
      "cardNo": {
        "@min": 0,
        "@max": 32
      },
      "name": {
        "@min": 0,
        "@max": 128
      },
      "beginSerialNo": {
        "@min": 1,
        "@max": 3000000000
      },
      "endSerialNo": {
        "@min": 1,
        "@max": 3000000000
      },
      "employeeNoString": {
        "@min": 0,
        "@max": 32
      },
      "timeReverseOrder": true
    },
    "InfoList": {
      "maxSize": 30,
      "attendanceStatus": {
        "@opt": "undefined,checkIn,checkOut,breakOut,breakIn,overtimeIn,overtimeOut"
      },
      "statusValue": {
        "@min": 0,
        "@max": 255
      }
    }
  }
}
```

The response above is shortened to the most relevant constraints. The live response also lists field limits for card reader number, door number, verify number, serial number, user type, current verify mode, and other event columns.

## Event Search

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/AcsEvent?format=json" \
  --data '{
    "AcsEventCond": {
      "searchID": "doc-event-1",
      "searchResultPosition": 0,
      "maxResults": 1,
      "major": 0,
      "minor": 0,
      "startTime": "2000-01-01T00:00:00+03:00",
      "endTime": "2037-12-31T23:59:59+03:00",
      "timeReverseOrder": true
    }
  }'
```

### Observed Response

Status: `200 OK`

```json
{
  "AcsEvent": {
    "searchID": "doc-event-1",
    "responseStatusStrg": "MORE",
    "numOfMatches": 1,
    "totalMatches": 68,
    "InfoList": [
      {
        "major": 2,
        "minor": 1024,
        "time": "2026-07-06T00:33:53+03:00",
        "netUser": "",
        "remoteHostAddr": "0.0.0.0",
        "cardNo": "",
        "cardType": 0,
        "name": "",
        "reportChannel": 0,
        "cardReaderKind": 0,
        "cardReaderNo": 0,
        "doorNo": 0,
        "verifyNo": 0,
        "alarmInNo": 0,
        "alarmOutNo": 0,
        "caseSensorNo": 0,
        "RS485No": 0,
        "multiCardGroupNo": 0,
        "deviceNo": 0,
        "employeeNoString": "",
        "InternetAccess": 0,
        "type": 0,
        "MACAddr": "",
        "swipeCardType": 0,
        "serialNo": 1,
        "userType": "normal",
        "currentVerifyMode": "faceOrFpOrCardOrPw",
        "attendanceStatus": "undefined",
        "statusValue": 0
      }
    ]
  }
}
```

## Event With Picture URL

A later event contained a face/verification picture reference. Sensitive fields are redacted here while preserving the response shape.

```json
{
  "major": 5,
  "minor": 75,
  "time": "2026-07-06T00:38:32+03:00",
  "cardType": 1,
  "name": "[redacted]",
  "cardReaderNo": 1,
  "doorNo": 1,
  "employeeNoString": "[redacted]",
  "userType": "normal",
  "currentVerifyMode": "faceOrFpOrCardOrPw",
  "attendanceStatus": "undefined",
  "statusValue": 0,
  "pictureURL": "http://192.168.1.200/LOCALS/pic/acsLinkCap/[redacted].jpeg@[redacted]",
  "picturesNumber": 1
}
```

## Event Total Count

### Request

Use the `AcsEventTotalNumCond` root. Reusing the search root returns `MessageParametersLack`.

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/AcsEventTotalNum?format=json" \
  --data '{
    "AcsEventTotalNumCond": {
      "major": 0,
      "minor": 0,
      "startTime": "2000-01-01T00:00:00+03:00",
      "endTime": "2037-12-31T23:59:59+03:00"
    }
  }'
```

### Observed Response

Status: `200 OK`

```json
{
  "AcsEventTotalNum": {
    "totalNum": 68
  }
}
```

## Integration Notes

- `maxResults` supports `1..30`.
- Use `searchResultPosition` for pagination.
- `major: 0` and `minor: 0` returned all event classes in testing.
- The device can return historical records via alert stream as well as event search.
- Picture URLs are separate media retrieval calls. See [picture URL media retrieval](media-picture-url.md).

