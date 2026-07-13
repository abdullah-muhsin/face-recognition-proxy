# Local Attendance Rules And Schemas

## Covered Endpoints

- `GET /ISAPI/AccessControl/LocalAttendance/rule?format=json`
- `GET /ISAPI/AccessControl/LocalAttendance/rule/capabilities?format=json`
- `GET /ISAPI/AccessControl/LocalAttendance/{Group,Shift,HolidayPlan,HolidayGroup,WeekPlan,PlanTemplate,GroupShift}/capabilities?format=json`
- `POST /ISAPI/AccessControl/LocalAttendance/.../Search?format=json` tested but rejected with `invalidID`

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
      "earliestSignInTime": {
        "@min": 0,
        "@max": 1440
      },
      "latestSignInTime": {
        "@min": 0,
        "@max": 1440
      },
      "allowLateTime": {
        "@min": 0,
        "@max": 1440
      },
      "earliestSignOutTime": {
        "@min": 0,
        "@max": 1440
      },
      "latestSignOutTime": {
        "@min": 0,
        "@max": 1440
      },
      "allowLeaveEarlyTime": {
        "@min": 0,
        "@max": 1440
      }
    },
    "supportedMethods": [
      "get",
      "put"
    ]
  }
}
```

## Group Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/Group/capabilities?format=json"
```

### Observed Response

```json
{
  "GroupCap": {
    "groupId": {
      "@min": 1,
      "@max": 128
    },
    "groupName": {
      "@min": 1,
      "@max": 32
    },
    "supportedMethods": [
      "post",
      "put",
      "delete"
    ]
  }
}
```

## Shift Capability

```json
{
  "ShiftCap": {
    "shiftId": {
      "@min": 1,
      "@max": 256
    },
    "shiftName": {
      "@min": 1,
      "@max": 32
    },
    "shiftType": {
      "@opt": [
        "normalShift",
        "manHoursShift"
      ]
    },
    "NormalShift": {
      "TimeRangeList": {
        "@size": 4,
        "signInTime": "00:00",
        "signOutTime": "00:00"
      }
    },
    "ManHoursShift": {
      "workHours": {
        "@min": 0,
        "@max": 1440
      },
      "latestSignInTime": "23:59:59",
      "RestTimeRangeList": {
        "@size": 3,
        "startRestTime": "00:00",
        "endRestTime": "23:59"
      }
    },
    "supportedMethods": [
      "post",
      "put",
      "delete"
    ]
  }
}
```

## Holiday Plan Capability

```json
{
  "HolidayPlanCap": {
    "holidayPlanId": {
      "@min": 1,
      "@max": 64
    },
    "holidayPlanName": {
      "@min": 1,
      "@max": 32
    },
    "holidayStartDate": "2000-01-01",
    "holidayEndDate": "2037-12-31",
    "supportedMethods": [
      "post",
      "put",
      "delete"
    ]
  }
}
```

## Holiday Group Capability

```json
{
  "HolidayGroupCap": {
    "holidayGroupId": {
      "@min": 1,
      "@max": 640
    },
    "holidayGroupName": {
      "@min": 1,
      "@max": 32
    },
    "holidayPlanList": {
      "@size": 64,
      "@min": 0,
      "@max": 64
    },
    "supportedMethods": [
      "post",
      "put",
      "delete"
    ]
  }
}
```

## Week Plan Capability

```json
{
  "WeekPlanCap": {
    "weekPlanId": {
      "@min": 1,
      "@max": 640
    },
    "weekPlanName": {
      "@min": 1,
      "@max": 32
    },
    "WeekPlanCfg": {
      "@size": 7,
      "week": {
        "@opt": [
          "Monday",
          "Tuesday",
          "Wednesday",
          "Thursday",
          "Friday",
          "Saturday",
          "Sunday"
        ]
      },
      "shiftId": {
        "@min": 0,
        "@max": 256
      }
    },
    "supportedMethods": [
      "post",
      "put",
      "delete"
    ]
  }
}
```

## Plan Template Capability

```json
{
  "PlanTemplateCap": {
    "planTemplateId": {
      "@min": 1,
      "@max": 640
    },
    "templateName": {
      "@min": 1,
      "@max": 32
    },
    "WeekPlanList": {
      "@size": 8,
      "startDate": "2000-01-01",
      "endDate": "2037-12-31",
      "weekPlanId": {
        "@min": 0,
        "@max": 640
      }
    },
    "holidayGroupNo": {
      "@size": 640,
      "@min": 1,
      "@max": 640
    },
    "supportedMethods": [
      "post",
      "put",
      "delete"
    ]
  }
}
```

## Group Shift Capability

```json
{
  "GroupShiftCap ": {
    "groupId": {
      "@min": 1,
      "@max": 128
    },
    "groupName": {
      "@min": 1,
      "@max": 32
    },
    "planTemplateId": {
      "@min": 1,
      "@max": 640
    },
    "supportedMethods": [
      "put"
    ]
  }
}
```

## Search Attempts

The top-level access-control capability advertises local-attendance search support. These paths exist for `POST`, but the body shapes tested returned `400 invalidID`.

Example tested request:

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/LocalAttendance/Group/Search?format=json" \
  --data '{
    "GroupSearchCond": {
      "searchID": "1",
      "searchResultPosition": 0,
      "maxResults": 5
    }
  }'
```

Observed response:

```json
{
  "statusCode": 4,
  "statusString": "Invalid Operation",
  "subStatusCode": "invalidID",
  "id": -1,
  "errorCode": 1073745928,
  "errorMsg": "invalidID"
}
```

The same `invalidID` response was observed for `Group`, `Shift`, `HolidayPlan`, `HolidayGroup`, `WeekPlan`, `PlanTemplate`, and `GroupShift` search attempts using generic paging bodies and simple explicit ID bodies.

## Integration Notes

- Capability endpoints are reliable for schema/range discovery.
- Create/update/delete methods are advertised but were not executed.
- Search endpoint body requirements are not fully discovered for this firmware.

