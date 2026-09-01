# Hikvision Push SDK Gateway

This is the production-facing half of the Push SDK attendance demonstration. It receives the terminal's outbound HTTPS Push SDK requests, authenticates the terminal, persists each supported access event to a local SQLite outbox, and delivers the canonical record and received JPEG to the Laravel receiver on a private Docker network.

```text
DS-K1T342MFWX-E1 -- HTTPS/443 --> Nginx -- loopback --> gateway -- private Docker network --> Laravel receiver
```

It is deliberately not a terminal-management service. It supports one documented HTTP route shape only:

```text
POST /iot/{terminal-serial-number}/global/0-global/model/service/operate/PUSH/{AuthInfo,Login,CommandRequest,CommandResult,Event,Logout}
```

The `{terminal-serial-number}` segment must be the terminal's configured Push SDK route serial number. `SerialNumber` remains the canonical ISAPI identity delivered to Laravel; set `PushSdkSerialNumber` when the terminal uses a shorter protocol identifier. There is no implicit device-ID mapping, legacy `/PUSHSDK/` route, plaintext HTTP transport, unauthenticated event route, or password-digest fallback. HTTPS is mandatory. When a terminal elects Push SDK security version 3 or 4 for an interaction, the gateway requires that exact negotiated version and encrypts the corresponding response.

## What it guarantees

- The terminal session follows the Push SDK AuthInfo/Login/custom-auth challenge flow and serializes requests per terminal. An AuthInfo challenge is accepted once and only for three command intervals, as specified by the protocol.
- Only Push SDK security modes 3 and 4 are accepted for encrypted command/event payloads. The configured login digest is explicit; the DS-K1T342MFWX-E1 on `V4.48.40 build 260629` uses `sha256`. The vendor documents a SHA-1 exception for this model only on `V3.16.1 build 250320`.
- The gateway accepts active `AccessControllerEvent` notifications containing the fields required by the Laravel contract. Other valid Push SDK event types and documented `noData` notifications are recorded as ignored; malformed access events are rejected.
- An event is acknowledged to the terminal only after its deduplication key and delivery payload have committed to SQLite. The key is terminal serial number plus vendor event UUID.
- The delivery worker retries Laravel failures from the durable outbox. Once delivered, it removes the canonical payload and picture bytes from SQLite while retaining deduplication metadata for the configured retention period.
- A picture is forwarded only when the Push SDK event itself supplied one in a `boundaryData` JPEG part. The gateway never fetches the terminal's management URL or a `pictureURL` value.

## Configuration

Copy the two templates outside Git-tracked paths and protect both files with mode `600`:

```bash
install -d -m 700 /home/abdullah/services/secrets/attendance-pushsdk/gateway
install -d -m 700 /home/abdullah/services/runtime/attendance-pushsdk/gateway/data
cp apps/pushsdk-gateway/.env.example \
  /home/abdullah/services/secrets/attendance-pushsdk/gateway/runtime.env
cp apps/pushsdk-gateway/docker/gateway.json.example \
  /home/abdullah/services/secrets/attendance-pushsdk/gateway/gateway.json
chmod 600 /home/abdullah/services/secrets/attendance-pushsdk/gateway/runtime.env \
  /home/abdullah/services/secrets/attendance-pushsdk/gateway/gateway.json
```

Set the following values before starting the gateway:

- In `runtime.env`, set `PUSHSDK_TERMINAL_PASSWORD` to the exact Push SDK password configured on the terminal. This is a dedicated gateway credential, not the terminal's ISAPI administrator password. Set `ATTENDANCE_PUSH_GATEWAY_TOKEN` to the same 32+-character value used by the Laravel Push SDK receiver.
- In `gateway.json`, replace `REPLACE_WITH_THE_TERMINAL_SERIAL_NUMBER` with the `<serialNumber>` returned by `GET /ISAPI/System/deviceInfo` on the terminal's local management interface. Set `PushSdkSerialNumber` to the exact identifier observed in the device's Push SDK URL; omit it only when that URL uses the full ISAPI serial. Keep the username, password-variable name, and `LoginPasswordDigest` aligned with the terminal setting.
- Register that exact serial number in the Laravel receiver before the terminal sends any events:

```bash
docker exec attendance_receiver_pushsdk php artisan attendance:terminal:register TERMINAL_SERIAL_NUMBER \
  --name="Front entrance" \
  --model="DS-K1T342MFWX-E1" \
  --time-zone="Asia/Baghdad"
```

`DataDirectory` must remain `/var/lib/pushsdk-gateway` in the container. On the VPS, the SQLite outbox is bind-mounted from `/home/abdullah/services/runtime/attendance-pushsdk/gateway/data` and must be retained across releases. Local Compose uses its named `pushsdk_gateway_data` volume by default.

## Local Compose run

Start the Laravel Push SDK receiver first. Its Compose project creates the internal `attendance_pushsdk_internal` network and provides the `attendance_receiver_pushsdk` DNS alias used by the gateway configuration.

For a local Compose run, copy the example environment to this application directory, set its config-file path, and start the gateway:

```bash
cp apps/pushsdk-gateway/.env.example apps/pushsdk-gateway/.env.production
# Edit .env.production and set PUSHSDK_GATEWAY_CONFIG_FILE to an absolute gateway.json path.
docker compose --project-directory apps/pushsdk-gateway \
  --env-file apps/pushsdk-gateway/.env.production up --build -d
```

The Compose init service assigns the named volume to the unprivileged `pushsdk` account before the gateway starts. The gateway itself listens only on `127.0.0.1:8100`; it must never be published directly to the Internet.

Verify only from the VPS or local host:

```bash
curl --fail http://127.0.0.1:8100/healthz
docker compose --project-directory apps/pushsdk-gateway ps
```

## VPS release

Use the receiver release first, then deploy this gateway. The release commands default to the VPS `services/` hierarchy. The gateway release command verifies that secret files have mode `600`, preserves the bind-mounted outbox, creates the shared internal Docker network if it is missing, builds before stopping the active container, and retains the old container for rollback.

```bash
./scripts/release-production.sh \
  --receiver pushsdk

./scripts/release-pushsdk-gateway.sh
```

Use `--dry-run` before a production release. Do not use a “fresh database” operation for this gateway: its SQLite database is the durable delivery outbox.

## Public TLS edge

Point a DNS name such as `pushsdk.example.com` at the VPS, allow inbound TCP 80 and 443, obtain a publicly trusted certificate for that exact name, then install [the Nginx template](/home/magnet/services/face-recognition-proxy/deploy/nginx/pushsdk-gateway.conf). Replace every occurrence of `pushsdk.example.com` and its certificate paths before enabling it.

Nginx is the only process exposed on port 443. It injects both `X-Forwarded-Proto: https` and the terminal source IP/port used by the five-attempt login lock. The gateway rejects a request without those headers. The reverse proxy therefore must keep forwarding to the loopback-only port `127.0.0.1:8100`.

## DS-K1T342MFWX-E1 terminal setup

For the device running firmware `V4.48.40 build 260629`:

1. Ensure the terminal has correct time/NTP, a working DNS resolver, and an outbound route to the VPS on TCP 443. It must not need any inbound port forwarding or public ISAPI management port.
2. In the terminal's Push SDK configuration page, enter the gateway DNS name, `443`, the gateway Push SDK username, and the exact password stored in the protected gateway `runtime.env`. Select the HTTPS Push SDK transport and leave server-certificate verification enabled.
3. The terminal must trust the certificate served for the configured DNS name. Use a public CA certificate; do not deploy a self-signed certificate or disable certificate verification.
4. Save the Push SDK configuration and generate a face/card attendance event. The device constructs its Push SDK URL with its protocol serial; that value must match `Gateway:Terminals:PushSdkSerialNumber`. The full ISAPI serial in `Gateway:Terminals:SerialNumber` must match the Laravel terminal registration.

This gateway does not configure WebSocket/WSS, terminal commands, event subscriptions, a media server, or any terminal management API. Those are outside the attendance record flow.

## Acceptance checks

Run these checks in order:

1. `curl --fail http://127.0.0.1:8100/healthz` returns `{"status":"ok"}` on the VPS.
2. `sudo nginx -t` succeeds and `https://pushsdk.example.com/healthz` returns `404`; health is intentionally not public.
3. Gateway logs show AuthInfo followed by a successful configured-digest login when the terminal saves its Push SDK settings.
4. After an attendance event, gateway logs show delivery to the Laravel receiver and the public Laravel dashboard shows exactly one record. Re-sending the same vendor UUID must not create another record.
5. When the event includes a JPEG, the Laravel record reports its stored picture; when it does not, no picture upload request is attempted.

If the terminal receives a `404`, its Push SDK URL serial does not match `PushSdkSerialNumber` in `gateway.json`. An `Invalid SessionID` after AuthInfo indicates that the configured username, password, or explicit digest does not match the terminal. A `Push SDK traffic must arrive through the HTTPS reverse proxy` error means Nginx is not forwarding the required HTTPS header. Correct the configured value; do not add an alternate route or downgrade the transport.
