# Tested Unsupported Or Limited Endpoints

This file records endpoints that were tested and did not produce a useful readable payload. This is useful because Hikvision endpoint availability varies by model and firmware.

## System And Network

| Endpoint | Method | Observed result | Notes |
| --- | --- | --- | --- |
| `/ISAPI/System/status` | `GET` | `404` | Not exposed on this firmware. |
| `/ISAPI/System/Network/ports` | `GET` | `404` | Use `/ISAPI/Security/adminAccesses` for port information. |
| `/ISAPI/System/Network/defaultRoute` | `GET` | `404` | Not exposed. |
| `/ISAPI/System/Network/routes` | `GET` | `404` | Not exposed. |
| `/ISAPI/System/Network/hostname` | `GET` | `404` | Not exposed. |
| `/ISAPI/System/Network/wireless` | `GET` | `404` | Wrong path for this firmware. Use `/ISAPI/System/Network/interfaces/2/wireless`. |
| `/ISAPI/System/Network/wireless/capabilities` | `GET` | `404` | Wrong path for this firmware. Use `/ISAPI/System/Network/interfaces/2/wireless/capabilities`. |
| `/ISAPI/System/Network/NTP` | `GET` | `404` | Wrong path for this firmware. Use `/ISAPI/System/time/ntpServers/1`. |
| `/ISAPI/System/Network/https/capabilities` | `GET` | `404` | HTTPS is advertised through security/admin-access endpoints. |
| `/ISAPI/System/SNAPConfig` | `GET` | `404` | `AccessControl/SNAPConfig` exists but returned empty body. |
| `/ISAPI/System/ClientProxyWEB` | `GET` | `404` | Capability flag advertised, common endpoint not exposed. |

## HTTPS

| Test | Result |
| --- | --- |
| TCP connect to `192.168.1.3:443` | Open |
| `curl -k --digest https://192.168.1.3/ISAPI/System/deviceInfo` | TLS handshake failure |
| `curl --tlsv1.2 --tls-max 1.2` | TLS handshake failure |
| `curl --tlsv1.0` / `--tlsv1.1` | Local OpenSSL/curl reported protocols unavailable |

Conclusion: HTTPS is configured/open, but it was not usable from the test client without further TLS/cipher investigation.

## Streaming And Camera-Style APIs

| Endpoint | Method | Observed result | Notes |
| --- | --- | --- | --- |
| `/ISAPI/Streaming/channels` | `GET` | `404` | No normal streaming channel list. |
| `/ISAPI/Streaming/channels/1` | `GET` | `404` | Not exposed. |
| `/ISAPI/Streaming/channels/101` | `GET` | `404` | Not exposed. |
| `/ISAPI/Streaming/channels/1/picture` | `GET` | `404` | Snapshot path not exposed. |
| `/ISAPI/Streaming/channels/101/picture` | `GET` | `404` | Snapshot path not exposed. |
| `/ISAPI/Video/inputs/channels` | `GET` | `404` | Not exposed. |
| `/ISAPI/Audio/channels` | `GET` | `404` | Not exposed despite one audio output in top-level capabilities. |
| `/ISAPI/System/USB` | `GET` | `404` | Not exposed through this path. |
| `/ISAPI/System/Hardware` | `GET` | `404` | Not exposed through this path. |

## Intelligent / Face Library Paths

| Endpoint | Method | Observed result | Notes |
| --- | --- | --- | --- |
| `/ISAPI/Intelligent/capabilities` | `GET` | `404` | Not exposed. |
| `/ISAPI/Intelligent/algorithmInfo` | `GET` | `404` | Use `/ISAPI/System/AlgorithmsInfo`. |
| `/ISAPI/Intelligent/FDLib` | `GET` | `404` | Not exposed. |
| `/ISAPI/Intelligent/FDLib/capabilities` | `GET` | `404` | Not exposed. |
| `/ISAPI/Intelligent/FDLib/FDSearch?format=json` | `GET` | `404` | Not exposed. |
| `/ISAPI/Intelligent/FDLib/FaceDataRecord?format=json` | `GET` | `404` | Not exposed. |

## Access-Control User/Card/Biometric APIs

| Endpoint | Method | Observed result | Notes |
| --- | --- | --- | --- |
| `/ISAPI/AccessControl/UserInfo/Search?format=json` | `GET` | `404` | Works with `POST`. |
| `/ISAPI/AccessControl/CardInfo/Search?format=json` | `GET` | `404` | Works with `POST`; returned no matches. |
| `/ISAPI/AccessControl/FaceInfo/Search?format=json` | `POST` | `notSupport` | Raw face search unavailable through this tested API. |
| `/ISAPI/AccessControl/FingerPrint/Search?format=json` | `POST` | `notSupport` | Raw fingerprint search unavailable through this tested API. |
| `/ISAPI/AccessControl/FingerPrintInfo/Search?format=json` | `POST` | `notSupport` | Raw fingerprint info search unavailable through this tested API. |
| `/ISAPI/AccessControl/FaceInfo/Count?format=json` | `GET` | `404` | Not exposed. |
| `/ISAPI/AccessControl/FingerPrint/Count?format=json` | `GET` | `404` | Not exposed. |

## Access-Control Optional Feature APIs

| Endpoint | Method | Observed result | Notes |
| --- | --- | --- | --- |
| `/ISAPI/AccessControl/DoorCfg/1?format=json` | `GET` | `404` | Door config is advertised but common endpoint did not respond. |
| `/ISAPI/AccessControl/Door/1?format=json` | `GET` | `404` | Not exposed. |
| `/ISAPI/AccessControl/InputProxy/channels?format=json` | `GET` | `404` | No input proxy exposed. |
| `/ISAPI/AccessControl/OutputProxy/channels?format=json` | `GET` | `404` | No output proxy exposed. |
| `/ISAPI/AccessControl/AlarmOut/channels?format=json` | `GET` | `404` | Device info reports zero alarm outputs. |
| `/ISAPI/AccessControl/TTSText?format=json` | `GET` | `404` | Wrong path. Use `/ISAPI/AccessControl/Verification/ttsText?format=json`. |
| `/ISAPI/AccessControl/AttendanceMode?format=json` | `GET` | `404` | Wrong path. Use `/ISAPI/AccessControl/Configuration/attendanceMode?format=json`. |
| `/ISAPI/AccessControl/AttendanceStatusPlan?format=json` | `GET` | `404` | Not exposed. Attendance mode works through `/Configuration/attendanceMode`. |
| `/ISAPI/AccessControl/CardDisplayCfg?format=json` | `GET` | `404` | Listed by web UI bundle but not supported by this device. |
| `/ISAPI/AccessControl/readerList?format=json` | `GET` | `404` | Listed by web UI bundle but not supported by this device. |
| `/ISAPI/AccessControl/CardVerificationRule?format=json` | `GET` | `404` | Listed by web UI bundle but not supported by this device. |
| `/ISAPI/AccessControl/Configuration/NFCCfg?format=json` | `GET` | `404` | Not supported. |
| `/ISAPI/AccessControl/Configuration/RFCardCfg?format=json` | `GET` | `404` | Not supported. |
| `/ISAPI/AccessControl/WiegandCfg/capabilities` | `GET` | `404` | Not supported on this model/firmware. |
| `/ISAPI/AccessControl/maskDetection?format=json` | `GET` | `404` | Not supported on this model/firmware. |
| `/ISAPI/AccessControl/temperatureMeasureCfg?format=json` | `GET` | `404` | Not supported on this model/firmware. |
| `/ISAPI/AccessControl/visitorParamCfg?format=json` | `GET` | `404` | Not supported on this model/firmware. |
| `/ISAPI/AccessControl/SNAPConfig?format=json` | `GET` | `200`, empty body | Endpoint exists but returned no readable payload. |
| `/ISAPI/AccessControl/DeployInfo?format=json` | `GET` | `200`, empty body | Endpoint exists but returned no readable payload. |
| `/ISAPI/AccessControl/IdentityTerminal?format=json` | `GET` | `200`, empty body | Endpoint exists but returned no readable payload. |

## Local Attendance Wrong-Path Search APIs

The following guessed upper-case search paths returned `400 invalidID`. They are not the correct paths for this firmware. Use the lower-case paths documented in [local attendance](access-control-local-attendance.md).

- `/ISAPI/AccessControl/LocalAttendance/Group/Search?format=json`
- `/ISAPI/AccessControl/LocalAttendance/Shift/Search?format=json`
- `/ISAPI/AccessControl/LocalAttendance/HolidayPlan/Search?format=json`
- `/ISAPI/AccessControl/LocalAttendance/HolidayGroup/Search?format=json`
- `/ISAPI/AccessControl/LocalAttendance/WeekPlan/Search?format=json`
- `/ISAPI/AccessControl/LocalAttendance/PlanTemplate/Search?format=json`
- `/ISAPI/AccessControl/LocalAttendance/GroupShift/Search?format=json`

Observed error:

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

## Security And RACM

| Endpoint | Method | Observed result | Notes |
| --- | --- | --- | --- |
| `/ISAPI/Security/users/admin` | `GET` | `404` | User ID path works: `/users/1`. |
| `/ISAPI/Security/sessionLogin/capabilities` | `GET` | `404` | Digest auth works directly. |
| `/ISAPI/Security/securityQuestion` | `GET` | `404` | Capability flag exists, endpoint not exposed by this path. |
| `/ISAPI/Security/securityEmail` | `GET` | `404` | Capability flag exists, endpoint not exposed by this path. |
| `/ISAPI/Racm/SecurityLog/logServer` | `GET` | `404` | Security log server flag exists, common endpoint not exposed. |
