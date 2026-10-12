# Device migration and acceptance test

Use this runbook when adding or reconfiguring a terminal. The public virtual
host deliberately denies readiness and metrics paths, so health checks are made
only through the VPS loopback listener. This runbook does not start, stop, or
reconfigure unrelated projects.

## Prepare the protected configuration

`terminals.json` contains one exact mapping for every terminal and only the
name of each protected password environment variable. `serialNumber` is the
canonical device serial used in PostgreSQL; `pushSdkSerial` is the value in the
PushSDK URL. The username and password must exactly match the values entered in
the terminal's PushSDK screen. `securityVersion` must be either `3` or `4` and
match the terminal's negotiated capability; this gateway does not downgrade.
`loginPasswordDigest` must be the single PBKDF2 algorithm documented for that
exact firmware (`sha256` or `sha1`). It is not inferred from a failed login;
the previous `digest` mapping key is not accepted.

On startup the gateway runs `SeedConfiguredTerminals`, which upserts this
mapping before the terminal can authenticate. It records only the terminal
identity, PushSDK protocol settings, and a credential fingerprint; the password
remains in the protected environment file. For the installed devices, use the
[DS-K1T342MFWX-E1 `GN0953967` terminal record](devices/hikvision-ds-k1t342mfwx-e1.md)
and [DS-K1T342MFWX-E1 `GN0954003` terminal record](devices/hikvision-ds-k1t342mfwx-e1-gn0954003.md)
as the sources of exact non-secret identifiers.

Use HTTPS server `vps.itplus.club`, HTTPS port `443`, and WebSocket port `443`
only after Nginx routes to the new loopback gateway. The terminal must show
online registration before continuing.

## Acceptance sequence

1. On the VPS, confirm `curl --fail http://127.0.0.1:18080/readyz` succeeds.
   The equivalent public URL intentionally returns `404` at Nginx.
2. Open `https://vps.itplus.club/app/`, sign in, and confirm the configured
   terminal initially reads `offline`.
3. Enable PushSDK on the terminal. The live monitor must show `auth_info` then
   `login`; terminal status becomes `online`.
4. Trigger any device event, such as a face/card verification or door action.
   Confirm a single `device.event_persisted` monitor event and one row in Device
   Events. Inspect its payload and confirm the source `eventList.data` value is
   present unchanged.
5. Trigger the same vendor event retry if the terminal supports it. The monitor
   must show `device.event_duplicate` and the row total must remain unchanged
   because of UUID deduplication.
6. Inspect Prometheus from the private host path. Confirm request/event counters
   move and no rejected-event counter increases.
7. Test a terminal logout/reconnect and then a short gateway restart while the
   terminal is online. On its next scheduled request, the console must record
   `pushsdk.session_resumed` and return to `online` without a new `AuthInfo` or
   `Login`. Device Events and Gateway Activity must still show records from
   before the restart.

## Gateway restart recovery

The gateway checkpoints every negotiated session before replying to the
terminal. The checkpoint contains no terminal password: only the payload mode,
server-generated salt and challenge values, their timestamps, iteration count,
and an exact configuration fingerprint. At startup it restores authenticated
checkpoints whose configuration still matches, regardless of idle duration.
Restoration alone does not mark a terminal online. The first valid request
records `pushsdk.session_resumed` and normal challenge rotation continues.

The gateway checks contact every five seconds. After 60 seconds without a
successful authenticated exchange, it marks the terminal offline and records
`pushsdk.session_idle` atomically. The authentication checkpoint stays in memory
and PostgreSQL. Repeated maintenance does not duplicate the idle activity.
`last_seen_at` remains the last successful contact; the idle transition and
rejected proofs do not advance it. The console refreshes on idle/resume activity,
and `pushsdk_sessions_active` counts online authenticated sessions.

Only the latest issued challenge is accepted. A correctly authenticated request
after an idle period restores online status, and its successful response issues
the next challenge. There is no cache accepting older or already-used proofs.
The initial, unauthenticated login challenge keeps its separate
three-command-interval deadline and is deleted when it expires.

Fresh `AuthInfo` replaces an existing online or idle session immediately.
`Logout`, removal of the configured terminal, and changes to its exact protocol
configuration revoke persisted authentication. All exchanges for one terminal,
including replacement and response writing, are serialized. A stale request
receives `401` but cannot delete a newer session; replaying `Login` after its
completion also cannot rotate or discard that session.

If the device repeats `Event` requests with `401` and never sends `AuthInfo`,
automatic recovery has not completed. Check the safe `reason` field in rejection
activity (`missing_session`, `not_authenticated`, or `custom_auth_mismatch`).
The March 2026 Hikvision demo's `HandleCommonResponse` uses the same three-field
invalid-session response for authenticated operations. Its lock/retry fields
belong to login failures; they are not a documented event-recovery mechanism.
A checkpoint already deleted by an older release cannot be reconstructed from
an incoming proof; one fresh `AuthInfo`/`Login` exchange is required in that case.
Do not claim firmware recovery from a gateway unit test: verify successful
command polling from the physical device after an interruption.

If any step fails, retain the exact gateway JSON logs and the terminal's
configuration screen, then fix the documented wire contract. Do not enable a
permissive parser fallback to force the test through.

For recurring offline states, follow [Terminal connection diagnosis](SESSION-DIAGNOSTICS.md)
to distinguish missing traffic, transport failures and authentication failures.
