# Video Intercom Related Address

## Endpoint

`GET /ISAPI/VideoIntercom/relatedDeviceAddress`

## Purpose

Reads video-intercom/SIP/center management address settings. The top-level system capability advertises related device address support.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/VideoIntercom/relatedDeviceAddress"
```

## Observed Response

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RelatedDeviceAddress version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <unitType>outdoor</unitType>
  <SIPServerAddress>
    <addressingFormatType>ipaddress</addressingFormatType>
    <ipAddress>0.0.0.0</ipAddress>
  </SIPServerAddress>
  <centerPort>0</centerPort>
  <CenterAddress>
    <addressingFormatType>ipaddress</addressingFormatType>
    <ipAddress>0.0.0.0</ipAddress>
  </CenterAddress>
  <ManageAddress>
    <addressingFormatType>ipaddress</addressingFormatType>
    <ipAddress>0.0.0.0</ipAddress>
  </ManageAddress>
</RelatedDeviceAddress>
```

## Integration Notes

- The settings are present but effectively unconfigured (`0.0.0.0`, port `0`).
- `GET /ISAPI/VideoIntercom/capabilities` returned `404`; use the top-level `/ISAPI/System/capabilities` for video-intercom capability flags.

