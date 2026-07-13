# Write Operation Behavior

## Purpose

This file records non-read-only method checks performed to understand create/update/delete API shapes. It is not a recommendation to run these calls casually in production.

Use a current backup/export strategy before executing writes against a real attendance device.

## User Records

### Create Or Record

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/UserInfo/Record?format=json" \
  --data '{}'
```

Observed validation response:

```json
{
  "statusCode": 6,
  "statusString": "Invalid Content",
  "subStatusCode": "MessageParametersLack",
  "errorCode": 1610612761,
  "errorMsg": "UserInfo"
}
```

Interpretation: the endpoint exists and expects a `UserInfo` JSON root for `POST`.

### Modify Or Setup

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X PUT \
  "$ISAPI_BASE/ISAPI/AccessControl/UserInfo/Modify?format=json" \
  --data '{}'
```

Observed validation response:

```json
{
  "statusCode": 6,
  "statusString": "Invalid Content",
  "subStatusCode": "MessageParametersLack",
  "errorCode": 1610612761,
  "errorMsg": "UserInfo"
}
```

The same validation behavior was observed for `PUT /ISAPI/AccessControl/UserInfo/SetUp?format=json`.

### Delete

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X PUT \
  "$ISAPI_BASE/ISAPI/AccessControl/UserInfo/Delete?format=json" \
  --data '{}'
```

Observed validation response:

```json
{
  "statusCode": 6,
  "statusString": "Invalid Content",
  "subStatusCode": "MessageParametersLack",
  "errorCode": 1610612761,
  "errorMsg": "UserInfoDelCond"
}
```

Interpretation: the user delete endpoint uses `PUT`, not HTTP `DELETE`, and expects `UserInfoDelCond`.

## Card Records

### Create Or Record

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/CardInfo/Record?format=json" \
  --data '{}'
```

Observed validation response:

```json
{
  "statusCode": 6,
  "statusString": "Invalid Content",
  "subStatusCode": "MessageParametersLack",
  "errorCode": 1610612761,
  "errorMsg": "CardInfo"
}
```

### Modify Or Setup

`PUT /ISAPI/AccessControl/CardInfo/Modify?format=json` and `PUT /ISAPI/AccessControl/CardInfo/SetUp?format=json` both expect a `CardInfo` root.

### Delete

```json
{
  "statusCode": 6,
  "statusString": "Invalid Content",
  "subStatusCode": "MessageParametersLack",
  "errorCode": 1610612761,
  "errorMsg": "CardInfoDelCond"
}
```

Interpretation: the card delete endpoint uses `PUT /ISAPI/AccessControl/CardInfo/Delete?format=json` and expects `CardInfoDelCond`.

## Fingerprint Configuration And Delete

### Delete Capability

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/FingerPrint/Delete/capabilities?format=json"
```

```json
{
  "FingerPrintDelete": {
    "mode": {
      "@opt": "byEmployeeNo,byCardReader"
    },
    "EmployeeNoDetail": {
      "employeeNo": { "@min": 1, "@max": 32 },
      "enableCardReader": { "@min": 1, "@max": 2 },
      "fingerPrintID": { "@min": 1, "@max": 10 }
    },
    "CardReaderDetail": {
      "cardReaderNo": { "@min": 1, "@max": 2 },
      "clearAllCard": "true,false",
      "employeeNo": { "@min": 1, "@max": 32 }
    }
  }
}
```

### Delete Validation

`PUT /ISAPI/AccessControl/FingerPrint/Delete?format=json` with `{}` returned `MessageParametersLack` for `FingerPrintDelete`. `POST` and HTTP `DELETE` returned `methodNotAllowed`.

### Fingerprint Config Capability

```json
{
  "FingerPrintCfg": {
    "employeeNo": { "@min": 1, "@max": 32 },
    "enableCardReader": { "@min": 1, "@max": 1 },
    "fingerPrintID": { "@min": 1, "@max": 10 },
    "fingerType": { "@opt": "normalFP,hijackFP,patrolFP,superFP" },
    "leaderFP": { "@min": 1, "@max": 1 },
    "StatusList": {
      "id": { "@min": 1, "@max": 2 },
      "cardReaderRecvStatus": { "@opt": "0,1,2,3,4,5,6,7,8,10" },
      "errorMsg": { "@min": 0, "@max": 32 }
    },
    "totalStatus": { "@opt": "0,1" },
    "isSupportSetUp": true
  }
}
```

## Face And Fingerprint Capture

The capture endpoints are not readable with `GET`, but they exist as non-GET endpoints.

| Endpoint | Method tested | Observed result |
| --- | --- | --- |
| `/ISAPI/AccessControl/CaptureFaceData?format=json` | `GET` | `methodNotAllowed` |
| `/ISAPI/AccessControl/CaptureFaceData?format=json` | `POST {}` | `badXmlContent` |
| `/ISAPI/AccessControl/CaptureFingerPrint?format=json` | `GET` | `methodNotAllowed` |
| `/ISAPI/AccessControl/CaptureFingerPrint?format=json` | `POST {}` | `badXmlContent` |

Interpretation: these endpoints likely require a specific XML body or multipart workflow. No actual capture was started successfully during this pass.

## Local Attendance Objects

The web UI uses lower-case object paths:

| Object | Add path | Edit/delete path | Search path |
| --- | --- | --- | --- |
| Group | `POST /LocalAttendance/group?format=json` | `PUT` or `DELETE /LocalAttendance/group/{id}?format=json` | `POST /LocalAttendance/groupSearch?format=json` |
| Shift | `POST /LocalAttendance/shift?format=json` | `PUT` or `DELETE /LocalAttendance/shift/{id}?format=json` | `POST /LocalAttendance/shiftSearch?format=json` |
| Holiday plan | `POST /LocalAttendance/holidayPlan?format=json` | `PUT` or `DELETE /LocalAttendance/holidayPlan/{id}?format=json` | `POST /LocalAttendance/holidayPlanSearch?format=json` |
| Holiday group | `POST /LocalAttendance/holidayGroup?format=json` | `PUT` or `DELETE /LocalAttendance/holidayGroup/{id}?format=json` | `POST /LocalAttendance/holidayGroupSearch?format=json` |
| Week plan | `POST /LocalAttendance/weekPlan?format=json` | `PUT` or `DELETE /LocalAttendance/weekPlan/{id}?format=json` | `POST /LocalAttendance/weekPlanSearch?format=json` |
| Plan template | `POST /LocalAttendance/planTemplate?format=json` | `PUT` or `DELETE /LocalAttendance/planTemplate/{id}?format=json` | `POST /LocalAttendance/planTemplateSearch?format=json` |
| Group shift | Not advertised for add | `PUT /LocalAttendance/groupShift/{id}?format=json` | `POST /LocalAttendance/groupShiftSearch?format=json` |

Important: the add endpoints auto-assign IDs. Supplying `groupId`, `shiftId`, `weekPlanId`, or `planTemplateId` in a `POST` body did not force that ID.

See [device state change log](../device-state-change-log.md) for the non-read-only local-attendance probes and the repair performed after ID `1` objects were deleted during testing.
