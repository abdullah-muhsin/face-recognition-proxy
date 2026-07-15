# Network Services And SSH

## Covered Probes And Endpoints

- `nmap -sV` service scan
- `GET /ISAPI/System/Network/ssh`
- `GET /ISAPI/System/Network/ssh/capabilities`
- `PUT /ISAPI/System/Network/ssh`
- RTSP authentication probe on TCP `554`
- HTTPS handshake probes on TCP `443` and `8443`
- DEV_MANAGE/admin-service probe on TCP `8000`

## TCP Service Profile

### Request

```bash
nmap -sV -p22,80,443,554,8000,8443 192.168.1.200
```

### Observed Response

Status: command completed successfully.

```text
PORT     STATE SERVICE        VERSION
22/tcp   open  ssh            Dropbear sshd 2022.83 (protocol 2.0)
80/tcp   open  http
443/tcp  open  ssl/https
554/tcp  open  rtsp
8000/tcp open  http-alt?
8443/tcp open  ssl/https-alt?
Service Info: OS: Linux; CPE: cpe:/o:linux:linux_kernel
```

Before SSH was enabled, a top-1000 `nmap -sV 192.168.1.200` scan showed only:

```text
80/tcp   open  http
443/tcp  open  ssl/https
554/tcp  open  rtsp
8000/tcp open  http-alt?
8443/tcp open  ssl/https-alt?
```

`nmap -O` OS fingerprinting was not performed because the client command was not run with root privileges.

## SSH State

### Read Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/ssh"
```

### Initial Observed Response

SSH was initially disabled:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
</SSH>
```

### Capability Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Network/ssh/capabilities"
```

### Capability Response

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled opt="true,false">false</enabled>
</SSH>
```

The `opt="true,false"` attribute confirms that SSH can be toggled. The element value should be treated as the default or current value at the time of that capability response, not as a read-only limitation.

## Enable SSH

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -X PUT \
  -H 'Content-Type: application/xml' \
  --data '<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
</SSH>' \
  "$ISAPI_BASE/ISAPI/System/Network/ssh"
```

### Observed Response

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ResponseStatus version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <requestURL></requestURL>
  <statusCode>1</statusCode>
  <statusString>OK</statusString>
  <subStatusCode>ok</subStatusCode>
</ResponseStatus>
```

### Post-Enable Read Response

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>true</enabled>
</SSH>
```

After this write, TCP `22` opened and identified as:

```text
22/tcp open ssh Dropbear sshd 2022.83 (protocol 2.0)
```

### Disable SSH

The same endpoint should disable SSH when sent with `false`. This was not executed after the observed enable operation.

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -X PUT \
  -H 'Content-Type: application/xml' \
  --data '<?xml version="1.0" encoding="UTF-8"?>
<SSH version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <enabled>false</enabled>
</SSH>' \
  "$ISAPI_BASE/ISAPI/System/Network/ssh"
```

## SSH Login Behavior

Manual SSH tests supplied by the operator showed:

```text
ssh admin@192.168.1.200
Permission denied

ssh root@192.168.1.200
BusyBox v1.31.1 (2023-12-29 13:56:52 CST) built-in shell (ash)
BusyBox v1.2.1 Protect Shell (psh svn326548) Build Time: Mar 17 2021:10:03:53
```

The protected shell `help` output listed:

```text
getHardInfo
help
Debug
sandbox
```

The operator transcript also showed that `exit` and `quit` were not accepted by the protected shell.

Do not assume the HTTP Digest `admin` account maps to a usable SSH login. The transcript showed `admin` SSH login denied while `root` reached the protected shell after a password was supplied.

The current SSH credential mapping is recorded in the git-ignored local file `../credentials.local.md`.

## HTTP Port 80

Port `80` is the confirmed working management and ISAPI endpoint. It serves the Angular web UI root page and accepts HTTP Digest authentication for `/ISAPI/...`.

`nmap` reported HTTP response headers similar to:

```text
HTTP/1.0 200 OK
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
X-XSS-Protection: 1; mode=block
Content-Type: text/html
```

## HTTPS Port 443

Port `443` is open and is advertised as HTTPS by `/ISAPI/Security/adminAccesses`.

### Probe

```bash
curl -k -I --connect-timeout 5 https://192.168.1.200/
```

### Observed Result

```text
curl: (35) TLS connect error: error:0A000410:SSL routines::ssl/tls alert handshake failure
```

Conclusion: the service is open, but it was not usable from this curl/OpenSSL client without additional TLS/cipher investigation.

## RTSP Port 554

Port `554` is open and speaks RTSP.

### Probe

```bash
curl -I --connect-timeout 5 rtsp://192.168.1.200/
```

### Observed Response

Dynamic nonce/random values are redacted.

```text
RTSP/1.0 401 Unauthorized
CSeq: 1
WWW-Authenticate: Digest realm="DS-d59740cc", nonce="[dynamic]", random="[dynamic]", stale="FALSE"
WWW-Authenticate: Basic realm="/"
```

Plain HTTP-style `GET` probes returned:

```text
RTSP/1.0 405 Method Not Allowed
```

RTSP authentication is exposed, but normal camera-style ISAPI streaming paths tested elsewhere returned `404`. No RTSP media path was confirmed in this documentation pass.

## DEV_MANAGE / Admin Service Port 8000

`/ISAPI/Security/adminAccesses` advertises protocol `DEV_MANAGE` on TCP `8000`. `nmap` labeled it `http-alt?`, but a normal HTTP probe did not produce normal HTTP semantics.

### Probe

```bash
curl -I --connect-timeout 5 http://192.168.1.200:8000/
```

### Observed Result

```text
curl: (1) Received HTTP/0.9 when not allowed
```

Treat this as the Hikvision device-management service, not as an ISAPI REST endpoint.

## HTTPS-Alt Port 8443

Port `8443` is open and `nmap` labels it `ssl/https-alt?`, but it did not behave like a normal HTTPS web endpoint from this client.

### Probe

```bash
curl -k -I --connect-timeout 5 https://192.168.1.200:8443/
```

### Observed Result

```text
curl: (35) TLS connect error: error:0A000410:SSL routines::ssl/tls alert handshake failure
```

`nmap` reported a fixed 16-byte binary response for many probe types:

```text
00 00 00 10 00 00 00 20 00 00 00 20 00 00 00 00
```

Do not treat TCP `8443` as a confirmed ISAPI HTTPS endpoint.

## Security Notes

- Keep SSH disabled unless actively needed for maintenance.
- SSH enablement is a real device state change. Record when it is changed and confirm the final state with `GET /ISAPI/System/Network/ssh`.
- Port `80` is the only confirmed fully usable ISAPI management surface in this documentation pass.
- Port `443` is advertised and open, but not curl-usable from this test client.
- Port `554` exposes RTSP authentication, but no usable media path was confirmed.
- Port `8000` should be reserved for Hikvision tooling or SDK integrations, not generic HTTP clients.
- Port `8443` is open but protocol behavior remains unidentified.
