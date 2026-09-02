# Hikvision DS-K1T342MFWX-E1 terminal record

This record identifies the currently integrated face-recognition terminal. It
contains operational identifiers only; credentials remain in the protected
runtime environment and are never committed.

| Field | Exact value | Source / use |
| --- | --- | --- |
| Model | `DS-K1T342MFWX-E1` | Device model |
| Gateway terminal serial | `DS-K1T342MFWX-E120260629V044840ENGN0953967` | `terminals.serial_number`; administration API terminal selector |
| PushSDK terminal ID | `GN0953967` | PushSDK URL segment and `terminals.pushsdk_serial` |
| LAN IPv4 address | `10.203.216.162` | Observed device event source; reference only, never a gateway delivery target |
| MAC address | `88:de:39:5d:b5:e1` | Observed `AccessControllerEvent.macAddress` |
| Access-controller channel | `1` | Observed `AccessControllerEvent.channelID` |
| Device event name | `Access Controller` | Observed `AccessControllerEvent.deviceName` |
| PushSDK protocol | HTTPS and secure WebSocket on public port `443` | Terminal connects to the gateway; the gateway does not connect back to the LAN address |
| PushSDK security version | `4` | Required terminal mapping value |
| Login password digest | `sha256` | Required terminal mapping value for this firmware integration |
| Command poll interval | `5` seconds | Required terminal mapping value |
| Error delay | `30` seconds | Required terminal mapping value |

## Declarative terminal seed

At each gateway startup, `SeedConfiguredTerminals` upserts every terminal in
the protected `terminals.json` mapping. For this device, the mapping must use
the two exact identifiers above and the protected PushSDK service account:

```json
{
  "serialNumber": "DS-K1T342MFWX-E120260629V044840ENGN0953967",
  "pushSdkSerial": "GN0953967",
  "username": "<exact PushSDK username configured on the terminal>",
  "passwordEnvironmentVariable": "PUSHSDK_TERMINAL_PASSWORD",
  "loginPasswordDigest": "sha256",
  "securityVersion": 4,
  "commandIntervalSeconds": 5,
  "errorDelaySeconds": 30
}
```

The password value belongs only in the protected environment file named by
`passwordEnvironmentVariable`. Do not add a device password, administrator
password, or a direct-device ISAPI URL to this repository.

The seed establishes the canonical database identity and resets the current
connection state to `offline`; a valid PushSDK session is the only way to mark
the terminal online. Existing terminal rows not present in the current
configuration are retained because event and command audit records reference
their serial numbers.
