# Hikvision DS-K1A340FWX ISAPI Documentation

Observed device: Hikvision `DS-K1A340FWX` Time and Attendance Terminal.

Test date: `2026-07-13`.

Observed access URL: `http://192.168.1.3`.

Authentication: HTTP Digest.

Use credential placeholders in all examples:

```bash
export ISAPI_BASE='http://192.168.1.3'
export ISAPI_USER='admin'
export ISAPI_PASS='********'
```

Do not hard-code the password in source code, logs, or documentation.

## Important Findings

- The device is an access-control/time-attendance terminal, not a normal IP camera.
- Full video streaming endpoints tested under `/ISAPI/Streaming/...` returned `404`.
- Access-control events are available through both polling (`/ISAPI/AccessControl/AcsEvent`) and a live multipart stream (`/ISAPI/Event/notification/alertStream`).
- User metadata is available. Raw face/fingerprint search endpoints tested on this firmware returned `notSupport`, even though enrolled user records expose `numOfFace` and `numOfFP`.
- Event records may include `pictureURL` values. Those URLs returned JPEG images when tested; binary images are intentionally not stored in this repo.
- The device advertises HTTPS on port `443`, and the port is open, but TLS handshaking failed from this WSL/curl environment. HTTP Digest on port `80` was used for all documented calls.
- `/ISAPI/System/Network/interfaces` reports interface IP `192.0.0.64`, while the tested reachable address was `192.168.1.3`. Treat these as separate facts until the network topology is confirmed.

## API Files

- [Capability summary](capability-summary.md)
- [System capabilities](apis/system-capabilities.md)
- [Device information](apis/system-device-info.md)
- [Time configuration](apis/system-time.md)
- [Network interfaces](apis/system-network-interfaces.md)
- [Algorithm information](apis/system-algorithms-info.md)
- [Platform services](apis/system-platform-services.md)
- [Security APIs](apis/security.md)
- [Event notification APIs](apis/events.md)
- [Access-control capabilities](apis/access-control-capabilities.md)
- [Users, cards, and biometric search behavior](apis/access-control-users-cards-biometrics.md)
- [Access-control event search](apis/access-control-events.md)
- [Access-control status and configuration](apis/access-control-status-config.md)
- [Card-reader and face-recognition configuration](apis/access-control-reader-face.md)
- [Attendance configuration](apis/access-control-attendance.md)
- [Local attendance rules and schemas](apis/access-control-local-attendance.md)
- [Image channels](apis/image-channels.md)
- [Video intercom related-address API](apis/video-intercom.md)
- [Picture URL media retrieval](apis/media-picture-url.md)
- [Tested unsupported or limited endpoints](apis/tested-unsupported-endpoints.md)

## External References Used For Probe Planning

- Hikvision ISAPI portal: `https://tpp.hikvision.com/download/ISAPI_OTAP`
- Hikvision product page: `https://www.hikvision.com/en/products/Access-Control-Products/Time-Attendance-Products/Value-Series/ds-k1a340fwx/`

The files in this directory are based on live responses from this exact device and firmware. External references were only used to choose safe read-only endpoints to test.
