# Picture URL Media Retrieval

## Endpoint Pattern

Event records may include URLs like:

```text
http://192.168.1.200/LOCALS/pic/acsLinkCap/...jpeg@WEB...
```

These are not `/ISAPI/...` paths, but they are returned by ISAPI event search and are part of the integration surface for verification/capture images.

## Purpose

Retrieves JPEG images linked from access-control events.

## Request

Use the exact `pictureURL` returned by `/ISAPI/AccessControl/AcsEvent`.

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  -o event-picture.jpg \
  'http://192.168.1.200/LOCALS/pic/acsLinkCap/[redacted].jpeg@[redacted]'
```

## Observed Response Metadata

A real event `pictureURL` was tested without storing the image in the repo.

```text
HTTP status: 200
Content-Type: image/jpeg
Downloaded bytes: 44409
Image type: JPEG image data, baseline, 768x432, 3 components
```

## Integration Notes

- Treat these images as sensitive biometric/attendance evidence.
- Store only when business rules require it.
- The URL includes event-specific path/token data. Do not log it unnecessarily.
- Digest authentication worked for the tested image URL.

