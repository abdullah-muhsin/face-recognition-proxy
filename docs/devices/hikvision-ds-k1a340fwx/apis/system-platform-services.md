# System Platform Services

## Covered Endpoints

- `GET /ISAPI/System/Network/EZVIZ`
- `GET /ISAPI/System/Network/EZVIZ/capabilities`
- `GET /ISAPI/System/Network/Ehome`
- `GET /ISAPI/System/Network/Ehome/capabilities`
- `GET /ISAPI/System/PictureServer`
- `GET /ISAPI/System/discoveryMode`

## EZVIZ / Hik-Connect Status

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/EZVIZ"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<EZVIZ version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
  <registerStatus>false</registerStatus>
  <serverAddress>
    <addressingFormatType>hostname</addressingFormatType>
    <hostName>litedev.hik-connect.com</hostName>
  </serverAddress>
  <verificationCode></verificationCode>
  <bindStatus>unbind</bindStatus>
</EZVIZ>
```

### Capability Response

```xml
<?xml version="1.0" encoding="UTF-8"?>
<EZVIZ version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled opt="true,false">false</enabled>
  <registerStatus opt="true,false">false</registerStatus>
  <serverAddress>
    <addressingFormatType opt="hostName">hostName</addressingFormatType>
    <hostName min="1" max="64">www.baidu.com</hostName>
  </serverAddress>
  <verificationCode min="6" max="12">null</verificationCode>
  <bindStatus opt="bind,unbind">bind</bindStatus>
</EZVIZ>
```

## EHome / ISUP-Style Platform Status

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/Ehome"
```

### Initial Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
  <addressingFormatType></addressingFormatType>
  <hostName></hostName>
  <ipAddress></ipAddress>
  <portNo>0</portNo>
  <deviceID></deviceID>
  <registerStatus>false</registerStatus>
  <protocolVersion></protocolVersion>
</Ehome>
```

### Capability Response

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled opt="true,false">false</enabled>
  <id min="1" max="1">1</id>
  <addressingFormatType opt="ipaddress,hostname"/>
  <hostName min="1" max="64"/>
  <ipAddress min="7" max="15"/>
  <portNo min="1024" max="65535"/>
  <deviceID min="1" max="32"/>
  <registerStatus opt="true,false"/>
  <key min="8" max="32"/>
  <protocolVersion opt="v5.0"/>
</Ehome>
```

### Writable ISUP Configuration Endpoint

The device web UI labels this platform access mode as `ISUP`. Its bundle builds an `Ehome` XML body and sends it to:

```http
PUT /ISAPI/System/Network/Ehome?centerID=1
```

Confirmed endpoint behavior:

| Probe | Result |
| --- | --- |
| `GET /ISAPI/System/Network/Ehome?centerID=1` | `200 OK`, same body as base EHome read. |
| `GET /ISAPI/System/Network/Ehome?centerID=2` | `404`, `invalidID`. |
| `GET /ISAPI/System/Network/Ehome/1` | `404`, `notSupport`. |
| `GET /ISAPI/System/Network/ISUP` | `404`, `notSupport`. |
| Empty `PUT /ISAPI/System/Network/Ehome` | `400`, `badXmlContent`; confirms `PUT` is the write method but XML is required. |
| `POST /ISAPI/System/Network/Ehome` | `400`, `methodNotAllowed`. |

Accepted enable body shape:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
  <deviceID>K1A340FWXPROBE1</deviceID>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>192.168.1.2</ipAddress>
  <portNo>7660</portNo>
  <key>********</key>
  <protocolVersion>v5.0</protocolVersion>
</Ehome>
```

The key was an 8-character temporary test key and is intentionally redacted. The key is accepted on write but is not returned by `GET`.

### ISUP Registration Probe

Test date/time: `2026-07-13`, around `21:11` to `21:13` Asia/Baghdad.

Temporary target:

- Listener host: `192.168.1.2`
- Listener port: `7660`
- Protocol surface: direct TCP listener, not HTTP

Observed result after enabling ISUP:

- `PUT /ISAPI/System/Network/Ehome?centerID=1` returned `200 OK`.
- Readback showed `enabled=true`, `ipAddress=192.168.1.2`, `portNo=7660`, `deviceID=K1A340FWXPROBE1`, `protocolVersion=v5.0`, and `registerStatus=false`.
- The listener accepted repeated TCP connections from `192.168.1.6`.
- Most first reads were `89` bytes. Some reads only captured the first `2` bytes before the terminal closed/retried.
- The first payload began with hex `10 57 01 01 00 09`.
- The ASCII-visible parts of the binary payload included the serial suffix `J59360966`, model `DS-K1A340FWX`, configured device ID `K1A340FWXPROBE1`, and the serial suffix again.

Example captured first payload:

```text
HEX   10 57 01 01 00 09 4A 35 39 33 36 30 39 36 36 0C 44 53 2D 4B 31 41 33 34 30 46 57 58 29 2C 0F 4B 31 41 33 34 30 46 57 58 50 52 4F 42 45 31 ...
ASCII .W....J59360966.DS-K1A340FWX),.K1A340FWXPROBE1...
```

Interpretation:

- The device does support outbound ISUP/EHome v5 registration attempts to a configured server.
- ISUP is not HTTP and cannot be received directly by a Laravel route.
- A raw TCP listener proves connection and packet identity only. It is not a working ISUP platform because it does not complete Hikvision's binary registration/authentication handshake.
- `registerStatus` stayed `false`, as expected without a real ISUP server.

### Restore Behavior After ISUP Probe

The original factory-empty state included `portNo=0`. After the first write, the firmware would not accept `portNo=0` through `PUT`:

```xml
<subStatusCode>portError</subStatusCode>
<errorMsg>portError</errorMsg>
```

Blank/omitted fields are accepted while disabled but mostly ignored. The final verified disabled state after cleanup was:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Ehome version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
  <addressingFormatType>ipaddress</addressingFormatType>
  <ipAddress>0.0.0.0</ipAddress>
  <portNo>1024</portNo>
  <deviceID>0</deviceID>
  <registerStatus>false</registerStatus>
  <protocolVersion>v5.0</protocolVersion>
</Ehome>
```

This is disabled and non-routable, but it is not byte-for-byte the original factory-empty EHome state.

### Official ISUP Context

Hikvision documentation identifies `7660` as the default ISUP device registration port. The same Hikvision WAN access documentation lists additional ISUP ports commonly used by full platforms: TCP alarm receiving `7332`, UDP alarm receiving `7334`, streaming via VAG `7661`, and plugin streaming `16000`.

Hikvision's Open Capabilities page describes Device Gateway as the component for adding and managing ISUP-enabled devices, including ISUP5.0/ISAPI access-control devices, for third-party platform integration.

References:

- `https://enpinfo.hikvision.com/unzip/20200731103027_05278_doc/GUID-E3A713A8-BC04-4E31-A287-C6E990E2F070.html`
- `https://tpp.hikvision.com/tpp/OpenCapabilities`
- `https://www.hikvision.com/content/dam/hikvision/en/support/how-to/how-to-document/access-control/DS-K1T-Series-EHome-Configuration-in-New-IVMS-4200.pdf`

## Picture Server

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/PictureServer"
```

### Observed Response

Status: `200 OK`

Content-Type was reported as XML in one probe and JSON with `?format=json`; the body was JSON.

```json
{
  "PictureServerInformation": {
    "cloudStorage": {
      "cloudManageHttpPort": 1024,
      "cloudTransDataPort": 1024,
      "cloudCmdPort": 1024,
      "cloudHeartBeatPort": 1024,
      "cloudStorageHttpPort": 1024,
      "cloudUsername": "",
      "cloudPassword": "",
      "cloudPoolId": 1,
      "clouldProtocolVersion": "",
      "clouldAccessKey": "",
      "clouldSecretKey": ""
    },
    "pictureServerType": "cloudStorage",
    "addressingFormatType": "hostname",
    "hostName": "",
    "portNo": 1024
  }
}
```

## Discovery Mode

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/discoveryMode"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<DiscoveryMode version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <PcapMode>nonDiscoverable</PcapMode>
  <SADP>discoverable</SADP>
</DiscoveryMode>
```

## Integration Notes

- EZVIZ is supported but disabled/unregistered.
- EHome/ISUP v5 configuration is supported and outbound ISUP registration attempts were confirmed.
- ISUP is a binary platform protocol, not an HTTP webhook target. A cloud Laravel app needs a real ISUP platform/gateway in front of it, then an explicit handoff from that gateway to Laravel.
- Picture server settings are present but empty.
- SADP discovery is enabled.
