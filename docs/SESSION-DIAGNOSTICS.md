# Terminal connection diagnosis

`pushsdk.session_idle` means the terminal has sent no successful authenticated
traffic for one minute. The gateway marks it offline but retains its latest
challenge so the next valid request can resume without a new login. This does
not identify why traffic stopped and sends no disconnect instruction.
`pushsdk.session_expired` now applies only to incomplete login handshakes;
historical entries from earlier releases also represent deleted authenticated
sessions. A terminal initiates PushSDK HTTP requests, including command polling.
The gateway cannot initiate an ISAPI exchange when that path is unavailable.

## Evidence to collect

1. Compare each terminal's last successful exchange with its idle activity
   (or expiry activity on older releases).
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

During the initial inspection, both terminals reported manual time, Asia/Shanghai, and clocks approximately
17 and 15 minutes ahead of UTC respectively. Their configured, inactive NTP
server was a private LAN address. No time or NTP settings were written before
the user restarted the devices and restored the command channel.

## Clock correction after the 09:38 reconnect

Both terminals authenticated successfully after the user's restart. They used
a different shared public IP address. The user later said Wi-Fi might also have
restarted, so the reconnect does not isolate the earlier outage to either the
terminal software or the network.

Native `PUSHCfg` readbacks confirmed both devices were enabled, using HTTPS to
`vps.itplus.club:443`, and reporting online. GN0954003's interface readback showed
Wi-Fi with DHCP and router-provided DNS. No network parameters were changed.

Native ISAPI capabilities confirmed NTP configuration. The separate NTP test
endpoint returned `notSupport` on both devices; it was not treated as a successful
connectivity test. The configuration writes below returned native status code 1
(`OK`), and NTP server readbacks matched the requested address, port and interval.

| Setting | Both Creative Minds terminals |
| --- | --- |
| NTP server | `162.159.200.1`, UDP port `123` |
| Synchronization interval | `60` minutes |
| Time mode | `NTP` |
| Time zone | `AST-3:00:00`, IANA `Asia/Baghdad` |

| Observation | GN0953967 | GN0954003 |
| --- | --- | --- |
| Clock lead before correction | 1,017 seconds | 873 seconds |
| Time configuration accepted | 09:41:53 | 09:48:21 |
| Subsequent event timestamp / server receipt | 09:42:06 / 09:42:06.533 | 09:48:52 / 09:48:52.543 |
| Session expiry after that last successful event | 09:42:23 (old policy) | 09:49:57 (new policy) |

The accurate post-change event timestamps demonstrate clock synchronization.
Both terminals stopped command polling after the backward correction. Subsequent
event retries reached the gateway but received the invalid-session response;
they did not immediately initiate `AuthInfo`. This is distinct from the earlier
outage with no arriving traffic. A device polling timer affected by a backward
clock step is a hypothesis to verify, not an established firmware diagnosis.

At the user's request, release `9a2f9c61f8d8` changed authenticated-session
inactivity expiry to 60 seconds, independently of the five-second polling
interval. Initial login challenges retain their own deadline. Tests cover a
30-second pause, restoring a checkpoint after that pause, expiry at 60 seconds,
and fresh authentication replacing an unexpired session without accepting an old
proof. Go tests with the race detector and `go vet` passed. GN0954003 resumed its
persisted session at 09:47:40 after deployment, proving physical-device continuity
through that gateway restart. Its later expiry after 64.811 seconds verifies the
new inactivity threshold and five-second maintenance sweep in production.


## Recovery policy correction after the clock change

The post-NTP `Event` retries exposed a gateway lifecycle flaw: the inactivity
sweep had deleted the challenge required to verify returning traffic. The
terminals continued sending their existing proof rather than starting a new
`AuthInfo` exchange. Increasing that deletion deadline to one minute delayed the
failure but did not address it.

The corrected policy separates liveness from authentication. One minute without
contact marks an authenticated terminal offline and retains the latest exact
challenge in PostgreSQL and memory. Returning traffic must prove that challenge;
then normal rotation resumes. Initial login expiry, explicit logout, fresh
AuthInfo replacement, and configuration invalidation still revoke the relevant
state. Completed Login messages cannot be replayed to discard a valid session.

Existing checkpoints deleted by the previous release remain unavailable and
require one fresh device login. This policy cannot make a terminal send command
polls when its own polling loop has stopped. The clock-step polling issue and
real-device recovery must be verified independently of gateway unit tests.

Validation before deployment: Go tests with the race detector passed with a
real, disposable PostgreSQL 17 database, including transactional audit rollback
and preservation of idle checkpoints. `go vet` and frontend formatting, lint,
and build checks passed. Protocol tests exercise a 24-hour idle period, restart
recovery, fresh AuthInfo replacement, used-proof rejection, logout, changed
configuration, late Login, and incomplete-handshake expiry.
