# Event Notification APIs

## Covered Endpoints

- `GET /ISAPI/Event/triggers`
- `GET /ISAPI/Event/notification/httpHosts`
- `GET /ISAPI/Event/notification/httpHosts/1`
- `GET /ISAPI/Event/notification/subscribeEventCap`
- `GET /ISAPI/Event/notification/alertStream`

## Event Triggers

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Event/triggers"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<EventTriggerList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <EventTrigger>
    <id>vmd-1</id>
    <eventType>VMD</eventType>
    <EventTriggerNotificationList/>
  </EventTrigger>
</EventTriggerList>
```

## HTTP Notification Host

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Event/notification/httpHosts"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<HttpHostNotificationList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <HttpHostNotification>
    <id>1</id>
    <url></url>
    <protocolType>HTTP</protocolType>
    <parameterFormatType>XML</parameterFormatType>
    <addressingFormatType>ipaddress</addressingFormatType>
    <ipAddress>0.0.0.0</ipAddress>
    <portNo>0</portNo>
    <httpAuthenticationMethod>none</httpAuthenticationMethod>
  </HttpHostNotification>
</HttpHostNotificationList>
```

`GET /ISAPI/Event/notification/httpHosts/1` returned the same host without the list wrapper.

## Subscribe Event Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Event/notification/subscribeEventCap"
```

### Observed Response

Status: `200 OK`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SubscribeEventCap version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <heartbeat min="1" max="30"/>
  <channelMode opt="all,list"/>
  <eventMode opt="all,list"/>
  <EventList>
    <Event>
      <type>AccessControllerEvent</type>
      <minorAlarm>0x404,0x405,0x406,0x407,0x408,0x409,0x40a,0x40b,0x40c</minorAlarm>
      <minorException>0x27,0x400,0x407,0x409,0x40a,0x428,0x429</minorException>
      <minorOperation>0x5a,0x5d,0x5e,0x70,0x71,0x79,0x7a,0x7b,0x7e,0x86,0x87,0x400,0x401,0x402,0x403,0x404,0x405,0x406,0x407,0x40a,0x40b,0x40c,0x40e,0x419,0x41a,0x42f,0x430,0x431,0x432,0x433,0x2600,0x2601</minorOperation>
      <minorEvent>0x1,0x6,0x7,0x8,0x9,0x11,0x12,0x13,0x14,0x15,0x16,0x17,0x18,0x19,0x1a,0x1b,0x1c,0x1d,0x1e,0x1f,0x20,0x26,0x27,0x31,0x33,0x4b,0x4c,0x51,0x52,0x68,0x97,0x98,0x9b,0x99,0x9a</minorEvent>
      <pictureURLType opt="binary" def=""/>
    </Event>
  </EventList>
</SubscribeEventCap>
```

## Alert Stream

### Request

The stream is long-running. Use a client-side timeout for diagnostics.

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  --max-time 5 \
  "$ISAPI_BASE/ISAPI/Event/notification/alertStream"
```

### Observed Response

Status: `200 OK`

Content-Type: `multipart/mixed; boundary=MIME_boundary`

The stream returned multipart JSON parts. A short sanitized sample:

```text
--MIME_boundary
Content-Type: application/json; charset="UTF-8"
Content-Length: 242

{
  "ipAddress": "192.0.0.64",
  "portNo": 80,
  "protocol": "HTTP",
  "channelID": 3,
  "dateTime": "2026-07-13T22:19:01+03:00",
  "activePostCount": 1,
  "eventType": "videoloss",
  "eventState": "inactive",
  "eventDescription": "videoloss alarm"
}

--MIME_boundary
Content-Type: application/json; charset="UTF-8"
Content-Length: 533

{
  "ipAddress": "192.0.0.64",
  "portNo": 80,
  "protocol": "HTTP",
  "dateTime": "2026-07-06T00:33:53+03:00",
  "activePostCount": 1,
  "eventType": "AccessControllerEvent",
  "eventState": "active",
  "eventDescription": "Access Controller Event",
  "AccessControllerEvent": {
    "deviceName": "Time and Attendance Terminal",
    "majorEventType": 2,
    "subEventType": 1024,
    "reportChannel": 3,
    "serialNo": 1,
    "attendanceStatus": "undefined",
    "statusValue": 0,
    "currentEvent": false,
    "frontSerialNo": 0,
    "picturesNumber": 0
  }
}
```

## Integration Notes

- Polling event history and consuming the live alert stream are both viable.
- The HTTP notification host is currently empty. Configure it only through deliberate `PUT`/`POST` operations; no write calls were executed during this documentation pass.
- Stream consumers must parse multipart boundaries and individual JSON parts.

