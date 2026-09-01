using System.Text;
using System.Text.Json;
using Microsoft.Data.Sqlite;
using System.Net;
using System.Net.Sockets;

namespace PushSdkGateway;

public sealed partial class PushProtocolHandler
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    private readonly GatewayOptions _options;
    private readonly DeviceRegistry _devices;
    private readonly SessionStore _sessions;
    private readonly AttendanceEventParser _eventParser;
    private readonly GatewayDatabase _database;
    private readonly ILogger<PushProtocolHandler> _logger;

    public PushProtocolHandler(
        GatewayOptions options,
        DeviceRegistry devices,
        SessionStore sessions,
        AttendanceEventParser eventParser,
        GatewayDatabase database,
        ILogger<PushProtocolHandler> logger)
    {
        _options = options;
        _devices = devices;
        _sessions = sessions;
        _eventParser = eventParser;
        _database = database;
        _logger = logger;
    }

    public async Task<DeviceReply> AuthenticateInfoAsync(HttpContext context, string terminalSerialNumber, CancellationToken cancellationToken)
    {
        RequireDeviceHttps(context.Request);
        var terminal = RequireRegisteredTerminal(terminalSerialNumber);
        RequireNoQuery(context.Request);
        var body = await ReadBodyAsync(context.Request, false, cancellationToken);
        var encryptionSecurityVersion = default(int?);

        if (body.Length != 0)
        {
            RequireJsonContent(context.Request);
            try
            {
                using var document = JsonDocument.Parse(body);
                var root = document.RootElement;
                if (root.ValueKind != JsonValueKind.Object
                    || !root.TryGetProperty("data", out var data)
                    || data.ValueKind != JsonValueKind.Object)
                {
                    throw new ProtocolException(400, "AuthInfo requires a JSON data object when it has a request body.");
                }

                if (data.TryGetProperty("securityVersion", out var securityVersions))
                {
                    if (securityVersions.ValueKind != JsonValueKind.Array
                        || securityVersions.EnumerateArray().Any(value => !value.TryGetInt32(out var version) || version is not (3 or 4)))
                    {
                        throw new ProtocolException(400, "AuthInfo securityVersion must contain only 3 or 4.");
                    }

                    encryptionSecurityVersion = securityVersions.EnumerateArray().Any(value => value.GetInt32() == 4)
                        ? 4
                        : securityVersions.EnumerateArray().Any(value => value.GetInt32() == 3)
                            ? 3
                            : null;
                }
            }
            catch (JsonException exception)
            {
                throw new ProtocolException(400, "AuthInfo request is not valid JSON.", exception);
            }
        }

        var session = _sessions.BeginAuthentication(terminal, _options.AuthChallengeIterations, encryptionSecurityVersion);
        _logger.LogInformation("Push SDK terminal {PushSdkSerialNumber} requested authentication information for canonical terminal {TerminalSerialNumber}.", terminalSerialNumber, terminal.SerialNumber);
        return DeviceReply.FromJson(200, Serialize(new
        {
            data = new
            {
                challenge = session.Challenge,
                salt = session.Salt,
                iterations = session.Iterations,
                isDataEncrypt = encryptionSecurityVersion is not null,
                securityVersion = new[] { 3, 4 },
            },
        }));
    }

    public async Task<DeviceReply> LoginAsync(HttpContext context, string terminalSerialNumber, CancellationToken cancellationToken)
    {
        RequireDeviceHttps(context.Request);
        var terminal = RequireRegisteredTerminal(terminalSerialNumber);
        var canonicalTerminalSerialNumber = terminal.SerialNumber;
        RequireNoQuery(context.Request);
        RequireJsonContent(context.Request);
        var body = await ReadBodyAsync(context.Request, true, cancellationToken);
        var deviceAddress = RequireDeviceAddress(context.Request);
        var nowUtc = DateTimeOffset.UtcNow;
        var lockStatus = _sessions.GetLoginLockStatus(deviceAddress, nowUtc);
        if (lockStatus.IsLocked)
        {
            return LoginFailure(lockStatus);
        }

        if (!_sessions.TryGetAuthenticated(canonicalTerminalSerialNumber, out var session))
        {
            var unauthenticatedSession = GetSessionForLogin(canonicalTerminalSerialNumber);
            if (unauthenticatedSession is null)
            {
                return LoginFailure(LoginLockStatus.Unlocked(5));
            }

            session = unauthenticatedSession;
        }

        await session.SerialGate.WaitAsync(cancellationToken);
        try
        {
            if (session.IsAuthenticated
                || session.LoginAttempted
                || nowUtc >= session.AuthInfoIssuedAtUtc.AddSeconds(3 * _options.CommandIntervalSeconds))
            {
                return LoginFailure(LoginLockStatus.Unlocked(5));
            }

            session.LoginAttempted = true;

            string username;
            string loginPassword;
            try
            {
                using var document = JsonDocument.Parse(body);
                var root = document.RootElement;
                if (root.ValueKind != JsonValueKind.Object
                    || !root.TryGetProperty("data", out var data)
                    || data.ValueKind != JsonValueKind.Object)
                {
                    throw new ProtocolException(400, "Login requires a JSON data object.");
                }

                username = RequiredString(data, "username", 64, "Login");
                loginPassword = RequiredString(data, "loginPassword", 128, "Login");
            }
            catch (JsonException exception)
            {
                throw new ProtocolException(400, "Login request is not valid JSON.", exception);
            }

            var expectedLoginPassword = PushCrypto.CalculateLoginPassword(session);
            if (username != session.Terminal.Username || !PushCrypto.FixedTimeEquals(expectedLoginPassword, loginPassword))
            {
                return LoginFailure(_sessions.RecordLoginFailure(deviceAddress, nowUtc, 60));
            }

            _sessions.RecordLoginSuccess(deviceAddress);
            session.IsAuthenticated = true;
            session.Challenge = PushCrypto.RandomHex(64);
            _logger.LogInformation("Push SDK terminal {PushSdkSerialNumber} authenticated as canonical terminal {TerminalSerialNumber} with the configured {PasswordDigest} login digest.", terminalSerialNumber, canonicalTerminalSerialNumber, session.Terminal.LoginPasswordDigest);
            return DeviceReply.FromJson(200, Serialize(new
            {
                status = 200,
                code = "0x00000000",
                errorMsg = "Succeeded.",
                data = new
                {
                    commandInterval = _options.CommandIntervalSeconds,
                    errorDelay = _options.ErrorDelaySeconds,
                },
            }), session.Challenge);
        }
        finally
        {
            session.SerialGate.Release();
        }
    }

    public Task<DeviceReply> CommandRequestAsync(HttpContext context, string terminalSerialNumber, CancellationToken cancellationToken)
    {
        return ExecuteAuthenticatedAsync(
            context,
            terminalSerialNumber,
            requireEmptyBody: true,
            supportsPayloadEncryption: true,
            async (_, _, _) =>
            {
                await Task.CompletedTask;
                return DeviceReply.FromJson(200, Serialize(new
                {
                    status = 200,
                    code = "0x00000000",
                    errorMsg = "Succeeded.",
                    commandNum = 0,
                }));
            },
            cancellationToken);
    }

    public Task<DeviceReply> CommandResultAsync(HttpContext context, string terminalSerialNumber, CancellationToken cancellationToken)
    {
        return ExecuteAuthenticatedAsync(
            context,
            terminalSerialNumber,
            requireEmptyBody: false,
            supportsPayloadEncryption: true,
            async (_, body, _) =>
            {
                try
                {
                    using var document = JsonDocument.Parse(body);
                    var root = document.RootElement;
                    if (root.ValueKind != JsonValueKind.Object
                        || !root.TryGetProperty("commandNum", out var commandCount)
                        || !commandCount.TryGetInt32(out var count)
                        || count != 0
                        || !root.TryGetProperty("commandList", out var commandList)
                        || commandList.ValueKind != JsonValueKind.Array
                        || commandList.GetArrayLength() != 0)
                    {
                        throw new ProtocolException(422, "The attendance gateway never issues device commands, so CommandResult must contain commandNum 0 and an empty commandList.");
                    }
                }
                catch (JsonException exception)
                {
                    throw new ProtocolException(400, "CommandResult request is not valid JSON.", exception);
                }

                await Task.CompletedTask;
                return DeviceReply.FromJson(200, Serialize(new
                {
                    status = 200,
                    code = "0x00000000",
                    errorMsg = "Succeeded.",
                    isPendingCommand = false,
                }));
            },
            cancellationToken);
    }

    public Task<DeviceReply> EventAsync(HttpContext context, string terminalSerialNumber, CancellationToken cancellationToken)
    {
        return ExecuteAuthenticatedAsync(
            context,
            terminalSerialNumber,
            requireEmptyBody: false,
            supportsPayloadEncryption: true,
            async (terminal, body, _) =>
            {
                var parsedEvents = _eventParser.ParseBatch(terminal.SerialNumber, body);
                try
                {
                    await _database.PersistEventsAsync(terminal.SerialNumber, parsedEvents, DateTimeOffset.UtcNow, cancellationToken);
                }
                catch (SqliteException exception)
                {
                    throw new ProtocolException(503, "The gateway could not persist the event batch; retry the Push SDK request.", exception);
                }

                return DeviceReply.FromJson(200, Serialize(parsedEvents.Select(parsedEvent => new
                {
                    UUID = parsedEvent.VendorEventId,
                    status = 200,
                    code = "0x00000000",
                    errorMsg = "Succeeded.",
                })));
            },
            cancellationToken);
    }

    public Task<DeviceReply> LogoutAsync(HttpContext context, string terminalSerialNumber, CancellationToken cancellationToken)
    {
        return ExecuteAuthenticatedAsync(
            context,
            terminalSerialNumber,
            requireEmptyBody: true,
            supportsPayloadEncryption: false,
            async (terminal, _, _) =>
            {
                _sessions.Remove(terminal.SerialNumber);
                await Task.CompletedTask;
                _logger.LogInformation("Push SDK terminal {PushSdkSerialNumber} logged out from canonical terminal {TerminalSerialNumber}.", terminalSerialNumber, terminal.SerialNumber);
                return DeviceReply.FromJson(200, Serialize(new
                {
                    status = 200,
                    code = "0x00000000",
                    errorMsg = "Succeeded.",
                }));
            },
            cancellationToken);
    }

    private async Task<DeviceReply> ExecuteAuthenticatedAsync(
        HttpContext context,
        string terminalSerialNumber,
        bool requireEmptyBody,
        bool supportsPayloadEncryption,
        Func<RegisteredTerminal, byte[], EncryptionContext?, Task<DeviceReply>> action,
        CancellationToken cancellationToken)
    {
        RequireDeviceHttps(context.Request);
        var terminal = RequireRegisteredTerminal(terminalSerialNumber);
        var canonicalTerminalSerialNumber = terminal.SerialNumber;
        if (!_sessions.TryGetAuthenticated(canonicalTerminalSerialNumber, out var session))
        {
            return ErrorReply(401, "Invalid SessionID.");
        }

        await session.SerialGate.WaitAsync(cancellationToken);
        try
        {
            var providedAuth = context.Request.Headers["My-Custom-Auth"].ToString();
            if (!PushCrypto.FixedTimeEquals(PushCrypto.CalculateCustomAuth(session), providedAuth))
            {
                return ErrorReply(401, "Invalid SessionID.");
            }

            EncryptionContext? encryption = null;
            DeviceReply reply;
            try
            {
                if (session.EncryptionSecurityVersion is null)
                {
                    RequireNoQuery(context.Request);
                }
                else if (!supportsPayloadEncryption)
                {
                    RequireNoQuery(context.Request);
                }
                else
                {
                    encryption = PushCrypto.ParseEncryptionContext(context.Request, session);
                }

                var body = await ReadBodyAsync(context.Request, !requireEmptyBody, cancellationToken);
                if (requireEmptyBody && body.Length != 0)
                {
                    throw new ProtocolException(400, "This endpoint does not accept a request body.");
                }

                if (!requireEmptyBody && body.Length == 0)
                {
                    throw new ProtocolException(400, "This endpoint requires a request body.");
                }

                if (encryption is null)
                {
                    if (!requireEmptyBody)
                    {
                        RequireJsonContent(context.Request);
                    }
                }
                else if (!requireEmptyBody)
                {
                    RequireBinaryContent(context.Request);
                    body = PushCrypto.Decrypt(encryption, body);
                }

                reply = await action(terminal, body, encryption);
            }
            catch (ProtocolException exception)
            {
                // Keep the terminal payload out of logs, while recording the
                // actionable protocol reason for an authenticated request.
                _logger.LogWarning(
                    "Rejected authenticated Push SDK request to {Path} from terminal {TerminalSerialNumber}: {Reason}",
                    context.Request.Path,
                    terminal.SerialNumber,
                    exception.Message);
                reply = ErrorReply(exception.StatusCode, exception.Message);
            }

            session.Challenge = PushCrypto.RandomHex(64);
            return reply with
            {
                Encryption = encryption,
                CustomChallenge = session.Challenge,
            };
        }
        finally
        {
            session.SerialGate.Release();
        }
    }

    private DeviceSession? GetSessionForLogin(string terminalSerialNumber)
    {
        return _sessions.TryGetSession(terminalSerialNumber, out var session) ? session : null;
    }

    private RegisteredTerminal RequireRegisteredTerminal(string terminalSerialNumber)
    {
        if (!_devices.TryGet(terminalSerialNumber, out var terminal))
        {
            throw new ProtocolException(404, "Terminal is not registered with this gateway.");
        }

        return terminal;
    }

    private void RequireDeviceHttps(HttpRequest request)
    {
        if (_options.RequireDeviceHttps && request.Headers["X-Forwarded-Proto"].ToString() != "https")
        {
            throw new ProtocolException(400, "Push SDK traffic must arrive through the HTTPS reverse proxy.");
        }
    }

    private string RequireDeviceAddress(HttpRequest request)
    {
        if (_options.RequireDeviceHttps)
        {
            var forwardedAddress = request.Headers["X-PushSDK-Device-Address"].ToString();
            var match = DeviceAddressPattern().Match(forwardedAddress);
            if (!match.Success
                || !IPAddress.TryParse(match.Groups["address"].Value, out _)
                || !int.TryParse(match.Groups["port"].Value, out var port)
                || port is < 1 or > 65535)
            {
                throw new ProtocolException(400, "The HTTPS reverse proxy must provide the terminal source address.");
            }

            return forwardedAddress;
        }

        var address = request.HttpContext.Connection.RemoteIpAddress;
        if (address is null || request.HttpContext.Connection.RemotePort is <= 0 or > 65535)
        {
            throw new ProtocolException(400, "The terminal source address is unavailable.");
        }

        return address.AddressFamily == AddressFamily.InterNetworkV6
            ? $"[{address}]:{request.HttpContext.Connection.RemotePort}"
            : $"{address}:{request.HttpContext.Connection.RemotePort}";
    }

    private async Task<byte[]> ReadBodyAsync(HttpRequest request, bool required, CancellationToken cancellationToken)
    {
        if (request.ContentLength is { } contentLength && contentLength > _options.MaxDeviceRequestBytes)
        {
            throw new ProtocolException(413, "The request body exceeds the configured device message limit.");
        }

        await using var output = new MemoryStream();
        await request.Body.CopyToAsync(output, cancellationToken);
        var body = output.ToArray();
        if (body.Length > _options.MaxDeviceRequestBytes)
        {
            throw new ProtocolException(413, "The request body exceeds the configured device message limit.");
        }

        if (required && body.Length == 0)
        {
            throw new ProtocolException(400, "The request body is required.");
        }

        return body;
    }

    private static void RequireNoQuery(HttpRequest request)
    {
        if (request.Query.Count != 0)
        {
            throw new ProtocolException(400, "This endpoint does not accept query parameters.");
        }
    }

    private static void RequireJsonContent(HttpRequest request)
    {
        if (!HasMediaType(request, "application/json"))
        {
            throw new ProtocolException(415, "The request Content-Type must be application/json.");
        }
    }

    private static void RequireBinaryContent(HttpRequest request)
    {
        if (!HasMediaType(request, "application/octet-stream"))
        {
            throw new ProtocolException(415, "Encrypted request Content-Type must be application/octet-stream.");
        }
    }

    private static bool HasMediaType(HttpRequest request, string expectedMediaType)
    {
        return request.ContentType?.Split(';', 2, StringSplitOptions.TrimEntries)[0] == expectedMediaType;
    }

    private static string RequiredString(JsonElement objectElement, string property, int maximumLength, string operation)
    {
        if (!objectElement.TryGetProperty(property, out var value) || value.ValueKind != JsonValueKind.String)
        {
            throw new ProtocolException(400, $"{operation} requires string '{property}'.");
        }

        var text = value.GetString()!;
        if (text.Length == 0 || text.Length > maximumLength)
        {
            throw new ProtocolException(400, $"{operation} '{property}' has an invalid length.");
        }

        return text;
    }

    private static DeviceReply LoginFailure(LoginLockStatus loginLock)
    {
        return DeviceReply.FromJson(401, Serialize(new
        {
            status = 401,
            code = "0x0020000f",
            errorMsg = loginLock.IsLocked ? "IP is locked due to too many failed attempts." : "Invalid SessionID.",
            data = new
            {
                lockStatus = loginLock.IsLocked ? "lock" : "unlock",
                unlockTime = loginLock.UnlockSeconds,
                retryLoginTime = loginLock.RemainingAttempts,
            },
        }));
    }

    private static DeviceReply ErrorReply(int statusCode, string message)
    {
        return DeviceReply.FromJson(statusCode, Serialize(new
        {
            status = statusCode,
            code = statusCode == 401 ? "0x0020000f" : "0x00100002",
            errorMsg = message,
        }));
    }

    private static string Serialize<T>(T value)
    {
        return JsonSerializer.Serialize(value, JsonOptions);
    }

    [System.Text.RegularExpressions.GeneratedRegex("\\A\\[(?<address>[0-9A-Fa-f:.]+)\\]:(?<port>[0-9]{1,5})\\z", System.Text.RegularExpressions.RegexOptions.CultureInvariant)]
    private static partial System.Text.RegularExpressions.Regex DeviceAddressPattern();
}

public sealed record DeviceReply(int StatusCode, string Payload, EncryptionContext? Encryption, string? CustomChallenge)
{
    public static DeviceReply FromJson(int statusCode, string payload, string? customChallenge = null) => new(statusCode, payload, null, customChallenge);
}

public static class DeviceReplyWriter
{
    public static async Task WriteAsync(HttpContext context, DeviceReply reply, CancellationToken cancellationToken)
    {
        context.Response.StatusCode = reply.StatusCode;
        if (reply.CustomChallenge is not null)
        {
            context.Response.Headers["My-Custom-Challenge"] = reply.CustomChallenge;
        }

        if (reply.Encryption is null)
        {
            context.Response.ContentType = "application/json; charset=utf-8";
            await context.Response.WriteAsync(reply.Payload, Encoding.UTF8, cancellationToken);
            return;
        }

        context.Response.ContentType = "application/octet-stream";
        await context.Response.Body.WriteAsync(PushCrypto.Encrypt(reply.Encryption, reply.Payload), cancellationToken);
    }
}
