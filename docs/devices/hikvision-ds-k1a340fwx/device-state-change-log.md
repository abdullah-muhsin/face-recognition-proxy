# Device State Change Log

This file records writes executed during discovery and integration probes. It exists so the documentation is transparent about non-read-only testing.

Test date: `2026-07-13`.

## SSH Enable Operation

The following write was executed by the operator after the second discovery pass and then verified live:

```bash
PUT /ISAPI/System/Network/ssh
```

Request body:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
</SSH>
```

Observed response:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ResponseStatus version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <requestURL></requestURL>
  <statusCode>1</statusCode>
  <statusString>OK</statusString>
  <subStatusCode>ok</subStatusCode>
</ResponseStatus>
```

Verified post-change state:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
</SSH>
```

TCP `22` then opened as Dropbear SSH `2022.83`.

## HTTP Notification Host Probe

The HTTP notification host was temporarily changed while testing direct event delivery:

```bash
PUT /ISAPI/Event/notification/httpHosts/1
PUT /ISAPI/Event/notification/httpHosts
```

Temporary target:

```xml
<HttpHostNotification version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>1</id>
  <url></url>
  <protocolType>HTTP</protocolType>
  <parameterFormatType>XML</parameterFormatType>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>192.168.1.2</ipAddress>
  <portNo>8088</portNo>
  <httpAuthenticationMethod>none</httpAuthenticationMethod>
</HttpHostNotification>
```

The device accepted the write and persisted `ipAddress` and `portNo`. It did not persist a non-empty `url`. No outbound HTTP delivery was observed for a live `AccessControllerEvent`.

The host was restored after the probe:

```xml
<HttpHostNotification version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>1</id>
  <url></url>
  <protocolType>HTTP</protocolType>
  <parameterFormatType>XML</parameterFormatType>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>0.0.0.0</ipAddress>
  <portNo>0</portNo>
  <httpAuthenticationMethod>none</httpAuthenticationMethod>
</HttpHostNotification>
```

## EHome / ISUP Registration Probe

The EHome/ISUP platform configuration was temporarily changed while testing whether the terminal can initiate outbound ISUP registration to a server.

Write endpoint:

```bash
PUT /ISAPI/System/Network/Ehome?centerID=1
```

Temporary target:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
  <deviceID>K1A340FWXPROBE1</deviceID>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>192.168.1.2</ipAddress>
  <portNo>7660</portNo>
  <key>********</key>
  <protocolVersion>v5.0</protocolVersion>
</Ehome>
```

The key was an 8-character temporary probe key and is intentionally redacted.

Observed response:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ResponseStatus version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <requestURL></requestURL>
  <statusCode>1</statusCode>
  <statusString>OK</statusString>
  <subStatusCode>ok</subStatusCode>
</ResponseStatus>
```

Verified temporary state:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>192.168.1.2</ipAddress>
  <portNo>7660</portNo>
  <deviceID>K1A340FWXPROBE1</deviceID>
  <registerStatus>false</registerStatus>
  <protocolVersion>v5.0</protocolVersion>
</Ehome>
```

A direct TCP listener on `192.168.1.2:7660` accepted repeated connections from the terminal at `192.168.1.6`. The first payloads were binary ISUP registration attempts. They included visible strings for serial suffix `J59360966`, model `DS-K1A340FWX`, and configured device ID `K1A340FWXPROBE1`.

Representative payload prefix:

```text
HEX   10 57 01 01 00 09 4A 35 39 33 36 30 39 36 36 0C 44 53 2D 4B 31 41 33 34 30 46 57 58 29 2C 0F 4B 31 41 33 34 30 46 57 58 50 52 4F 42 45 31 ...
ASCII .W....J59360966.DS-K1A340FWX),.K1A340FWXPROBE1...
```

The probe listener was not a Hikvision ISUP server, so it did not complete registration. `registerStatus` remained `false`.

### ISUP Cleanup

A direct attempt to restore the original factory-empty EHome state failed because this firmware rejects `portNo 0` in a `PUT` body after EHome has been configured:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ResponseStatus version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <requestURL></requestURL>
  <statusCode>6</statusCode>
  <statusString>Invalid Content</statusString>
  <subStatusCode>portError</subStatusCode>
  <errorCode>1610612748</errorCode>
  <errorMsg>portError</errorMsg>
</ResponseStatus>
```

Blank or omitted fields were accepted while disabled but mostly ignored. The final verified state was disabled and non-routable:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>0.0.0.0</ipAddress>
  <portNo>1024</portNo>
  <deviceID>0</deviceID>
  <registerStatus>false</registerStatus>
  <protocolVersion>v5.0</protocolVersion>
</Ehome>
```

This differs from the original factory-empty readback, but ISUP is disabled and points at no routable host.

## Local Attendance DELETE Probe

The following calls were executed with an empty JSON body while probing method support:

```bash
DELETE /ISAPI/AccessControl/LocalAttendance/Group/1?format=json
DELETE /ISAPI/AccessControl/LocalAttendance/Shift/1?format=json
DELETE /ISAPI/AccessControl/LocalAttendance/HolidayPlan/1?format=json
DELETE /ISAPI/AccessControl/LocalAttendance/HolidayGroup/1?format=json
DELETE /ISAPI/AccessControl/LocalAttendance/WeekPlan/1?format=json
DELETE /ISAPI/AccessControl/LocalAttendance/PlanTemplate/1?format=json
```

Observed response shape:

```json
{
  "statusCode": 1,
  "statusString": "OK",
  "subStatusCode": "ok",
  "id": 1
}
```

These calls were destructive. After the probe, the lower-case search endpoints showed default local-attendance records starting at ID `2`, with ID `1` absent.

## Attempted Restoration Of ID 1

`POST` attempts with explicit `*Id: 1` did not recreate ID `1`; the device auto-assigned new IDs. Those temporary records were deleted immediately.

Temporary records created and then deleted:

| Type | Auto-assigned ID |
| --- | --- |
| Group | `8` |
| Shift | `4` |
| Week plan | `8` |
| Plan template | `8` |

Indexed `PUT /LocalAttendance/{object}/1?format=json` attempts returned:

```json
{
  "statusCode": 3,
  "statusString": "Device Error",
  "subStatusCode": "deviceError",
  "id": -1,
  "errorCode": 805306369,
  "errorMsg": "deviceError"
}
```

## Consistency Repair

After ID `1` shift was absent, week plans `2..7` still referenced `shiftId: 1`. To keep local attendance schedules internally consistent, week plans `2..7` were updated to reference existing `shiftId: 2`, which has the same observed 08:00 to 17:00 normal-shift pattern.

Repair request shape:

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X PUT \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/weekPlan/2?format=json" \
  --data '{
    "WeekPlan": {
      "weekPlanName": "Week Schedule2",
      "WeekPlanCfg": [
        { "week": "Sunday", "shiftId": 0 },
        { "week": "Monday", "shiftId": 2 },
        { "week": "Tuesday", "shiftId": 2 },
        { "week": "Wednesday", "shiftId": 2 },
        { "week": "Thursday", "shiftId": 2 },
        { "week": "Friday", "shiftId": 2 },
        { "week": "Saturday", "shiftId": 0 }
      ]
    }
  }'
```

The same pattern was applied to week plans `2`, `3`, `4`, `5`, `6`, and `7`.

Each update returned:

```json
{
  "statusCode": 1,
  "statusString": "OK",
  "subStatusCode": "ok"
}
```

## Verified Final Local Attendance State

After cleanup and repair:

| Object | Verified records |
| --- | --- |
| Groups | IDs `2..7`: `Admin. Dept.`, `Sales Dept.`, `Financial Dept.`, `Production Dept.`, `Purchasing Dept.`, `R&D Dept.` |
| Shifts | ID `2`: `08:00-17:00`; ID `3`: `09:00-12:00` and `13:00-18:00` |
| Week plans | IDs `2..7`, weekdays reference `shiftId: 2`, weekends reference `0` |
| Plan templates | IDs `2..7`, each mapped to the same-numbered week plan |
| Group shifts | Groups `2..7` mapped to plan templates `2..7` |
| Holidays | No holiday plans or holiday groups configured |

## Operational Note

Do not use HTTP `DELETE` probes against a production attendance device unless the target object is known and intentionally being removed. This firmware treats those calls as real deletes even with an empty body.
