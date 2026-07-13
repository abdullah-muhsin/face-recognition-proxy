# Attendance Configuration

## Covered Endpoints

- `GET /ISAPI/AccessControl/keyCfg/attendance?format=json`
- `GET /ISAPI/AccessControl/keyCfg/{1..6}/attendance?format=json`
- `GET /ISAPI/AccessControl/Attendance/weekPlan/capabilities?format=json`
- `GET /ISAPI/AccessControl/Attendance/weekPlan/1?format=json`
- `GET /ISAPI/AccessControl/Attendance/planTemplate/capabilities?format=json`
- `GET /ISAPI/AccessControl/Attendance/planTemplate?format=json`
- `GET /ISAPI/AccessControl/Attendance/planTemplate/1?format=json`

## Attendance Shortcut Keys

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/keyCfg/attendance?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "AttendanceList": [
    {
      "enable": true,
      "attendanceStatus": "checkIn",
      "label": "checkIn"
    },
    {
      "enable": true,
      "attendanceStatus": "checkOut",
      "label": "checkOut"
    },
    {
      "enable": true,
      "attendanceStatus": "breakOut",
      "label": "breakOut"
    },
    {
      "enable": true,
      "attendanceStatus": "breakIn",
      "label": "breakIn"
    },
    {
      "enable": false,
      "attendanceStatus": "overtimeIn",
      "label": "overtimeIn"
    },
    {
      "enable": false,
      "attendanceStatus": "overtimeOut",
      "label": "overtimeOut"
    }
  ]
}
```

### Indexed Key Requests

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/keyCfg/1/attendance?format=json"
```

Observed index mapping:

```text
1 -> checkIn, enabled
2 -> checkOut, enabled
3 -> breakOut, enabled
4 -> breakIn, enabled
5 -> overtimeIn, disabled
6 -> overtimeOut, disabled
7 -> 404
```

## Attendance Week Plan Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Attendance/weekPlan/capabilities?format=json"
```

### Observed Response

```json
{
  "AttendanceWeekPlanCap": {
    "planNo": {
      "@min": 1,
      "@max": 3
    },
    "enable": {
      "@opt": [
        true,
        false
      ]
    },
    "WeekPlanCfg": {
      "maxSize": 1,
      "id": {
        "@min": 1,
        "@max": 1
      },
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
      "enable": {
        "@opt": [
          true,
          false
        ]
      },
      "TimeSegment": {
        "beginTime": "00:00:00",
        "endTime": "23:59:59",
        "validUnit": "second"
      }
    }
  }
}
```

## Attendance Week Plan 1

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Attendance/weekPlan/1?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "AttendanceWeekPlan": {
    "enable": false,
    "WeekPlanCfg": [
      {
        "id": 1,
        "week": "Sunday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      },
      {
        "id": 1,
        "week": "Monday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      },
      {
        "id": 1,
        "week": "Tuesday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      },
      {
        "id": 1,
        "week": "Wednesday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      },
      {
        "id": 1,
        "week": "Thursday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      },
      {
        "id": 1,
        "week": "Friday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      },
      {
        "id": 1,
        "week": "Saturday",
        "enable": false,
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "00:00:00"
        }
      }
    ]
  }
}
```

## Attendance Plan Templates

### Capability Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Attendance/planTemplate/capabilities?format=json"
```

### Capability Response

```json
{
  "AttendancePlanTemplateCap": {
    "templateNo": {
      "@min": 1,
      "@max": 3
    },
    "property": {
      "@opt": [
        "check",
        "break",
        "overtime"
      ]
    },
    "enable": {
      "@opt": [
        true,
        false
      ]
    },
    "templateName": {
      "@min": 0,
      "@max": 32
    },
    "weekPlanNo": {
      "@min": 1,
      "@max": 3
    }
  }
}
```

### List Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Attendance/planTemplate?format=json"
```

### List Response

```json
{
  "AttendancePlanTemplateList": [
    {
      "templateNo": 1,
      "enable": false,
      "property": "check",
      "templateName": "template1",
      "weekPlanNo": 1
    },
    {
      "templateNo": 2,
      "enable": false,
      "property": "break",
      "templateName": "template2",
      "weekPlanNo": 2
    },
    {
      "templateNo": 3,
      "enable": false,
      "property": "overtime",
      "templateName": "template3",
      "weekPlanNo": 3
    }
  ]
}
```

### Template 1 Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/Attendance/planTemplate/1?format=json"
```

### Template 1 Response

```json
{
  "AttendancePlanTemplate": {
    "enable": false,
    "property": "check",
    "templateName": "template1",
    "weekPlanNo": 1
  }
}
```

## Integration Notes

- Attendance statuses exist even when no attendance schedule is enabled.
- Plan numbers and template numbers are limited to `1..3` on this firmware.
- Use access-control event search to read actual attendance/verification records.

