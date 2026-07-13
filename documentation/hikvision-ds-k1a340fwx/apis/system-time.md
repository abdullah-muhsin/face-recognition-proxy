# System Time

## Endpoints

- `GET /ISAPI/System/time`
- `GET /ISAPI/System/time/capabilities`

## Purpose

Reads current device time and the accepted time configuration schema.

## Request: Current Time

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/time"
```

## Observed Response: Current Time

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Time version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <timeMode>NTP</timeMode>
  <localTime>2026-07-13T19:14:08+03:00</localTime>
  <timeZone>CST-3:00:00</timeZone>
</Time>
```

## Request: Capabilities

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/time/capabilities"
```

## Observed Response: Capabilities

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Time version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <timeMode opt="NTP,manual"/>
  <localTime min="0" max="256"/>
  <timeZone min="0" max="256"/>
  <timeType opt="local,UTC"/>
</Time>
```

## Integration Notes

- Event timestamps are returned with the `+03:00` offset.
- The device was configured for `NTP` mode during testing.

