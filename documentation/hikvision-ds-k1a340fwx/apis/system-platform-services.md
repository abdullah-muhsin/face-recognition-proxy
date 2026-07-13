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

### Observed Response

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

- EZVIZ and EHome are supported but disabled/unregistered.
- Picture server settings are present but empty.
- SADP discovery is enabled.

