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
and enabled flag. There is no activation timestamp or occurrence cutoff. URLs with embedded
credentials, fragments or a non-HTTPS scheme are rejected. Changes are retained
with their administrative actor in gateway activity. Endpoint/key
changes are rejected while that terminal has pending or sending deliveries.
Each delivery retains its original endpoint, key ID and exact body independently
of later configuration changes. Retries never redirect an old event to a new
school.

Only newly inserted live, classified AccessControllerEvent records with
`majorEventType=5`, `subEventType=75` and `eventState=active` qualify while the destination is enabled, including delayed historical scans
received after the terminal reconnects. A qualifying source
missing required identity/time data remains explicit and fails receiver
validation; the gateway does not invent those values. A raw event, projection,
delivery record and gateway activity commit together before PushSDK success.
A failed delivery INSERT rolls the entire event batch back. Vendor retries
produce one source event and one delivery.

Imported ISAPI retained events and existing archive records are not automatically
queued. Use the explicit historical backfill preview and queue operation. Pausing a destination stops new collection and worker claims. An HTTP
request already in flight may finish. Re-enabling resumes its existing queued
records; events received during the pause remain archive-only.

## Worker and receipt

The worker runs outside PushSDK request handling. PostgreSQL leases and
`FOR UPDATE SKIP LOCKED` protect claims. A 60-second lease recovers work after a
crash; result writes verify the attempt generation so an expired worker cannot
complete another worker's claim. HTTP has a 10-second deadline, uses normal TLS
verification, and follows no redirects.

The version-2 JSON message has exactly `schemaVersion`, `source`, `eventId`,
`terminalSerialNumber`, `pushSdkSerial`, `majorEventType`, `subEventType`,
`eventState`, `occurredAt`, `employeeNoString`, `employeeName`, and
`eventSerialNumber`. `source` is exactly `pushsdk` or `isapi`. ISAPI messages
use the original source SHA-256 in hexadecimal as `eventId` and an explicit null
`eventState`; an active state is never invented for an archive record. Existing
version-1 messages retain their original bytes and finish using matching receipts.
New messages always use version 2. It includes no pictures, raw source body or credentials.
The original vendor event identifier is opaque and preserved exactly. Person
identifiers remain exact strings. The declared occurrence instant retains its
precision and explicit timezone representation from the database projection.

Requests include `X-PushSDK-Key-Id`, current Unix seconds as
`X-PushSDK-Timestamp`, and `X-PushSDK-Signature`. The signature is lowercase
hexadecimal HMAC-SHA256 using the configured key string over
`timestamp + "\n" + exact_request_body`. Each attempt signs the same stored body
with a fresh request timestamp.

Only HTTP 200 and the exact versioned receipt
`{"schemaVersion":2,"eventId":"<matching identifier>","receipt":"stored"}`
(with the message's exact schema version)
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
  `signingKeyId`, `enabled`
- `GET /api/v1/admin/event-deliveries?limit=25&offset=0`
- `POST /api/v1/admin/event-deliveries/{id}/retry` for failed deliveries

For GN0953967 and GN0954003, configure
`https://creative.itplus.club/api/integrations/pushsdk/events`. Register both
terminal serials and verified teacher mappings in the Creative Minds tenant
before enabling the destination. Restore the terminal connection and synchronize
its clock, then verify a fresh scan in both applications. The archive's previous
person names are not evidence of a current Creative Minds teacher assignment.

## Historical backfill

`POST /api/v1/admin/terminals/{serial}/backfill/preview` accepts `startsAt` and
`endsAt` (RFC3339, known offsets, past range, at most 31 days; end exclusive).
The result lists the destination, exact identities/names, original scan dates,
eligible and excluded counts, and a preview token. No records are changed.
A request containing over 10000 source records requires a smaller range.

`POST /api/v1/admin/terminals/{serial}/backfill/queue` takes that exact range and
`previewToken`. It recomputes the preview under the terminal lock, rejects changed
archives/destinations/queue eligibility, and inserts the entire audited batch and
outbox in one transaction. Concurrent submissions cannot double-queue records.
Already queued records remain in their existing delivery state; failed deliveries
use Retry instead of backfill. Cross-source copies are recognized only when the
non-null event serial, exact scan instant, person ID and name match a valid
PushSDK representative. ISAPI-only records retain their own source identity.
The school deduplicates by terminal/source/event ID and daily attendance.

`DELETE /api/v1/admin/terminals/{serial}/delivery` removes a destination only when
no unfinished deliveries remain. Source archives, completed deliveries and gateway
activity are preserved. An audit entry records the administrator.

The incorrect GN0953979 Creative Minds destination must be removed; its device
records remain in the gateway archive and are not included in Creative Minds
backfills. Teacher mappings belong to the school platform and are never guessed
by the gateway or derived from numeric ID equality across devices.
