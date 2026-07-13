# Capability Summary

This summary separates verified live behavior from feature flags that were advertised but not successfully exercised.

## Confirmed Working APIs

| Area | Confirmed behavior |
| --- | --- |
| Device identity | Model, firmware, serial number, MAC address, hardware/BSP/DSP versions are available from `/ISAPI/System/deviceInfo`. |
| Time | Current time, time capabilities, time zone, and NTP server configuration are available. Device was in NTP mode with `+03:00` timestamps. |
| Network | Wired interface `1`, Wi-Fi interface `2`, Wi-Fi status, access-point scan, SSH state/control, and exposed TCP services are documented. Interface `2` reports the observed reachable IP `192.168.1.3`. |
| Security | Digest authentication works. Admin account list, online user, illegal-login lock, user-check, and admin access ports are readable. |
| Access-control users | User count and user search work. One user was enrolled at test time. User records include validity, door rights, group, and face/fingerprint/card counts. |
| Cards | Card count and card search work. Count was zero at test time. |
| Events | Access-control event search, event total count, HTTP notification host read/write, subscription capability, and live alert stream work. HTTP attendance/access-controller delivery to a configured host was not observed. |
| Event pictures | Event `pictureURL` values return JPEG images with Digest auth. One tested image was `768x432`, `44409` bytes. |
| Reader config | Card-reader configs for reader `1` and `2` are readable. Reader `1` exposes face and fingerprint functions. |
| Face recognition | Face recognition mode is readable and set to `normalMode`. Reader config exposes thresholds, liveness detection, and anti-attack settings. |
| Attendance | Attendance status keys, attendance mode, weekly attendance plans, plan templates, verification plans, local attendance rule, and local attendance object searches are readable. |
| Verification TTS | Verification TTS and holiday TTS plan configuration are readable; TTS is currently disabled. |
| Audio/language | Device language and audio output volume are readable. |
| Image settings | Image channels `1` and `2` are readable. |
| Platform services | EZVIZ, EHome, picture server, and discovery mode are readable. EZVIZ/EHome were disabled or unregistered. |
| Video intercom related address | Related address configuration is readable but unconfigured. |

## Confirmed Counts And Capacities

| Item | Observed value |
| --- | --- |
| Enrolled users | `1` |
| Cards | `0` |
| Stored access-control events | `68` |
| Fingerprint capacity | `3000` from reader config |
| Current fingerprint count on reader 1 | `1` |
| Attendance plan numbers | `1..3` |
| Attendance template numbers | `1..3` |
| Local attendance groups | Capability range `1..128`; currently verified groups `2..7` |
| Local attendance shifts | Capability range `1..256`; currently verified shifts `2..3` |
| Local attendance week plans/templates | Capability range `1..640`; currently verified week plans/templates `2..7` |
| Wi-Fi interfaces | Interface `2` enabled and connected during testing |
| Open TCP ports after SSH enable | `22`, `80`, `443`, `554`, `8000`, `8443` |
| Audio outputs | `1`, volume `5` |

## Advertised But Not Fully Verified

| Feature | Result |
| --- | --- |
| HTTPS | Port `443` is open and advertised, but TLS handshake failed from the test client. |
| HTTPS-alt | Port `8443` is open, but protocol behavior is not confirmed as ISAPI HTTPS. |
| RTSP | Port `554` is open and challenges with RTSP Digest/Basic auth, but no media path was confirmed. |
| SSH shell | Port `22` exposes Dropbear after ISAPI enablement. Manual login reached a protected BusyBox shell; shell command surface was not fully explored. |
| Raw face/fingerprint search | Tested search endpoints returned `notSupport`. |
| Full video streaming | `/ISAPI/Streaming/...` endpoints returned `404`. |
| Door config/control | Door config support is advertised, but common read paths returned `404`; remote door control is not supported by capability flag. |
| SNAP/deploy/identity endpoints | Some returned `200` with empty body. |
| Generic UI bundle endpoints | The web UI lists many generic endpoints that returned `notSupport` on this device. |
| Direct HTTP event push | HTTP notification host configuration is writable, but AccessControllerEvent trigger binding was not exposed and no outbound HTTP request was observed during a live access-controller event. |
| Write operations | User/card/fingerprint/local-attendance write method validation was tested. Destructive reset/clear/firmware/control operations were not executed. See [device state change log](device-state-change-log.md). |

## Practical Integration Shape

For an attendance integration, the safest confirmed flow is:

1. Poll `/ISAPI/AccessControl/AcsEventTotalNum?format=json` for total count or use serial/time windows.
2. Page `/ISAPI/AccessControl/AcsEvent?format=json` with `maxResults <= 30`.
3. Read `employeeNoString`, `name`, `time`, `currentVerifyMode`, `attendanceStatus`, `statusValue`, and optional `pictureURL`.
4. Fetch `pictureURL` only when an image is required.
5. Use `/ISAPI/AccessControl/UserInfo/Search?format=json` to map employee records and validity windows.
6. Use `/ISAPI/Event/notification/alertStream` for live event streaming if long-running multipart parsing is acceptable.

Do not treat `/ISAPI/Event/notification/httpHosts` as a confirmed direct attendance webhook mechanism on this firmware.
