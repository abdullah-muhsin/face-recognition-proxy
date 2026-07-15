# Security APIs

## Covered Endpoints

- `GET /ISAPI/Security/capabilities`
- `GET /ISAPI/Security/users`
- `GET /ISAPI/Security/users/1`
- `GET /ISAPI/Security/onlineUser`
- `GET /ISAPI/Security/adminAccesses`
- `GET /ISAPI/Security/illegalLoginLock`
- `GET /ISAPI/Security/userCheck`

## Capabilities

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Security/capabilities"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SecurityCap version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <supportUserNums>1</supportUserNums>
  <userBondIpNums>1</userBondIpNums>
  <userBondMacNums>1</userBondMacNums>
  <isSupCertificate>false</isSupCertificate>
  <issupIllegalLoginLock>true</issupIllegalLoginLock>
  <isSupportOnlineUser>true</isSupportOnlineUser>
  <isSupportAnonymous>false</isSupportAnonymous>
  <isSupportStreamEncryption>false</isSupportStreamEncryption>
  <securityVersion opt="1"/>
  <keyIterateNum>100</keyIterateNum>
  <isSupportUserCheck>true</isSupportUserCheck>
  <RSAKeyLength opt="1024" def="1024"/>
  <isSupportONVIFUserManagement>false</isSupportONVIFUserManagement>
  <WebCertificateCap>
    <CertificateType opt="digest">digest</CertificateType>
    <SecurityAlgorithm>
      <algorithmType opt="MD5,SHA256,MD5/SHA256">MD5</algorithmType>
    </SecurityAlgorithm>
  </WebCertificateCap>
  <isSupportConfigFileImport>true</isSupportConfigFileImport>
  <isSupportConfigFileExport>true</isSupportConfigFileExport>
  <isSupportSecurityQuestionConfig>true</isSupportSecurityQuestionConfig>
  <isSupportSecurityEmail>true</isSupportSecurityEmail>
  <SecurityLimits>
    <LoginPasswordLenLimit min="8" max="16">8</LoginPasswordLenLimit>
    <SecurityAnswerLenLimit min="1" max="128"/>
  </SecurityLimits>
  <cfgFileSecretKeyLenLimit min="0" max="32"/>
  <isIrreversible>true</isIrreversible>
  <salt>2F1C2DE6D8D243EF46F177D830796D51CC10733BEC81DC0CAABE837942351A92</salt>
  <isSupportDeviceCertificatesManagement>true</isSupportDeviceCertificatesManagement>
  <isSupportRTSPCertificate opt="true,false">true</isSupportRTSPCertificate>
  <isSupportEncryptCertificate>true</isSupportEncryptCertificate>
  <isSupportCertificateCustomID>true</isSupportCertificateCustomID>
  <isSupportSecurityEmail>true</isSupportSecurityEmail>
</SecurityCap>
```

## Users

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Security/users"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<UserList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <User>
    <id>1</id>
    <userName>admin</userName>
    <userLevel>Administrator</userLevel>
  </User>
</UserList>
```

`GET /ISAPI/Security/users/1` returned the same single user without the list wrapper.

## Online User

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Security/onlineUser"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<OnlineUserList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <OnlineUser>
    <id>65536</id>
    <name>admin</name>
    <type>admin</type>
    <loginTime>2026-07-13 19:03:51</loginTime>
    <clientAddress>
      <ipAddress>192.168.1.2</ipAddress>
    </clientAddress>
  </OnlineUser>
</OnlineUserList>
```

## Admin Access Protocols

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Security/adminAccesses"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<AdminAccessProtocolList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <AdminAccessProtocol>
    <id>1</id>
    <enabled>true</enabled>
    <protocol>HTTP</protocol>
    <portNo>80</portNo>
  </AdminAccessProtocol>
  <AdminAccessProtocol>
    <id>2</id>
    <protocol>DEV_MANAGE</protocol>
    <portNo>8000</portNo>
  </AdminAccessProtocol>
  <AdminAccessProtocol>
    <id>4</id>
    <enabled>true</enabled>
    <protocol>HTTPS</protocol>
    <portNo>443</portNo>
  </AdminAccessProtocol>
</AdminAccessProtocolList>
```

Port sanity check from the test machine:

```text
80/tcp    open
443/tcp   open
8000/tcp  open
```

HTTPS note: `curl -k --digest ... https://192.168.1.200/...` failed during TLS handshake from this environment. See [network services and SSH](system-network-services.md) for the fuller port-level scan, including RTSP, SSH, and TCP `8443`.

## Illegal Login Lock

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Security/illegalLoginLock"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<IllegalLoginLock version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
  <maxIllegalLoginTimes>5</maxIllegalLoginTimes>
  <maxIllegalLoginLockTime>1800</maxIllegalLoginLockTime>
</IllegalLoginLock>
```

## User Check

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Security/userCheck"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<userCheck version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <statusValue>200</statusValue>
  <statusString>OK</statusString>
  <isDefaultPassword>false</isDefaultPassword>
  <isRiskPassword>false</isRiskPassword>
  <isActivated>true</isActivated>
</userCheck>
```

## Integration Notes

- Use Digest auth.
- Only one API/security account is exposed by `/ISAPI/Security/users`.
- Config import/export is advertised but not tested because exports may contain sensitive configuration.
