# Device Information

## Endpoint

`GET /ISAPI/System/deviceInfo`

## Purpose

Returns model, firmware, hardware, MAC address, serial number, and device type.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/deviceInfo"
```

## Observed Response

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<DeviceInfo version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <deviceName>Time and Attendance Terminal</deviceName>
  <deviceID>255</deviceID>
  <deviceDescription>ACS</deviceDescription>
  <deviceLocation>hangzhou</deviceLocation>
  <systemContact>Hikvision.China</systemContact>
  <model>DS-K1A340FWX</model>
  <serialNumber>DS-K1A340FWX20240102V010207ENJ59360966</serialNumber>
  <macAddress>40:24:b2:e7:46:04</macAddress>
  <firmwareVersion>V1.2.7</firmwareVersion>
  <firmwareReleasedDate>build 240102</firmwareReleasedDate>
  <bootVersion>16908295</bootVersion>
  <bootReleasedDate></bootReleasedDate>
  <hardwareVersion>0x10000</hardwareVersion>
  <deviceType>ACS</deviceType>
  <telecontrolID>255</telecontrolID>
  <supportBeep>false</supportBeep>
  <supportVideoLoss>true</supportVideoLoss>
  <alarmOutNum>0</alarmOutNum>
  <RS485Num>0</RS485Num>
  <bspVersion>1.1.1.654629 build 2023-12-29,13:56:15</bspVersion>
  <dspVersion>1.1.0.569665 build 2023-05-31,13:45:23</dspVersion>
  <OEMCode>1</OEMCode>
  <customizedInfo></customizedInfo>
  <marketType>1</marketType>
</DeviceInfo>
```

## Integration Notes

- `deviceType` is `ACS`, confirming the access-control/time-attendance focus.
- `alarmOutNum` and `RS485Num` are zero, matching the missing I/O proxy endpoints tested later.

