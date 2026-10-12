# Terminal connection diagnosis

`pushsdk.session_expired` means authenticated traffic stopped for the session's
validity window. It does not identify why the terminal stopped contacting the
gateway. Expiry changes gateway state; it does not send a disconnect or restart
instruction to the device. A terminal initiates PushSDK HTTP requests, including
reauthentication and command polling. The gateway cannot initiate an ISAPI
exchange when that path is unavailable.

## Evidence to collect

1. Compare each terminal's last successful exchange with its expiry activity.
   Use server receipt times, not event timestamps from the device clock.
2. Check `/var/log/nginx/pushsdk-gateway-access.log` for subsequent `AuthInfo`,
   `Login`, rejected requests, or requests from a different source address.
3. Check `/var/log/nginx/pushsdk-gateway-transport.log` for connection IDs,
   requests per connection, completion, HTTP status, and upstream timings.
   It records only `/iot/` transport metadata, without bodies, authentication
   headers, or query strings. The host's `/var/log/nginx/*.log` rotation must
   cover it. An HTTP 200 alone does not prove the terminal processed a response;
   `completion` is Nginx's request-completion field, not a device acknowledgement.
4. Check gateway JSON logs, including stdout, and the dedicated Nginx error log.
   Readiness is `http://127.0.0.1:18080/readyz` on the VPS; the public route is
   intentionally unavailable.
5. If HTTP requests stop, check established TCP connections and use a bounded
   packet-header observation for the last known source IP. No traffic from an
   old IP alone does not exclude a changed NAT address; also search by terminal
   ID across the access logs. Do not retain payloads or authentication headers.
6. Correlate with the terminal's own PushSDK status/error, uptime, and the
   school's router WAN, DNS, DHCP and power history. Server-side silence cannot
   distinguish a terminal process stall from a network or power outage.

Do not treat longer inactivity timeouts as a repair for missing traffic, ignore
proofs, restart healthy gateway processes periodically, or claim that unit tests
prove firmware recovery. After
any repair, verify actual `AuthInfo`/`Login`, sustained polling and a new event
from both physical terminals. Then verify recovery after a controlled connection
interruption when school operations permit it.

## Investigation: 12 October 2026

All times below use Asia/Baghdad. Creative Minds terminals are `GN0953967` and
`GN0954003`; `GN0953979` belongs outside this tenant.

| Observation | GN0953967 | GN0954003 |
| --- | --- | --- |
| Last exchange on 12 October | 09:09:46, CommandResult, HTTP 200 | 09:09:47, CommandRequest, HTTP 200 |
| Session expiry | 09:10:03 | 09:10:03 |
| Shared interruption on 11 October | 10:17:17–11:29:23 | 10:17:19–11:29:17 |
| First request after that gap | AuthInfo, HTTP 200 | AuthInfo, HTTP 200 |
| Last exchange on 7 October | 10:04:34, HTTP 200 | 10:04:33, HTTP 200 |

The 12 October investigation found no further terminal HTTP requests, no
established TCP connection to their last shared public address, and no packets
to/from that address on ports 80/443 during a 40-second observation. The gateway
was ready and had not restarted since 6 October. Earlier 11 October invalid
session responses were followed by successful authentication and polling.

Two command-result UUID validation errors occurred on 12 October at 09:08:40
and 09:08:42 during batched, read-only time/capability requests. Both were followed
immediately by successful results and then continued polling. The rejected UUID
values were not retained, so their cause is unconfirmed. Later NTP configuration
GETs also completed. No device configuration write preceded the interruption.

This evidence establishes traffic loss before expiry and previously successful
reauthentication. It does not establish a permanent root cause. Device-side and
school-network observations remain necessary; the added transport log supplies
missing connection and timing evidence on subsequent requests.

At 09:26, the transport logging configuration passed `nginx -t` and was deployed
with a graceful Nginx reload. A request using an unregistered diagnostic terminal
returned the expected 404 and produced valid JSON transport metadata. Gateway
readiness and the public administration UI returned 200; the gateway process was
not restarted. PushSDK and configuration tests passed with the Go race detector.

With the user's authorization, `tcpdump` was installed on the VPS. A second,
45-second capture at 09:28 retained only text connection summaries, with no raw
packet file or payload output. It captured incoming connections, including an
external HTTPS control request, but no traffic from the school's last known
public address; the capture reported zero kernel drops. Public DNS resolved the
gateway to the correct VPS IPv4 address and had no AAAA record. The user confirmed
their computer was not currently on the school network. Device-side status and
network evidence were still unavailable.

Both terminals reported manual time, Asia/Shanghai, and clocks approximately
17 and 15 minutes ahead of UTC respectively. Their configured, inactive NTP
server was a private LAN address. Clock correction remains pending a working
command channel; no time or NTP settings were written during this investigation.
