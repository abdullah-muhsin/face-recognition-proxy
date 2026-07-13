# Network And Wi-Fi

## Covered Endpoints

- `GET /ISAPI/System/Network/capabilities`
- `GET /ISAPI/System/Network/interfaces/2/capabilities`
- `GET /ISAPI/System/Network/interfaces/2/ipAddress`
- `GET /ISAPI/System/Network/interfaces/2/wireless`
- `GET /ISAPI/System/Network/interfaces/2/wireless/capabilities`
- `GET /ISAPI/System/Network/interfaces/2/wireless/connectStatus`
- `GET /ISAPI/System/Network/interfaces/2/wireless/accessPointList`
- `GET /ISAPI/System/Network/ssh`

## Network Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/capabilities"
```

### Observed Response

Status: `200 OK`

```xml
<NetworkCap version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <isSupportWireless>true</isSupportWireless>
  <isSupportPPPoE>false</isSupportPPPoE>
  <isSupportBond>false</isSupportBond>
  <isSupport802_1x>false</isSupport802_1x>
  <isSupportNtp>true</isSupportNtp>
  <isSupportFtp>false</isSupportFtp>
  <isSupportHttps>true</isSupportHttps>
  <SnmpCap>
    <isSupport>false</isSupport>
  </SnmpCap>
  <isSupportSSH opt="true,false">true</isSupportSSH>
  <isSupportEZVIZ>true</isSupportEZVIZ>
  <isSupportEhome>true</isSupportEhome>
  <isSupportWirelessServer>false</isSupportWirelessServer>
  <isSupportWebSocket>true</isSupportWebSocket>
  <isSupportWebSocketS>true</isSupportWebSocketS>
  <isSupportEZVIZQRcode>true</isSupportEZVIZQRcode>
  <isSupportEZVIZUnbind>true</isSupportEZVIZUnbind>
</NetworkCap>
```

## Wi-Fi Interface

The working Wi-Fi path is interface `2`. The top-level paths such as `/ISAPI/System/Network/wireless` returned `404 notSupport`.

### Capability Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/interfaces/2/capabilities"
```

### Capability Response

```xml
<NetworkInterface version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>2</id>
  <IPAddress>
    <ipVersion opt="v4">v4</ipVersion>
    <addressingType opt="dynamic,static"/>
    <DNSEnable opt="true,false">false</DNSEnable>
  </IPAddress>
  <Wireless>
    <enabled opt="true,false">false</enabled>
    <ssid min="1" max="32"/>
    <WirelessSecurity>
      <securityMode opt="disable,WPA-personal,WPA2-personal"/>
      <WPA>
        <sharedKey min="8" max="63"/>
        <wpaKeyLength min="8" max="63"/>
      </WPA>
    </WirelessSecurity>
    <isSupportConnectStatus>true</isSupportConnectStatus>
  </Wireless>
</NetworkInterface>
```

### IP Address Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/interfaces/2/ipAddress"
```

### IP Address Response

```xml
<IPAddress version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <ipVersion>v4</ipVersion>
  <addressingType>dynamic</addressingType>
  <ipAddress>192.168.1.3</ipAddress>
  <subnetMask>255.255.255.0</subnetMask>
  <DefaultGateway>
    <ipAddress>192.168.1.1</ipAddress>
  </DefaultGateway>
  <PrimaryDNS>
    <ipAddress>8.8.8.8</ipAddress>
  </PrimaryDNS>
  <SecondaryDNS>
    <ipAddress>8.8.4.4</ipAddress>
  </SecondaryDNS>
  <DNSEnable>false</DNSEnable>
</IPAddress>
```

### Wireless Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/interfaces/2/wireless"
```

### Wireless Response

Sensitive Wi-Fi material is redacted.

```xml
<Wireless version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
  <channel>auto</channel>
  <ssid>[redacted]</ssid>
  <WirelessSecurity>
    <securityMode>WPA-personal</securityMode>
    <WPA>
      <algorithmType>AES</algorithmType>
      <sharedKey>[redacted]</sharedKey>
      <wpaKeyLength>8</wpaKeyLength>
    </WPA>
  </WirelessSecurity>
</Wireless>
```

### Connect Status Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/interfaces/2/wireless/connectStatus"
```

### Connect Status Response

```xml
<WirelessConnectStatus version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
  <status>success</status>
  <ssid>[redacted]</ssid>
  <ipAddress>192.168.1.3</ipAddress>
</WirelessConnectStatus>
```

### Access Point Scan

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/interfaces/2/wireless/accessPointList"
```

Observed response contained four nearby access points. SSIDs are redacted in this documentation; the connected AP had `signalStrength` `100`, `securityMode` `WPA2-personal`, and `connected` `true`.

## SSH

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/ssh"
```

### Observed Response

```xml
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
</SSH>
```

## Integration Notes

- Interface `1` is the wired/static interface and reports `192.0.0.64`.
- Interface `2` is the Wi-Fi interface and reports the observed reachable address `192.168.1.3`.
- Do not store the `sharedKey` returned by the Wi-Fi API in application logs or documentation.
- SSH is supported by capability but disabled.
