# Network Interfaces

## Endpoints

- `GET /ISAPI/System/Network/interfaces`
- `GET /ISAPI/System/Network/interfaces/1`
- `GET /ISAPI/System/Network/interfaces/1/ipAddress`
- `GET /ISAPI/System/Network/interfaces/1/link`
- `GET /ISAPI/System/Network/interfaces/2/ipAddress`

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

- Interface `1` reports wired/static address `192.0.0.64`.
- Interface `2` is the Wi-Fi interface and reports dynamic address `192.168.1.200`, which was the reachable test URL.
- `GET /ISAPI/System/Network/interfaces/1` returned the same interface data without the list wrapper.
- `GET /ISAPI/System/Network/interfaces/1/ipAddress` and `/link` returned the IP and link subdocuments.
- Common endpoints for ports, default route, routes, hostname, and top-level wireless returned `404` on this firmware.
- See [network and Wi-Fi](system-network-wireless.md) for interface `2`, Wi-Fi status, and access-point scan.
- See [network services and SSH](system-network-services.md) for exposed TCP ports, SSH state/control, RTSP, HTTPS, and DEV_MANAGE behavior.
