# Device migration and acceptance test

Use this runbook when adding or reconfiguring a terminal. The public virtual
host deliberately denies readiness and metrics paths, so health checks are made
only through the VPS loopback listener. Existing legacy services are already
retired; this runbook does not start, stop, or reconfigure unrelated projects.

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
4. Perform one face/card attendance action. Confirm a single
   `attendance.received` monitor event and one row in the console.
5. Trigger the same vendor event retry if the terminal supports it. The row
   total must remain unchanged because of UUID deduplication.
6. Inspect Prometheus from the private host path. Confirm request/event counters
   move and no rejected-event counter increases.
7. Test a terminal logout/reconnect and a gateway restart. The console must show
   offline after restart until the terminal performs `AuthInfo` and `Login`
   again.

If any step fails, retain the exact gateway JSON logs and the terminal's
configuration screen, then fix the documented wire contract. Do not enable a
legacy parser fallback to force the test through.
