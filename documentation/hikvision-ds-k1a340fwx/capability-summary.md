# Capability Summary

This summary separates verified live behavior from feature flags that were advertised but not successfully exercised.

## Confirmed Working APIs

| Area | Confirmed behavior |
| --- | --- |
| Device identity | Model, firmware, serial number, MAC address, hardware/BSP/DSP versions are available from `/ISAPI/System/deviceInfo`. |
| Time | Current time and time configuration capabilities are available. Device was in NTP mode with `+03:00` timestamps. |
| Network | Interface and link details are available, but reported IP differs from the observed reachable URL. |
| Security | Digest authentication works. Admin account list, online user, illegal-login lock, user-check, and admin access ports are readable. |
| Access-control users | User count and user search work. One user was enrolled at test time. User records include validity, door rights, group, and face/fingerprint/card counts. |
| Cards | Card count and card search work. Count was zero at test time. |
| Events | Access-control event search, event total count, HTTP notification configuration, subscription capability, and live alert stream work. |
| Event pictures | Event `pictureURL` values return JPEG images with Digest auth. One tested image was `768x432`, `44409` bytes. |
| Reader config | Card-reader configs for reader `1` and `2` are readable. Reader `1` exposes face and fingerprint functions. |
| Face recognition | Face recognition mode is readable and set to `normalMode`. Reader config exposes thresholds, liveness detection, and anti-attack settings. |
| Attendance | Attendance status keys, weekly attendance plans, plan templates, and local attendance rule are readable. |
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
| Local attendance groups | Capability range `1..128` |
| Local attendance shifts | Capability range `1..256` |
| Local attendance week plans/templates | Capability range `1..640` |

## Advertised But Not Fully Verified

| Feature | Result |
| --- | --- |
| HTTPS | Port `443` is open and advertised, but TLS handshake failed from the test client. |
| Raw face/fingerprint search | Tested search endpoints returned `notSupport`. |
| Full video streaming | `/ISAPI/Streaming/...` endpoints returned `404`. |
| Door config/control | Door config support is advertised, but common read paths returned `404`; remote door control is not supported by capability flag. |
| Local attendance searches | Capability flags advertise search, but tested `Search` requests returned `invalidID`. Capability schemas are readable. |
| SNAP/deploy/identity endpoints | Some returned `200` with empty body. |
| Write operations | Create/update/delete/reset/clear/firmware/control operations were intentionally not executed. |

## Practical Integration Shape

For an attendance integration, the safest confirmed flow is:

1. Poll `/ISAPI/AccessControl/AcsEventTotalNum?format=json` for total count or use serial/time windows.
2. Page `/ISAPI/AccessControl/AcsEvent?format=json` with `maxResults <= 30`.
3. Read `employeeNoString`, `name`, `time`, `currentVerifyMode`, `attendanceStatus`, `statusValue`, and optional `pictureURL`.
4. Fetch `pictureURL` only when an image is required.
5. Use `/ISAPI/AccessControl/UserInfo/Search?format=json` to map employee records and validity windows.
6. Use `/ISAPI/Event/notification/alertStream` for live event streaming if long-running multipart parsing is acceptable.

