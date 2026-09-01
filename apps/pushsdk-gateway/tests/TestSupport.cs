using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace PushSdkGateway.Tests;

internal sealed class TestEnvironment : IDisposable
{
    public const string LaravelTokenVariable = "PUSHSDK_TEST_LARAVEL_TOKEN";
    public const string TerminalPasswordVariable = "PUSHSDK_TEST_TERMINAL_PASSWORD";
    public const string TerminalSerialNumber = "DS-K1T341CMFW-E1";
    public const string TerminalUsername = "attendance_gateway";
    public const string TerminalPassword = "correct-horse-battery";

    private readonly string? _previousLaravelToken;
    private readonly string? _previousTerminalPassword;

    public TestEnvironment()
    {
        DataDirectory = Path.Combine(Path.GetTempPath(), "pushsdk-gateway-tests", Guid.NewGuid().ToString("N"));
        _previousLaravelToken = Environment.GetEnvironmentVariable(LaravelTokenVariable);
        _previousTerminalPassword = Environment.GetEnvironmentVariable(TerminalPasswordVariable);
        Environment.SetEnvironmentVariable(LaravelTokenVariable, "a-test-token-that-is-longer-than-thirty-two-characters");
        Environment.SetEnvironmentVariable(TerminalPasswordVariable, TerminalPassword);
    }

    public string DataDirectory { get; }

    public GatewayOptions CreateOptions(bool requireDeviceHttps = false)
    {
        return new GatewayOptions
        {
            DataDirectory = DataDirectory,
            Laravel = new LaravelOptions
            {
                BaseUrl = "http://attendance-receiver.test",
                BearerTokenEnvironmentVariable = LaravelTokenVariable,
            },
            Terminals =
            [
                new TerminalOptions
                {
                    SerialNumber = TerminalSerialNumber,
                    Username = TerminalUsername,
                    PasswordEnvironmentVariable = TerminalPasswordVariable,
                    LoginPasswordDigest = "sha256",
                },
            ],
            CommandIntervalSeconds = 5,
            ErrorDelaySeconds = 30,
            AuthChallengeIterations = 5000,
            MaxDeviceRequestBytes = 4 * 1024 * 1024,
            MaxPictureBytes = 2 * 1024 * 1024,
            DeliveryLeaseSeconds = 120,
            DeliveredEventRetentionDays = 90,
            RequireDeviceHttps = requireDeviceHttps,
        };
    }

    public void Dispose()
    {
        Environment.SetEnvironmentVariable(LaravelTokenVariable, _previousLaravelToken);
        Environment.SetEnvironmentVariable(TerminalPasswordVariable, _previousTerminalPassword);
        if (Directory.Exists(DataDirectory))
        {
            Directory.Delete(DataDirectory, recursive: true);
        }
    }
}

internal static class TestProtocol
{
    public static byte[] BuildEventEnvelope(string eventId, string dataFormat, byte[] rawData)
    {
        return JsonSerializer.SerializeToUtf8Bytes(new
        {
            eventNum = 1,
            eventList = new[]
            {
                new
                {
                    UUID = eventId,
                    dataFormat,
                    data = Convert.ToBase64String(rawData),
                },
            },
        });
    }

    public static byte[] AccessEventJson(string employeeNumber = "1001")
    {
        return JsonSerializer.SerializeToUtf8Bytes(new
        {
            eventType = "AccessControllerEvent",
            eventState = "active",
            dateTime = "2026-09-01T08:15:30+03:00",
            AccessControllerEvent = new
            {
                employeeNoString = employeeNumber,
                name = "Amina Karim",
                currentVerifyMode = "face",
                attendanceStatus = "checkIn",
                statusValue = 1,
            },
        });
    }

    public static string CalculateLoginPassword(string salt, string challenge, int iterations)
    {
        var passwordHash = HexSha256(TestEnvironment.TerminalUsername + salt + TestEnvironment.TerminalPassword);
        return Convert.ToHexString(Rfc2898DeriveBytes.Pbkdf2(
            passwordHash + challenge,
            Encoding.UTF8.GetBytes(salt),
            iterations,
            HashAlgorithmName.SHA256,
            64)).ToLowerInvariant();
    }

    public static string CalculateCustomAuth(string salt, string challenge)
    {
        var passwordHash = HexSha256(TestEnvironment.TerminalUsername + salt + TestEnvironment.TerminalPassword);
        return HexSha256(passwordHash + challenge);
    }

    public static EncryptionContext CalculateEncryption(string salt, int iterations, string random, string iv, int security)
    {
        var passwordHash = HexSha256(TestEnvironment.TerminalUsername + salt + TestEnvironment.TerminalPassword);
        var material = Rfc2898DeriveBytes.Pbkdf2(
            passwordHash + random,
            Encoding.UTF8.GetBytes(salt),
            iterations,
            HashAlgorithmName.SHA256,
            64);
        return new EncryptionContext(material[..(security == 3 ? 16 : 32)], Convert.FromHexString(iv));
    }

    private static string HexSha256(string value)
    {
        return Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(value))).ToLowerInvariant();
    }
}
