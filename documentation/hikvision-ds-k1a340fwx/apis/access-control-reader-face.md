# Card-Reader And Face-Recognition Configuration

## Covered Endpoints

- `GET /ISAPI/AccessControl/CardReaderCfg/capabilities?format=json`
- `GET /ISAPI/AccessControl/CardReaderCfg/1?format=json`
- `GET /ISAPI/AccessControl/CardReaderCfg/2?format=json`
- `GET /ISAPI/AccessControl/FaceRecognizeMode?format=json`
- `GET /ISAPI/AccessControl/FaceRecognizeMode/capabilities?format=json`

## Card-Reader Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/CardReaderCfg/capabilities?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "CardReaderCfg": {
    "cardReaderNo": {
      "@min": 1,
      "@max": 1
    },
    "enable": "true,false",
    "okLedPolarity": {
      "@opt": "cathode,anode"
    },
    "errorLedPolarity": {
      "@opt": "cathode,anode"
    },
    "swipeInterval": {
      "@min": 0,
      "@max": 255
    },
    "pressTimeout": {
      "@min": 1,
      "@max": 255
    },
    "enableFailAlarm": "true,false",
    "maxReadCardFailNum": {
      "@min": 1,
      "@max": 10
    },
    "offlineCheckTime": {
      "@min": 0,
      "@max": 255
    },
    "fingerPrintCheckLevel": {
      "@opt": "3,5,6,12,13"
    },
    "fingerPrintCapacity": {
      "@min": 1,
      "@max": 3000
    },
    "fingerPrintNum": {
      "@min": 0,
      "@max": 3000
    },
    "faceMatchThresholdN": {
      "@min": 0,
      "@max": 100
    },
    "faceRecogizeTimeOut": {
      "@min": 1,
      "@max": 20
    },
    "faceRecogizeInterval": {
      "@min": 1,
      "@max": 10
    },
    "cardReaderFunction": {
      "@opt": "fingerPrint,face,fingerVein"
    },
    "cardReaderDescription": {
      "@min": 1,
      "@max": 16
    },
    "faceImageSensitometry": {
      "@min": 0,
      "@max": 65535
    },
    "livingBodyDetect": "true,false",
    "faceMatchThreshold1": {
      "@min": 0,
      "@max": 100
    },
    "envirMode": {
      "@opt": "indoor,other"
    },
    "liveDetLevelSet": {
      "@opt": "low,middle,high"
    },
    "liveDetAntiAttackCntLimit": {
      "@min": 5,
      "@max": 15
    },
    "enableLiveDetAntiAttack": "true,false",
    "faceRecogizeEnable": {
      "@opt": "1,3"
    }
  }
}
```

## Reader 1

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/CardReaderCfg/1?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "CardReaderCfg": {
    "enable": true,
    "okLedPolarity": "anode",
    "errorLedPolarity": "anode",
    "buzzerPolarity": "anode",
    "swipeInterval": 0,
    "pressTimeout": 10,
    "enableFailAlarm": false,
    "maxReadCardFailNum": 5,
    "enableTamperCheck": true,
    "offlineCheckTime": 0,
    "fingerPrintCheckLevel": 5,
    "faceMatchThresholdN": 87,
    "faceQuality": 50,
    "faceRecogizeTimeOut": 3,
    "faceRecogizeInterval": 3,
    "cardReaderFunction": [
      "fingerPrint",
      "face"
    ],
    "cardReaderDescription": "DS-K1A340FWX",
    "livingBodyDetect": true,
    "faceMatchThreshold1": 60,
    "buzzerTime": 10,
    "envirMode": "other",
    "liveDetLevelSet": "low",
    "liveDetAntiAttackCntLimit": 5,
    "enableLiveDetAntiAttack": true,
    "fingerPrintCapacity": 3000,
    "fingerPrintNum": 1,
    "faceRecogizeEnable": 1
  }
}
```

## Reader 2

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/CardReaderCfg/2?format=json"
```

### Observed Response

Status: `200 OK`

```json
{
  "CardReaderCfg": {
    "enable": true,
    "okLedPolarity": "anode",
    "errorLedPolarity": "anode",
    "buzzerPolarity": "anode",
    "swipeInterval": 0,
    "pressTimeout": 10,
    "enableFailAlarm": false,
    "maxReadCardFailNum": 5,
    "enableTamperCheck": true,
    "offlineCheckTime": 0,
    "fingerPrintCheckLevel": 5,
    "faceMatchThresholdN": 87,
    "faceQuality": 50,
    "faceRecogizeTimeOut": 3,
    "faceRecogizeInterval": 3,
    "cardReaderFunction": [],
    "cardReaderDescription": "",
    "livingBodyDetect": true,
    "faceMatchThreshold1": 60,
    "buzzerTime": 10,
    "envirMode": "other",
    "liveDetLevelSet": "low",
    "liveDetAntiAttackCntLimit": 5,
    "enableLiveDetAntiAttack": true,
    "faceRecogizeEnable": 1
  }
}
```

`GET /ISAPI/AccessControl/CardReaderCfg/3?format=json` returned `404`.

## Face Recognize Mode

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/FaceRecognizeMode?format=json"
```

### Observed Response

```json
{
  "FaceRecognizeMode": {
    "mode": "normalMode"
  }
}
```

Capability:

```json
{
  "FaceRecognizeMode": {
    "mode": {
      "@opt": "normalMode"
    }
  }
}
```

## Integration Notes

- Reader 1 exposes the meaningful face/fingerprint functions.
- Reader 2 exists but has an empty `cardReaderFunction` list.
- The firmware uses the misspelled key `faceRecogize...`; integration code should match the exact spelling returned by the API.

