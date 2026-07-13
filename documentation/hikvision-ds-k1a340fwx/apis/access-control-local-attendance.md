# Local Attendance Rules, Schedules, And Searches

## Covered Endpoints

- `GET /ISAPI/AccessControl/LocalAttendance/rule?format=json`
- `GET /ISAPI/AccessControl/LocalAttendance/rule/capabilities?format=json`
- `GET /ISAPI/AccessControl/LocalAttendance/{group,shift,holidayPlan,holidayGroup,weekPlan,planTemplate,groupShift}/capabilities?format=json`
- `GET /ISAPI/AccessControl/LocalAttendance/{groupSearch,shiftSearch,holidayPlanSearch,holidayGroupSearch,weekPlanSearch,planTemplateSearch,groupShiftSearch}/capabilities?format=json`
- `POST /ISAPI/AccessControl/LocalAttendance/{groupSearch,shiftSearch,holidayPlanSearch,holidayGroupSearch,weekPlanSearch,planTemplateSearch,groupShiftSearch}?format=json`

## Important Path Rule

Use the lower-case web UI paths. Earlier guessed paths like `/LocalAttendance/Group/Search?format=json` returned `invalidID`; the correct path is `/LocalAttendance/groupSearch?format=json`.

## Local Attendance Rule

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/rule?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "Rule": {
    "NormalShift": {
      "earliestSignInTime": 0,
      "latestSignInTime": 0,
      "allowLateTime": 0,
      "earliestSignOutTime": 0,
      "latestSignOutTime": 0,
      "allowLeaveEarlyTime": 0
    }
  }
}
```

### Capability Response

```json
{
  "RuleCap": {
    "NormalShift": {
      "earliestSignInTime": { "@min": 0, "@max": 1440 },
      "latestSignInTime": { "@min": 0, "@max": 1440 },
      "allowLateTime": { "@min": 0, "@max": 1440 },
      "earliestSignOutTime": { "@min": 0, "@max": 1440 },
      "latestSignOutTime": { "@min": 0, "@max": 1440 },
      "allowLeaveEarlyTime": { "@min": 0, "@max": 1440 }
    },
    "supportedMethods": [
      "get",
      "put"
    ]
  }
}
```

## Object Capability Summary

| Object | Capability endpoint | ID range | Supported methods |
| --- | --- | --- | --- |
| Group | `/LocalAttendance/group/capabilities?format=json` | `1..128` | `post`, `put`, `delete` |
| Shift | `/LocalAttendance/shift/capabilities?format=json` | `1..256` | `post`, `put`, `delete` |
| Holiday plan | `/LocalAttendance/holidayPlan/capabilities?format=json` | `1..64` | `post`, `put`, `delete` |
| Holiday group | `/LocalAttendance/holidayGroup/capabilities?format=json` | `1..640` | `post`, `put`, `delete` |
| Week plan | `/LocalAttendance/weekPlan/capabilities?format=json` | `1..640` | `post`, `put`, `delete` |
| Plan template | `/LocalAttendance/planTemplate/capabilities?format=json` | `1..640` | `post`, `put`, `delete` |
| Group shift | `/LocalAttendance/groupShift/capabilities?format=json` | group `1..128`, template `1..640` | `put` |

## Search Body Pattern

All working local-attendance searches require:

- the object-specific root, such as `GroupSearchCond` or `ShiftSearchCond`
- `searchID`
- `searchResultPosition`
- `maxResults`
- `searchType`

Example:

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/groupSearch?format=json" \
  --data '{
    "GroupSearchCond": {
      "searchID": "doc-lagroup-all",
      "searchResultPosition": 0,
      "searchType": "all",
      "maxResults": 30
    }
  }'
```

## Group Search

### Observed Response

Status: `200 OK`

```json
{
  "GroupSearchResult": {
    "searchID": "doc-lagroup-all",
    "responseStatusStrg": "OK",
    "numOfMatches": 6,
    "totalMatches": 6,
    "GroupInfo": [
      { "groupId": 2, "groupName": "Admin. Dept." },
      { "groupId": 3, "groupName": "Sales Dept." },
      { "groupId": 4, "groupName": "Financial Dept." },
      { "groupId": 5, "groupName": "Production Dept." },
      { "groupId": 6, "groupName": "Purchasing Dept." },
      { "groupId": 7, "groupName": "R&D Dept." }
    ]
  }
}
```

## Shift Search

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/shiftSearch?format=json" \
  --data '{
    "ShiftSearchCond": {
      "searchID": "doc-lashift-all",
      "searchResultPosition": 0,
      "searchType": "all",
      "maxResults": 20
    }
  }'
```

### Observed Response

```json
{
  "ShiftSearchResult": {
    "searchID": "doc-lashift-all",
    "responseStatusStrg": "OK",
    "numOfMatches": 2,
    "totalMatches": 2,
    "ShiftInfo": [
      {
        "shiftId": 2,
        "shiftName": "Normal Shift2",
        "shiftType": "normalShift",
        "NormalShift": {
          "TimeRangeList": [
            { "signInTime": "08:00", "signOutTime": "17:00" },
            { "signInTime": "00:00", "signOutTime": "00:00" },
            { "signInTime": "00:00", "signOutTime": "00:00" },
            { "signInTime": "00:00", "signOutTime": "00:00" }
          ]
        }
      },
      {
        "shiftId": 3,
        "shiftName": "Normal Shift3",
        "shiftType": "normalShift",
        "NormalShift": {
          "TimeRangeList": [
            { "signInTime": "09:00", "signOutTime": "12:00" },
            { "signInTime": "13:00", "signOutTime": "18:00" },
            { "signInTime": "00:00", "signOutTime": "00:00" },
            { "signInTime": "00:00", "signOutTime": "00:00" }
          ]
        }
      }
    ]
  }
}
```

## Holiday Searches

Holiday plans and holiday groups are supported but currently empty.

```json
{
  "HolidayPlanSearchResult": {
    "responseStatusStrg": "NO MATCH",
    "numOfMatches": 0,
    "totalMatches": 0
  }
}
```

The same no-match result was observed for `holidayGroupSearch`.

## Week Plan Search

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/weekPlanSearch?format=json" \
  --data '{
    "WeekPlanSearchCond": {
      "searchID": "doc-laweek-all",
      "searchResultPosition": 0,
      "searchType": "all",
      "maxResults": 30
    }
  }'
```

### Observed Response Summary

Status: `200 OK`

Week plans `2..7` exist. After consistency repair, each uses no shift on Sunday/Saturday and `shiftId: 2` on Monday through Friday.

```json
{
  "weekPlanId": 2,
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
```

## Plan Template Search

Plan templates `2..7` exist and each maps to the same-numbered week plan:

```json
{
  "PlanTemplateSearchResult": {
    "responseStatusStrg": "OK",
    "numOfMatches": 6,
    "totalMatches": 6,
    "PlanTemplateInfo": [
      {
        "planTemplateId": 2,
        "templateName": "Shift Schedule2",
        "WeekPlanList": [
          {
            "startDate": "2026-01-01",
            "endDate": "2026-12-31",
            "weekPlanId": 2
          }
        ],
        "holidayGroupNo": []
      }
    ]
  }
}
```

## Group Shift Search

```json
{
  "GroupShiftSearchResult": {
    "responseStatusStrg": "OK",
    "numOfMatches": 6,
    "totalMatches": 6,
    "GroupShiftInfo": [
      { "groupId": 2, "groupName": "Admin. Dept.", "planTemplateId": 2 },
      { "groupId": 3, "groupName": "Sales Dept.", "planTemplateId": 3 },
      { "groupId": 4, "groupName": "Financial Dept.", "planTemplateId": 4 },
      { "groupId": 5, "groupName": "Production Dept.", "planTemplateId": 5 },
      { "groupId": 6, "groupName": "Purchasing Dept.", "planTemplateId": 6 },
      { "groupId": 7, "groupName": "R&D Dept.", "planTemplateId": 7 }
    ]
  }
}
```

## Write Behavior

The object endpoints are real write endpoints. `POST` creates new records and auto-assigns IDs; `PUT /{id}` edits existing records; `DELETE /{id}` deletes records. See [write operation behavior](access-control-write-operations.md) and [device state change log](../device-state-change-log.md).

## Integration Notes

- Search endpoints are reliable when using the exact lower-case path and object-specific root.
- `searchResultPosition` is required even though some capability responses do not list it.
- `searchType: "all"` works for all local attendance search endpoints.
- `searchType: "id"` works for group search, but returns `NO MATCH` if the ID is absent.
- Avoid blind `DELETE` tests; this firmware accepts them as destructive operations.
