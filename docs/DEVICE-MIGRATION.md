# Device migration and acceptance test

Use this runbook when adding or reconfiguring a terminal. The public virtual
host deliberately denies readiness and metrics paths, so health checks are made
only through the VPS loopback listener. This runbook does not start, stop, or
reconfigure unrelated projects.

## Prepare the protected configuration

`terminals.json` contains one exact terminal mapping and only the name of its
protected password environment variable. `serialNumber` is the
canonical device serial used in PostgreSQL; `pushSdkSerial` is the value in the
PushSDK URL. The username and password must exactly match the values entered in
the terminal's PushSDK screen. `securityVersion` must be either `3` or `4` and
match the terminal's negotiated capability; this gateway does not downgrade.
`loginPasswordDigest` must be the single PBKDF2 algorithm documented for that
exact firmware (`sha256` or `sha1`). It is not inferred from a failed login;
the previous `digest` mapping key is not accepted.

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
and an exact configuration fingerprint. At startup, it restores a checkpoint
only when the terminal configuration still matches and the most recent
challenge is younger than the vendor's three-command-interval validity window.
The first valid request proves that continuity and records
`pushsdk.session_resumed` before the next challenge is issued.

If the outage exceeds that protocol window, or the configured PushSDK contract
changes, the checkpoint is deleted. The corresponding request receives the
documented `401` invalid-session envelope (`0x0020000f`, `Invalid SessionID.`)
without a next challenge. A conforming terminal then performs `AuthInfo` and
`Login`; the gateway never replays or guesses a challenge and never changes
terminal configuration automatically.

If any step fails, retain the exact gateway JSON logs and the terminal's
configuration screen, then fix the documented wire contract. Do not enable a
permissive parser fallback to force the test through.
