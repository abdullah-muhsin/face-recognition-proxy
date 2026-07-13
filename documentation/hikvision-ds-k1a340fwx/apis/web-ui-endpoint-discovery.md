# Web UI Endpoint Discovery

## Purpose

The device's own web UI was inspected to discover model-specific ISAPI paths. This was useful because several advertised capabilities use endpoint names that differ from common Hikvision examples.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/" \
  -o /tmp/hikweb-assets/root.html

curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/doc/app.js" \
  -o /tmp/hikweb-assets/app.js
```

Additional lazy-loaded chunks were fetched for attendance, access-control, network, security, audio/video, and maintenance pages.

## Observed Web UI Shape

The root page is an Angular application and loads `doc/app.js`. The main bundle contained route names for code-split chunks and hundreds of ISAPI path strings.

Relevant discoveries that changed the probe results:

| Feature area | Correct path discovered in web UI | Earlier guessed path result |
| --- | --- | --- |
| Wi-Fi | `/ISAPI/System/Network/interfaces/2/wireless` | `/ISAPI/System/Network/wireless` returned `notSupport` |
| Local attendance group search | `/ISAPI/AccessControl/LocalAttendance/groupSearch?format=json` | `/LocalAttendance/Group/Search?format=json` returned `invalidID` |
| Local attendance object writes | `/LocalAttendance/group/{id}?format=json` and lower-case object names | Upper-case object paths were misleading and destructive when used with `DELETE` |
| Attendance mode | `/ISAPI/AccessControl/Configuration/attendanceMode?format=json` | `/AccessControl/AttendanceMode?format=json` returned `notSupport` |
| Verification TTS | `/ISAPI/AccessControl/Verification/ttsText?format=json` | `/AccessControl/TTSText?format=json` returned `notSupport` |
| Verification plans | `/VerifyWeekPlanCfg`, `/VerifyHolidayPlanCfg`, `/VerifyHolidayGroupCfg` | `/VerifyWeekPlan`, `/VerifyHolidayPlan`, `/VerifyHolidayGroup` returned `notSupport` |

## Integration Notes

- Treat the web UI endpoint map as a model-specific hint, not proof that every path is supported.
- Many generic UI paths returned `notSupport` on this device even though they were present in the JavaScript bundle.
- Static UI assets were stored only under `/tmp/hikweb-assets`; they were not added to the repository.
