# Network Interfaces

## Endpoints

- `GET /ISAPI/System/Network/interfaces`
- `GET /ISAPI/System/Network/interfaces/1`

## Purpose

Reads network interface configuration and link properties.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/interfaces"
```

## Observed Response

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<NetworkInterfaceList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <NetworkInterface>
    <id>1</id>
    <IPAddress>
      <ipVersion>v4</ipVersion>
      <addressingType>static</addressingType>
      <ipAddress>192.0.0.64</ipAddress>
      <subnetMask>255.255.255.0</subnetMask>
      <DefaultGateway>
        <ipAddress>192.0.0.1</ipAddress>
      </DefaultGateway>
      <PrimaryDNS>
        <ipAddress>8.8.8.8</ipAddress>
      </PrimaryDNS>
      <SecondaryDNS>
        <ipAddress>8.8.4.4</ipAddress>
      </SecondaryDNS>
    </IPAddress>
    <Link>
      <MACAddress>40:24:b2:e7:46:04</MACAddress>
      <autoNegotiation>true</autoNegotiation>
      <speed>1000</speed>
      <duplex>full</duplex>
      <MTU>1500</MTU>
    </Link>
  </NetworkInterface>
</NetworkInterfaceList>
```

## Integration Notes

- The endpoint reports `192.0.0.64`, but the reachable test URL was `192.168.1.3`.
- `GET /ISAPI/System/Network/interfaces/1` returned the same interface data without the list wrapper.
- Common endpoints for ports, default route, routes, hostname, wireless, and NTP server details returned `404` on this firmware.

