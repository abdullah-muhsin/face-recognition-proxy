using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging.Abstractions;
using System.Net;
using System.Text;
using System.Text.Json;

namespace PushSdkGateway.Tests;

public sealed class PushProtocolHandlerTests
{
    [Fact]
    public async Task AuthenticatesAndPersistsAnEncryptedAttendanceEventBeforeAcknowledgingIt()
    {
        using var environment = new TestEnvironment();
        var options = environment.CreateOptions();
        options.Validate();
        var database = new GatewayDatabase(options);
        await database.InitializeAsync(CancellationToken.None);
        var handler = new PushProtocolHandler(
            options,
            new DeviceRegistry(options),
            new SessionStore(),
            new AttendanceEventParser(options),
            database,
            NullLogger<PushProtocolHandler>.Instance);

        var authInfo = await handler.AuthenticateInfoAsync(
            CreateContext("application/json", Encoding.UTF8.GetBytes("{\"data\":{\"securityVersion\":[3,4]}}")),
            TestEnvironment.TerminalSerialNumber,
            CancellationToken.None);
        using var authInfoDocument = JsonDocument.Parse(authInfo.Payload);
        var authData = authInfoDocument.RootElement.GetProperty("data");
        var challenge = authData.GetProperty("challenge").GetString()!;
        var salt = authData.GetProperty("salt").GetString()!;
        var iterations = authData.GetProperty("iterations").GetInt32();

        var loginPassword = TestProtocol.CalculateLoginPassword(salt, challenge, iterations);
        var login = await handler.LoginAsync(
            CreateContext("application/json", JsonSerializer.SerializeToUtf8Bytes(new
            {
                data = new { username = TestEnvironment.TerminalUsername, loginPassword },
            })),
            TestEnvironment.TerminalSerialNumber,
            CancellationToken.None);
        Assert.Equal(200, login.StatusCode);
        challenge = Assert.IsType<string>(login.CustomChallenge);

        const string random = "0123456789abcdef";
        const string iv = "00112233445566778899aabbccddeeff";
        var encryption = TestProtocol.CalculateEncryption(salt, iterations, random, iv, 4);
        var eventBody = TestProtocol.BuildEventEnvelope("event-encrypted-1", "jsonData", TestProtocol.AccessEventJson());
        var encryptedEvent = PushCrypto.Encrypt(encryption, Encoding.UTF8.GetString(eventBody));
        var eventContext = CreateContext("application/octet-stream", encryptedEvent);
        eventContext.Request.QueryString = QueryString.Create(new Dictionary<string, string?>
        {
            ["security"] = "4",
            ["iv"] = iv,
            ["random"] = random,
        });
        eventContext.Request.Headers["My-Custom-Auth"] = TestProtocol.CalculateCustomAuth(salt, challenge);

        var acknowledgement = await handler.EventAsync(eventContext, TestEnvironment.TerminalSerialNumber, CancellationToken.None);

        Assert.Equal(200, acknowledgement.StatusCode);
        Assert.NotNull(acknowledgement.Encryption);
        Assert.Matches("^[0-9a-f]{64}$", Assert.IsType<string>(acknowledgement.CustomChallenge));
        var responseContext = CreateContext(null, Array.Empty<byte>());
        await DeviceReplyWriter.WriteAsync(responseContext, acknowledgement, CancellationToken.None);
        Assert.Equal("application/octet-stream", responseContext.Response.ContentType);
        var encryptedAcknowledgement = ((MemoryStream)responseContext.Response.Body).ToArray();
        Assert.Contains("event-encrypted-1", Encoding.UTF8.GetString(PushCrypto.Decrypt(encryption, encryptedAcknowledgement)), StringComparison.Ordinal);
        var delivery = await database.ClaimDeliveryAsync(DateTimeOffset.UtcNow, TimeSpan.FromMinutes(2), CancellationToken.None);
        var claimed = Assert.IsType<LeasedDelivery>(delivery);
        Assert.Equal("event-encrypted-1", claimed.VendorEventId);
        using var payload = JsonDocument.Parse(claimed.PayloadJson);
        Assert.Equal("attendance.push-sdk.gateway.v1", payload.RootElement.GetProperty("schema").GetString());
        Assert.Equal("1001", payload.RootElement.GetProperty("event").GetProperty("employee_number").GetString());
    }

    [Fact]
    public async Task SupportsPlaintextAndNegotiatedEncryptedCommandRequestsOverHttps()
    {
        using var environment = new TestEnvironment();
        var options = environment.CreateOptions();
        options.Validate();
        var database = new GatewayDatabase(options);
        await database.InitializeAsync(CancellationToken.None);
        var handler = new PushProtocolHandler(
            options,
            new DeviceRegistry(options),
            new SessionStore(),
            new AttendanceEventParser(options),
            database,
            NullLogger<PushProtocolHandler>.Instance);

        var session = await Authenticate(handler);
        var command = CreateContext(null, Array.Empty<byte>());
        command.Request.Headers["My-Custom-Auth"] = TestProtocol.CalculateCustomAuth(session.Salt, session.Challenge);

        var response = await handler.CommandRequestAsync(command, TestEnvironment.TerminalSerialNumber, CancellationToken.None);

        Assert.Equal(200, response.StatusCode);
        Assert.Null(response.Encryption);
        var nextChallenge = Assert.IsType<string>(response.CustomChallenge);

        const string random = "fedcba9876543210";
        const string iv = "ffeeddccbbaa99887766554433221100";
        var encryptedCommand = CreateContext(null, Array.Empty<byte>());
        encryptedCommand.Request.QueryString = QueryString.Create(new Dictionary<string, string?>
        {
            ["security"] = "4",
            ["iv"] = iv,
            ["random"] = random,
        });
        encryptedCommand.Request.Headers["My-Custom-Auth"] = TestProtocol.CalculateCustomAuth(session.Salt, nextChallenge);

        var encryptedResponse = await handler.CommandRequestAsync(encryptedCommand, TestEnvironment.TerminalSerialNumber, CancellationToken.None);

        Assert.Equal(200, encryptedResponse.StatusCode);
        Assert.NotNull(encryptedResponse.Encryption);
    }

    [Fact]
    public async Task PermitsAnEmptyAuthInfoRequestWithoutNegotiatingPayloadEncryption()
    {
        using var environment = new TestEnvironment();
        var options = environment.CreateOptions();
        options.Validate();
        var database = new GatewayDatabase(options);
        await database.InitializeAsync(CancellationToken.None);
        var handler = new PushProtocolHandler(
            options,
            new DeviceRegistry(options),
            new SessionStore(),
            new AttendanceEventParser(options),
            database,
            NullLogger<PushProtocolHandler>.Instance);

        var response = await handler.AuthenticateInfoAsync(
            CreateContext(null, Array.Empty<byte>()),
            TestEnvironment.TerminalSerialNumber,
            CancellationToken.None);

        using var document = JsonDocument.Parse(response.Payload);
        Assert.False(document.RootElement.GetProperty("data").GetProperty("isDataEncrypt").GetBoolean());
    }

    private static async Task<(string Salt, string Challenge, int Iterations)> Authenticate(PushProtocolHandler handler)
    {
        var authInfo = await handler.AuthenticateInfoAsync(
            CreateContext("application/json", Encoding.UTF8.GetBytes("{\"data\":{\"securityVersion\":[3,4]}}")),
            TestEnvironment.TerminalSerialNumber,
            CancellationToken.None);
        using var authInfoDocument = JsonDocument.Parse(authInfo.Payload);
        var data = authInfoDocument.RootElement.GetProperty("data");
        var salt = data.GetProperty("salt").GetString()!;
        var challenge = data.GetProperty("challenge").GetString()!;
        var iterations = data.GetProperty("iterations").GetInt32();
        var loginPassword = TestProtocol.CalculateLoginPassword(salt, challenge, iterations);
        var login = await handler.LoginAsync(
            CreateContext("application/json", JsonSerializer.SerializeToUtf8Bytes(new
            {
                data = new { username = TestEnvironment.TerminalUsername, loginPassword },
            })),
            TestEnvironment.TerminalSerialNumber,
            CancellationToken.None);
        return (salt, Assert.IsType<string>(login.CustomChallenge), iterations);
    }

    private static DefaultHttpContext CreateContext(string? contentType, byte[] body)
    {
        var context = new DefaultHttpContext();
        context.Request.Method = HttpMethods.Post;
        context.Request.ContentType = contentType;
        context.Request.ContentLength = body.Length;
        context.Request.Body = new MemoryStream(body);
        context.Response.Body = new MemoryStream();
        context.Connection.RemoteIpAddress = IPAddress.Loopback;
        context.Connection.RemotePort = 39001;
        return context;
    }
}
