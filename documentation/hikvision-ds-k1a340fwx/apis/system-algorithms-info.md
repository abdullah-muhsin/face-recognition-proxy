# System Algorithm Information

## Endpoints

- `GET /ISAPI/System/AlgorithmsInfo`
- `GET /ISAPI/System/algorithmsInfo`

Both case variants responded with the same JSON payload.

## Purpose

Reads installed algorithm package metadata for face recognition, intelligent processing, calibration, and liveness detection.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/AlgorithmsInfo?format=json"
```

## Observed Response

Status: `200 OK`

Content-Type: `application/json`

```json
{
  "ALgorithmsInfo": [
    {
      "type": "face",
      "name": "DFRv4.3.7_NT98525_Linux32bit_INT8_SafeChip_build20201013_release.zip",
      "versionInfo": {
        "major": 4,
        "minor": 3,
        "revision": 7
      },
      "platform": "NT98525",
      "os": "Linux32bit",
      "accuracy": "INT8",
      "encryption": "SafeChip",
      "buildTime": "build20201013",
      "versionProperties": "release.zip"
    },
    {
      "type": "intelligent",
      "versionInfo": {
        "major": 1,
        "minor": 4,
        "revision": 3
      },
      "buildTime": "201111"
    },
    {
      "type": "calibration",
      "versionInfo": {
        "major": 110,
        "svnVersion": -1
      },
      "buildTime": "191210"
    },
    {
      "type": "living",
      "versionInfo": {
        "major": 2,
        "minor": 1,
        "revision": 8,
        "svnVersion": 310773
      },
      "buildTime": "2021-05-14,14:23:16"
    }
  ]
}
```

## Integration Notes

- The expected `/ISAPI/Intelligent/algorithmInfo` path returned `404`; use `/ISAPI/System/AlgorithmsInfo`.
- Liveness support is also reflected by card-reader config fields such as `livingBodyDetect` and `liveDetLevelSet`.

