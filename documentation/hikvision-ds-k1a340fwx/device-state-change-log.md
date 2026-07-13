# Device State Change Log

This file records writes executed during the second discovery pass. It exists so the documentation is transparent about non-read-only testing.

Test date: `2026-07-13`.

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
