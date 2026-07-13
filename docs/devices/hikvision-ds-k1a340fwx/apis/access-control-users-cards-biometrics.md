# Users, Cards, And Biometric Search Behavior

## User Info Capabilities

### Endpoint

`GET /ISAPI/AccessControl/UserInfo/capabilities?format=json`

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/UserInfo/capabilities?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "UserInfo": {
    "supportFunction": {
      "@opt": "post,delete,put,get,setUp"
    },
    "UserInfoSearchCond": {
      "maxResults": {
        "@min": 1,
        "@max": 30
      },
      "EmployeeNoList": {
        "maxSize": 30,
        "employeeNo": {
          "@min": 1,
          "@max": 32
        }
      },
      "fuzzySearch": {
        "@min": 0,
        "@max": 32
      },
      "isSupportNumOfFace": 0,
      "isSupportNumOfFP": 0,
      "isSupportNumOfCard": 0
    },
    "UserInfoDelCond": {
      "EmployeeNoList": {
        "maxSize": 30,
        "employeeNo": {
          "@min": 1,
          "@max": 32
        }
      }
    },
    "employeeNo": {
      "@min": 1,
      "@max": 32
    },
    "name": {
      "@min": 0,
      "@max": 128
    },
    "userType": {
      "@opt": "normal"
    },
    "closeDelayEnabled": "true,false",
    "Valid": {
      "enable": "true,false",
      "beginTime": {
        "@min": 1,
        "@max": 32
      },
      "endTime": {
        "@min": 1,
        "@max": 32
      },
      "timeRangeBegin": "2000-01-01T00:00:00",
      "timeRangeEnd": "2037-12-31T23:59:59",
      "timeType": {
        "@opt": "local,UTC"
      }
    },
    "maxBelongGroup": 4,
    "belongGroup": {
      "@min": 1,
      "@max": 32
    },
    "password": {
      "@min": 1,
      "@max": 8
    },
    "doorRight": {
      "@min": 1,
      "@max": 1
    },
    "RightPlan": {
      "maxSize": 1,
      "doorNo": {
        "@min": 1,
        "@max": 1
      },
      "maxPlanTemplate": 4,
      "planTemplateNo": {
        "@min": 1,
        "@max": 255
      }
    },
    "maxOpenDoorTime": {
      "@min": 0,
      "@max": 255
    },
    "openDoorTime": {
      "@min": 0,
      "@max": 255
    },
    "roomNumber": {
      "@min": 0,
      "@max": 99
    },
    "floorNumber": {
      "@min": 0,
      "@max": 99
    },
    "localUIRight": "true,false",
    "userVerifyMode": {
      "@opt": "faceOrFpOrCardOrPw,face,fp,fpOrface,faceAndFp,faceOrPw,faceAndPw,fpAndPw,faceAndPwAndFp"
    },
    "maxRecordNum": 1500,
    "gender": {
      "@opt": "male,female,unknown"
    },
    "purePwdVerifyEnable": true
  }
}
```

## User Count

### Endpoint

`GET /ISAPI/AccessControl/UserInfo/Count?format=json`

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/UserInfo/Count?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "UserInfoCount": {
    "userNumber": 1
  }
}
```

## User Search

### Endpoint

`POST /ISAPI/AccessControl/UserInfo/Search?format=json`

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/UserInfo/Search?format=json" \
  --data '{
    "UserInfoSearchCond": {
      "searchID": "doc-probe-user-1",
      "searchResultPosition": 0,
      "maxResults": 1
    }
  }'
```

### Observed Response

Status: `200 OK`

```json
{
  "UserInfoSearch": {
    "searchID": "doc-probe-user-1",
    "responseStatusStrg": "OK",
    "numOfMatches": 1,
    "totalMatches": 1,
    "UserInfo": [
      {
        "employeeNo": "1",
        "name": "admin",
        "userType": "normal",
        "closeDelayEnabled": false,
        "Valid": {
          "enable": true,
          "beginTime": "2026-07-06T00:38:29",
          "endTime": "2037-12-31T23:59:59",
          "timeType": "local"
        },
        "belongGroup": "",
        "password": "",
        "doorRight": "1",
        "RightPlan": [
          {
            "doorNo": 1,
            "planTemplateNo": "1"
          }
        ],
        "maxOpenDoorTime": 0,
        "openDoorTime": 0,
        "roomNumber": 0,
        "floorNumber": 0,
        "localUIRight": true,
        "gender": "unknown",
        "numOfCard": 0,
        "numOfFP": 1,
        "numOfFace": 1,
        "groupId": 1,
        "localAtndPlanTemplateId": 0
      }
    ]
  }
}
```

## Card Count

### Endpoint

`GET /ISAPI/AccessControl/CardInfo/Count?format=json`

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/CardInfo/Count?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "CardInfoCount": {
    "cardNumber": 0
  }
}
```

## Card Search

### Endpoint

`POST /ISAPI/AccessControl/CardInfo/Search?format=json`

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/CardInfo/Search?format=json" \
  --data '{
    "CardInfoSearchCond": {
      "searchID": "doc-card-1",
      "searchResultPosition": 0,
      "maxResults": 1
    }
  }'
```

### Observed Response

Status: `200 OK`

```json
{
  "CardInfoSearch": {
    "searchID": "doc-card-1",
    "responseStatusStrg": "NO MATCH",
    "numOfMatches": 0,
    "totalMatches": 0
  }
}
```

## Face And Fingerprint Search Tests

### Face Search Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/FaceInfo/Search?format=json" \
  --data '{
    "FaceInfoSearchCond": {
      "searchID": "doc-face-1",
      "searchResultPosition": 0,
      "maxResults": 1
    }
  }'
```

### Observed Face Search Response

Status: `200 OK`

```json
{
  "statusCode": 4,
  "statusString": "Invalid Operation",
  "subStatusCode": "notSupport",
  "errorCode": 1073741825,
  "errorMsg": "notSupport"
}
```

### Fingerprint Search Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -H 'Content-Type: application/json' \
  -X POST \
  "$ISAPI_BASE/ISAPI/AccessControl/FingerPrint/Search?format=json" \
  --data '{
    "FingerPrintSearchCond": {
      "searchID": "doc-fp-1",
      "searchResultPosition": 0,
      "maxResults": 1
    }
  }'
```

### Observed Fingerprint Search Response

Status: `200 OK`

```json
{
  "statusCode": 4,
  "statusString": "Invalid Operation",
  "subStatusCode": "notSupport",
  "errorCode": 1073741825,
  "errorMsg": "notSupport"
}
```

## Integration Notes

- Use `POST` for search endpoints. `GET` against `.../Search?format=json` returned `404`.
- User records expose biometric counts but not raw biometric templates through the tested search endpoints.
- User/card create, modify, and delete endpoints were validated with empty bodies to identify required roots, but no valid user or card write was executed. See [write operation behavior](access-control-write-operations.md).
