# School attendance event delivery

Event delivery is an explicit, optional integration owned by the gateway. School
ownership, teacher mappings and attendance policy belong to the receiving school
application. An administrator configures one HTTPS destination per terminal in
Event delivery. No destination is inferred from terminal names, person names or
network addresses.

## Configuration and capture

`DELIVERY_SIGNING_KEYS` in the protected gateway environment is a JSON object of
key IDs to exactly 64 lowercase hexadecimal signing-key strings. Key IDs contain
1–64 letters, digits, underscores or hyphens. `{}` disables destination creation.
The console returns key IDs only. It never accepts or exposes secret key values.
The receiving tenant must configure the same key ID and secret.

A destination contains its full terminal serial, exact HTTPS URL, key ID,
explicit RFC3339 activation instant and enabled flag. URLs with embedded
credentials, fragments or a non-HTTPS scheme are rejected. Changes are retained
with their administrative actor in gateway activity. Endpoint/key/activation
changes are rejected while that terminal has pending or sending deliveries.
Each delivery retains its original endpoint, key ID and exact body independently
of later configuration changes. Retries never redirect an old event to a new
school.

Only newly inserted live, classified AccessControllerEvent records with
`majorEventType=5`, `subEventType=75` and `eventState=active` qualify. Both receipt
and occurrence must be at or after destination activation. A qualifying source
missing required identity/time data remains explicit and fails receiver
validation; the gateway does not invent those values. A raw event, projection,
delivery record and gateway activity commit together before PushSDK success.
A failed delivery INSERT rolls the entire event batch back. Vendor retries
produce one source event and one delivery.

Imported ISAPI retained events and existing archive records are not automatically
queued. Pausing a destination stops new collection and worker claims. An HTTP
request already in flight may finish. Re-enabling resumes its existing queued
records; events received during the pause remain archive-only.

## Worker and receipt

The worker runs outside PushSDK request handling. PostgreSQL leases and
`FOR UPDATE SKIP LOCKED` protect claims. A 60-second lease recovers work after a
crash; result writes verify the attempt generation so an expired worker cannot
complete another worker's claim. HTTP has a 10-second deadline, uses normal TLS
verification, and follows no redirects.

The version-1 JSON message has exactly `schemaVersion`, `eventId`,
`terminalSerialNumber`, `pushSdkSerial`, `majorEventType`, `subEventType`,
`eventState`, `occurredAt`, `employeeNoString`, `employeeName`, and
`eventSerialNumber`. It includes no pictures, raw source body or credentials.
The original vendor event identifier is opaque and preserved exactly. Person
identifiers remain exact strings. The declared occurrence instant retains its
precision and explicit timezone representation from the database projection.

Requests include `X-PushSDK-Key-Id`, current Unix seconds as
`X-PushSDK-Timestamp`, and `X-PushSDK-Signature`. The signature is lowercase
hexadecimal HMAC-SHA256 using the configured key string over
`timestamp + "\n" + exact_request_body`. Each attempt signs the same stored body
with a fresh request timestamp.

Only HTTP 200 and the exact versioned receipt
`{"schemaVersion":1,"eventId":"<matching identifier>","receipt":"stored"}`
complete delivery. Unknown receipt fields, trailing JSON and a different event
ID are rejected. Delivery means the school durably received the event; its
attendance audit may report unmapped identity, clock error or attendance conflict.

Network errors, HTTP 408, 429 and 5xx retry after 5, 10, 20, 40, 80, 160, then
300 seconds, continuing at the 300-second cap. Other responses and invalid
receipts fail visibly for administrator correction and an explicit Retry action.
Response bodies and networking errors are never copied into activity or delivery
logs. The console shows endpoint, state, attempts, HTTP result, safe failure code
and schedule. The receiver must deduplicate terminal/event IDs: a lost receipt
can cause the same committed event to be submitted again.

## API

All administration endpoints use the existing signed-in administrator session:

- `GET /api/v1/admin/delivery-configuration`
- `PUT /api/v1/admin/terminals/{serial}/delivery` with exactly `endpointUrl`,
  `signingKeyId`, `enabledFrom`, `enabled`
- `GET /api/v1/admin/event-deliveries?limit=25&offset=0`
- `POST /api/v1/admin/event-deliveries/{id}/retry` for failed deliveries

For GN0953979, configure
`https://creative.itplus.club/api/integrations/pushsdk/events`. Register both
terminal serials and verified teacher mappings in the Creative Minds tenant
before enabling the destination. Restore the terminal connection and synchronize
its clock, then verify a fresh scan in both applications. The archive's previous
person names are not evidence of a current Creative Minds teacher assignment.
