# Verification Plans

## Covered Endpoints

- `GET /ISAPI/AccessControl/VerifyWeekPlanCfg/capabilities?format=json`
- `GET /ISAPI/AccessControl/VerifyWeekPlanCfg/{1..2}?format=json`
- `GET /ISAPI/AccessControl/VerifyHolidayPlanCfg/capabilities?format=json`
- `GET /ISAPI/AccessControl/VerifyHolidayPlanCfg/1?format=json`
- `GET /ISAPI/AccessControl/VerifyHolidayGroupCfg/capabilities?format=json`
- `GET /ISAPI/AccessControl/VerifyHolidayGroupCfg/1?format=json`
- `GET /ISAPI/AccessControl/VerifyPlanTemplate/capabilities?format=json`
- `GET /ISAPI/AccessControl/VerifyPlanTemplate/{1..2}?format=json`
- `GET /ISAPI/AccessControl/UserRightPlanTemplate/capabilities?format=json`
- `GET /ISAPI/AccessControl/UserRightPlanTemplate/{1..2}?format=json`
- `GET /ISAPI/AccessControl/CardReaderPlan/capabilities?format=json`
- `GET /ISAPI/AccessControl/CardReaderPlan/{1..2}?format=json`
- `GET /ISAPI/AccessControl/ClearPlansCfg/capabilities?format=json`

## Verification Week Plan

### Capability Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/VerifyWeekPlanCfg/capabilities?format=json"
```

### Capability Response

```json
{
  "VerifyWeekPlanCfg": {
    "planNo": { "@min": 1, "@max": 2 },
    "enable": "true,false",
    "WeekPlanCfg": {
      "maxSize": 56,
      "week": { "@opt": "Monday,Tuesday,Wednesday,Thursday,Friday,Saturday,Sunday" },
      "id": { "@min": 1, "@max": 8 },
      "enable": "true,false",
      "verifyMode": {
        "@opt": "faceOrFpOrCardOrPw,face,fp,fpOrface,faceAndFp,faceOrPw,faceAndPw,fpAndPw,faceAndPwAndFp"
      },
      "TimeSegment": {
        "beginTime": "00:00:00",
        "endTime": "24:00:00",
        "validUnit": "second"
      }
    }
  }
}
```

### Plan Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/VerifyWeekPlanCfg/1?format=json"
```

### Observed Response Summary

Status: `200 OK`

The full response contains `56` rows: `7` weekdays times `8` segments. The enabled rows are segment `id: 1` for each day:

```json
{
  "VerifyWeekPlanCfg": {
    "enable": true,
    "WeekPlanCfg": {
      "totalRows": 56,
      "enabledRows": [
        {
          "week": "Monday",
          "id": 1,
          "enable": true,
          "verifyMode": "faceOrFpOrCardOrPw",
          "TimeSegment": {
            "beginTime": "00:00:00",
            "endTime": "24:00:00"
          }
        }
      ]
    }
  }
}
```

Plans `1` and `2` were both enabled with the same all-day first segment.

## Verification Holiday Plan

### Capability Response

```json
{
  "VerifyHolidayPlanCfg": {
    "planNo": { "@min": 1, "@max": 128 },
    "enable": "true,false",
    "beginDate": "2000-01-01",
    "endDate": "2037-12-31",
    "HolidayPlanCfg": {
      "maxSize": 8,
      "id": { "@min": 1, "@max": 8 },
      "enable": "true,false",
      "verifyMode": {
        "@opt": "faceOrFpOrCardOrPw,face,fp,fpOrface,faceAndFp,faceOrPw,faceAndPw,fpAndPw,faceAndPwAndFp"
      },
      "TimeSegment": {
        "beginTime": "00:00:00",
        "endTime": "24:00:00",
        "validUnit": "second"
      }
    }
  }
}
```

### Observed Plan 1

```json
{
  "VerifyHolidayPlanCfg": {
    "enable": false,
    "beginDate": "2000-01-01",
    "endDate": "2037-12-31",
    "HolidayPlanCfg": [
      {
        "id": 1,
        "enable": false,
        "verifyMode": "faceOrFpOrCardOrPw",
        "TimeSegment": {
          "beginTime": "00:00:00",
          "endTime": "23:59:59"
        }
      }
    ]
  }
}
```

Plan 1 contains eight disabled holiday segments with the same default mode and time range.

## Verification Holiday Group

```json
{
  "VerifyHolidayGroupCfg": {
    "groupNo": { "@min": 1, "@max": 8 },
    "enable": "true,false",
    "groupName": { "@min": 1, "@max": 32 },
    "holidayPlanNo": { "@min": 1, "@max": 128 }
  }
}
```

Observed group `1`:

```json
{
  "VerifyHolidayGroupCfg": {
    "enable": false,
    "groupName": "",
    "holidayPlanNo": ""
  }
}
```

## Plan Templates

### Verify Plan Template

```json
{
  "VerifyPlanTemplate": {
    "templateNo": { "@min": 1, "@max": 2 },
    "enable": "true,false",
    "templateName": { "@min": 1, "@max": 32 },
    "weekPlanNo": { "@min": 1, "@max": 2 },
    "holidayGroupNo": { "@min": 1, "@max": 8 }
  }
}
```

Observed templates:

```json
[
  {
    "templateNo": 1,
    "enable": true,
    "templateName": "",
    "weekPlanNo": 1,
    "holidayGroupNo": ""
  },
  {
    "templateNo": 2,
    "enable": false,
    "templateName": "",
    "weekPlanNo": 0,
    "holidayGroupNo": ""
  }
]
```

### User Right Plan Template

```json
{
  "UserRightPlanTemplate": {
    "templateNo": { "@min": 1, "@max": 255 },
    "enable": "true,false",
    "templateName": { "@min": 1, "@max": 32 },
    "weekPlanNo": { "@min": 1, "@max": 128 },
    "holidayGroupNo": { "@min": 1, "@max": 64 }
  }
}
```

Observed templates `1` and `2` matched the verify-plan template status above.

## Card Reader Plan

### Capability Response

```json
{
  "CardReaderPlan": {
    "cardReaderNo": { "@min": 1, "@max": 2 },
    "templateNo": { "@min": 1, "@max": 2 }
  }
}
```

Observed reader plan mapping:

```json
[
  { "cardReaderNo": 1, "templateNo": 1 },
  { "cardReaderNo": 2, "templateNo": 2 }
]
```

## Clear Plans Capability

This endpoint describes a destructive clear operation. It was read only; no clear operation was executed.

```json
{
  "ClearPlansCfg": {
    "ClearFlags": {
      "doorStatusWeekPlan": "true,false",
      "cardReaderWeekPlan": "true,false",
      "userRightWeekPlan": "true,false",
      "doorStatusHolidayPlan": "true,false",
      "cardReaderHolidayPlan": "true,false",
      "userRightHolidayPlan": "true,false",
      "doorStatusHolidayGroup": "true,false",
      "cardReaderHolidayGroup": "true,false",
      "userRightHolidayGroup": "true,false",
      "doorStatusTemplate": "true,false",
      "cardReaderTemplate": "true,false",
      "userRightTemplate": "true,false"
    }
  }
}
```

## Integration Notes

- The working endpoint names include `Cfg` for week/holiday plans, but not for `/VerifyPlanTemplate`.
- Common variants such as `/VerifyWeekPlan/...` and `/VerifyHolidayPlan/...` returned `notSupport`; use the exact paths above.
- Verification schedules and local attendance schedules are separate API families.
