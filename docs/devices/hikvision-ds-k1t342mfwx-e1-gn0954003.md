# Hikvision DS-K1T342MFWX-E1 terminal `GN0954003`

This is the second integrated face-recognition terminal. Its identity was read
from the device through authenticated, read-only ISAPI endpoints on 2026-09-02.
The document contains operational identifiers only. Administrator and PushSDK
passwords belong exclusively in protected runtime secret files.

## Exact identity

| Field | Exact value | Source / use |
| --- | --- | --- |
| Model | `DS-K1T342MFWX-E1` | `GET /ISAPI/System/deviceInfo` |
| Gateway terminal serial | `DS-K1T342MFWX-E120260629V044840ENGN0954003` | `terminals.serial_number`; administration API terminal selector |
| PushSDK terminal ID | `GN0954003` | Device serial suffix; same PushSDK identifier format verified on the paired DS-K1T342MFWX-E1 integration |
| Device name | `Access Controller` | `GET /ISAPI/System/deviceInfo` |
| Device ID | `255` | `GET /ISAPI/System/deviceInfo`; distinct from the gateway terminal serial and PushSDK terminal ID |
| Device type | `ACS` / `accessControlTerminal` | `GET /ISAPI/System/deviceInfo` |
| Firmware | `V4.48.40`, build `260629` | `GET /ISAPI/System/deviceInfo` |
| Hardware | `V1.0.0` | `GET /ISAPI/System/deviceInfo` |
| Production date | `2026-01-06` | `GET /ISAPI/System/deviceInfo` |
| Ethernet MAC address | `88:DE:39:5D:B6:05` | `GET /ISAPI/System/deviceInfo` and network interface `1` |
| Wi-Fi MAC address | `0C:CD:B4:40:62:12` | `GET /ISAPI/System/Network/interfaces`, interface `2` |
| Local time zone | `Asia/Shanghai` (`UTC+08:00`) | `GET /ISAPI/System/time` |

Network addresses are intentionally not retained as terminal identity. Wi-Fi
allocation can change; the serial numbers and MAC addresses above are the
stable operational identifiers. The gateway does not make direct LAN
connections when operating the terminal. Its ISAPI commands are delivered by
the terminal's authenticated PushSDK session.

## PushSDK contract and seed

The terminal reported PushSDK disabled before enrolment. Its capability document
at `GET /ISAPI/System/Network/PUSHCfg/capabilities?format=json` confirms HTTPS,
secure WebSocket, hostname addressing, certified mode, a platform username of
at most 32 characters, and a password of at most 32 characters.

Configure the terminal only through
`PUT /ISAPI/System/Network/PUSHCfg?format=json` with this exact contract:

| Setting | Value |
| --- | --- |
| Enabled | `true` |
| Address format | `hostname` |
| Platform hostname | `vps.itplus.club` |
| Protocol | `https` |
| HTTPS port | `443` |
| Secure WebSocket port | `443` |
| Platform username | `attendance_gateway` |
| Platform password | the protected shared `PUSHSDK_TERMINAL_PASSWORD` value |

The terminal mapping must be present in the protected `terminals.json` before
PushSDK is enabled. At gateway startup, `SeedConfiguredTerminals` records the
canonical identity and begins it in `offline` state. A successful `AuthInfo`
and `Login` exchange is the only path to `online`.

This configuration was accepted by the terminal on 2026-09-02. It then
reported `online` and the gateway recorded `AuthInfo` and `Login` at security
version `4` in plaintext payload mode. That observed payload mode is retained
as protocol state; it is not converted to an encrypted mode by the gateway.

```json
{
  "serialNumber": "DS-K1T342MFWX-E120260629V044840ENGN0954003",
  "pushSdkSerial": "GN0954003",
  "username": "attendance_gateway",
  "passwordEnvironmentVariable": "PUSHSDK_TERMINAL_PASSWORD",
  "loginPasswordDigest": "sha256",
  "securityVersion": 4,
  "commandIntervalSeconds": 5,
  "errorDelaySeconds": 30
}
```

The mapping's login password digest is `sha256`. The initial `AuthInfo`
exchange must confirm security version `4`; this gateway does not guess,
normalize, or downgrade the negotiated contract. If the terminal instead
presents another exact identifier or negotiation value, stop enrolment and
correct the protected mapping before accepting events or commands.

## Verified device capabilities relevant to the gateway

The terminal's `GET /ISAPI/AccessControl/capabilities` response confirms
access-control event storage and search, remote door control, user and card
management, face capture, infrared face capture, face-recognition mode, and
device-event support. Once it is online, gateway operators can submit any
documented ISAPI command through the audited PushSDK command API; the command
remains queued until the terminal polls for it.

## Verified retained-event reconciliation contract

Authenticated read-only probes on 2026-09-03 confirmed the terminal's native
retained access-event API. The gateway uses this contract for the Terminal
Registry **Sync retained events** action; it does not automate or depend on the
device web-console endpoints.

| Contract element | Verified value |
| --- | --- |
| Search endpoint | `POST /ISAPI/AccessControl/AcsEvent?format=json` |
| Device-time endpoint | `GET /ISAPI/System/time` (documented `Time.localTime` XML) |
| Earliest accepted all-history start | `2000-01-01T00:00:00+08:00` |
| All-category selector | `major: 0`, `minor: 0` |
| Maximum page size | `30` |
| Maximum result position / retained total | `150000` |
| Pagination statuses observed | `MORE`, `OK`, `NO MATCH` |
| Event categories advertised | alarm `1`, exception `2`, operation `3`, event `5` |

The gateway first obtains the device `localTime` and uses that exact timestamp
as the end bound. It does not use a transient IP address, browser time, or host
time to define the archive search. It records the raw ISAPI page in the command
audit and retains each raw `InfoList` record separately. Since an `InfoList`
record has no PushSDK UUID and its `serialNo` can eventually wrap, deduplication
uses the exact record bytes' SHA-256 fingerprint within this terminal; the
numeric serial is retained as source context, not promoted to a global ID.

## Acceptance record

After the first online registration, the gateway delivered a read-only
`GET /ISAPI/System/deviceInfo` command through the public administration API.
The terminal completed it, and its retained response bytes confirmed model
`DS-K1T342MFWX-E1`, the serial above, and firmware `V4.48.40`. The terminal
omitted `dataFormat` from that command result; the gateway records that exact
absence (`responseDataFormatDeclared: false`) while retaining the response
bytes. It does not infer or rewrite a response format.
