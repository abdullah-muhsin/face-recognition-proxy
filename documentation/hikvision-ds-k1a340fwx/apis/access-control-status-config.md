# Access-Control Status And Configuration

## Covered Endpoints

- `GET /ISAPI/AccessControl/AcsWorkStatus?format=json`
- `GET /ISAPI/AccessControl/AcsWorkStatus/capabilities?format=json`
- `GET /ISAPI/AccessControl/AcsCfg?format=json`
- `GET /ISAPI/AccessControl/AcsCfg/capabilities?format=json`
- `GET /ISAPI/AccessControl/EventOptimizationCfg?format=json`
- `GET /ISAPI/AccessControl/AcsEvent/StorageCfg?format=json`
- `GET /ISAPI/AccessControl/ClearPictureCfg/capabilities?format=json`

## Work Status

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/AcsWorkStatus?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "AcsWorkStatus": {
    "doorLockStatus": [0],
    "doorStatus": [4],
    "magneticStatus": [0],
    "caseStatus": [],
    "powerSupplyStatus": "ACPowerSupply",
    "antiSneakStatus": "close",
    "hostAntiDismantleStatus": "close",
    "cardReaderOnlineStatus": [1],
    "cardReaderAntiDismantleStatus": [],
    "ezvizStatus": "unregistered",
    "cardReaderVerifyMode": [10, 3],
    "alarmOutStatus": [0],
    "cardNum": 0
  }
}
```

### Capability Response

```json
{
  "AcsWorkStatus": {
    "doorLockStatus": {
      "@opt": "0,1,2,3,4"
    },
    "doorStatus": {
      "@opt": "1,2,3,4"
    },
    "magneticStatus": {
      "@opt": "0,1,2,3,4"
    },
    "powerSupplyStatus": {
      "@opt": "ACPowerSupply"
    },
    "antiSneakStatus": {
      "@opt": "close,open"
    },
    "hostAntiDismantleStatus": {
      "@opt": "close,open"
    },
    "cardReaderVerifyMode": {
      "@opt": "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24"
    },
    "cardNum": {
      "@min": 1,
      "@max": 50000
    }
  }
}
```

## ACS Configuration

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/AcsCfg?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "AcsCfg": {
    "showCapPic": false,
    "overlayUserInfo": false,
    "voicePrompt": true,
    "uploadCapPic": true,
    "saveCapPic": true,
    "showPicture": true,
    "showEmployeeNo": true,
    "showName": true,
    "uploadVerificationPic": true,
    "saveVerificationPic": true,
    "saveFacePic": true,
    "desensitiseEmployeeNo": true,
    "desensitiseName": true
  }
}
```

### Capability Response

```json
{
  "AcsCfg": {
    "showCapPic": "true,false",
    "overlayUserInfo": "true,false",
    "voicePrompt": "true,false",
    "uploadCapPic": "true,false",
    "saveCapPic": "true,false",
    "showPicture": "true,false",
    "showEmployeeNo": "true,false",
    "showName": "true,false",
    "uploadVerificationPic": "true,false",
    "saveVerificationPic": "true,false",
    "saveFacePic": "true,false",
    "desensitiseEmployeeNo": "true,false",
    "desensitiseName": "true,false"
  }
}
```

## Event Optimization

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/EventOptimizationCfg?format=json"
```

### Observed Response

```json
{
  "EventOptimizationCfg": {
    "enable": true,
    "isCombinedLinkageEvents": true
  }
}
```

Capability:

```json
{
  "EventOptimizationCfg": {
    "enable": "true,false",
    "isCombinedLinkageEvents": "true,false"
  }
}
```

## Event Storage

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/AcsEvent/StorageCfg?format=json"
```

### Observed Response

```json
{
  "EventStorageCfg": {
    "mode": "cycle"
  }
}
```

Capability:

```json
{
  "EventStorageCfgCap": {
    "mode": {
      "@opt": [
        "regular",
        "time",
        "cycle"
      ]
    },
    "checkTime": {
      "@min": 0,
      "@max": 32
    },
    "period": {
      "@min": 10,
      "@max": 86400
    }
  }
}
```

## Clear Picture Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/ClearPictureCfg/capabilities?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "ClearPictureCfgCap": {
    "ClearFlags": {
      "facePicture": {
        "@opt": [
          true,
          false
        ]
      },
      "capOrVerifyPicture": {
        "@opt": [
          true,
          false
        ]
      }
    }
  }
}
```

## Integration Notes

- `AcsWorkStatus` is useful for health/status polling.
- `AcsCfg` shows that captured, verification, and face pictures are saved and uploaded.
- Clear-picture APIs are destructive; only capability discovery was tested.

