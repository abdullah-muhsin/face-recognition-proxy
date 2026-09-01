using System.Text.RegularExpressions;

namespace PushSdkGateway;

public sealed class GatewayOptions
{
    public const string SectionName = "Gateway";

    public string DataDirectory { get; init; } = string.Empty;

    public LaravelOptions Laravel { get; init; } = new();

    public List<TerminalOptions> Terminals { get; init; } = [];

    public int CommandIntervalSeconds { get; init; }

    public int ErrorDelaySeconds { get; init; }

    public int AuthChallengeIterations { get; init; }

    public int MaxDeviceRequestBytes { get; init; }

    public int MaxPictureBytes { get; init; }

    public int DeliveryLeaseSeconds { get; init; }

    public int DeliveredEventRetentionDays { get; init; }

    public bool RequireDeviceHttps { get; init; }

    public void Validate()
    {
        if (string.IsNullOrWhiteSpace(DataDirectory) || !Path.IsPathFullyQualified(DataDirectory))
        {
            throw new InvalidOperationException("Gateway:DataDirectory must be an absolute path.");
        }

        Laravel.Validate();

        if (Terminals.Count == 0)
        {
            throw new InvalidOperationException("Gateway:Terminals must contain at least one terminal.");
        }

        if (Terminals.Select(terminal => terminal.SerialNumber).Distinct(StringComparer.Ordinal).Count() != Terminals.Count)
        {
            throw new InvalidOperationException("Gateway:Terminals contains duplicate serial numbers.");
        }

        foreach (var terminal in Terminals)
        {
            terminal.Validate();
        }

        if (Terminals.Select(terminal => terminal.EffectivePushSdkSerialNumber).Distinct(StringComparer.Ordinal).Count() != Terminals.Count)
        {
            throw new InvalidOperationException("Gateway:Terminals contains duplicate Push SDK serial numbers.");
        }

        if (CommandIntervalSeconds is < 1 or > 60)
        {
            throw new InvalidOperationException("Gateway:CommandIntervalSeconds must be between 1 and 60.");
        }

        if (ErrorDelaySeconds is < 30 or > 300)
        {
            throw new InvalidOperationException("Gateway:ErrorDelaySeconds must be between 30 and 300.");
        }

        if (AuthChallengeIterations is < 500 or > 5000)
        {
            throw new InvalidOperationException("Gateway:AuthChallengeIterations must be between 500 and 5000.");
        }

        if (MaxPictureBytes is < 4 or > 2 * 1024 * 1024)
        {
            throw new InvalidOperationException("Gateway:MaxPictureBytes must be between 4 and 2097152.");
        }

        if (MaxDeviceRequestBytes < MaxPictureBytes + (MaxPictureBytes / 2))
        {
            throw new InvalidOperationException("Gateway:MaxDeviceRequestBytes is too small for a base64-encoded maximum-size picture.");
        }

        if (DeliveryLeaseSeconds is < 30 or > 900)
        {
            throw new InvalidOperationException("Gateway:DeliveryLeaseSeconds must be between 30 and 900.");
        }

        if (DeliveredEventRetentionDays is < 1 or > 3650)
        {
            throw new InvalidOperationException("Gateway:DeliveredEventRetentionDays must be between 1 and 3650.");
        }
    }
}

public sealed partial class LaravelOptions
{
    public string BaseUrl { get; init; } = string.Empty;

    public string BearerTokenEnvironmentVariable { get; init; } = string.Empty;

    public Uri ParseBaseUri()
    {
        if (!Uri.TryCreate(BaseUrl, UriKind.Absolute, out var uri)
            || uri.Scheme is not ("http" or "https")
            || !string.IsNullOrEmpty(uri.Query)
            || !string.IsNullOrEmpty(uri.Fragment)
            || (uri.AbsolutePath != "/" && uri.AbsolutePath.Length != 0))
        {
            throw new InvalidOperationException("Gateway:Laravel:BaseUrl must be an absolute HTTP(S) origin without a path, query, or fragment.");
        }

        return uri;
    }

    public void Validate()
    {
        _ = ParseBaseUri();

        if (!EnvironmentVariableNamePattern().IsMatch(BearerTokenEnvironmentVariable))
        {
            throw new InvalidOperationException("Gateway:Laravel:BearerTokenEnvironmentVariable is invalid.");
        }

        var token = Environment.GetEnvironmentVariable(BearerTokenEnvironmentVariable);
        if (string.IsNullOrEmpty(token) || token.Length < 32)
        {
            throw new InvalidOperationException($"The Laravel bearer token environment variable '{BearerTokenEnvironmentVariable}' must contain at least 32 characters.");
        }
    }

    [GeneratedRegex("\\A[A-Z][A-Z0-9_]*\\z", RegexOptions.CultureInvariant)]
    private static partial Regex EnvironmentVariableNamePattern();
}

public sealed partial class TerminalOptions
{
    public string SerialNumber { get; init; } = string.Empty;

    public string PushSdkSerialNumber { get; init; } = string.Empty;

    public string Username { get; init; } = string.Empty;

    public string PasswordEnvironmentVariable { get; init; } = string.Empty;

    public string LoginPasswordDigest { get; init; } = string.Empty;

    public string EffectivePushSdkSerialNumber => string.IsNullOrWhiteSpace(PushSdkSerialNumber)
        ? SerialNumber
        : PushSdkSerialNumber;

    public void Validate()
    {
        if (!TerminalSerialPattern().IsMatch(SerialNumber))
        {
            throw new InvalidOperationException("Every Gateway:Terminals:SerialNumber must contain only letters, digits, dots, underscores, or hyphens and be at most 160 characters.");
        }

        if (!TerminalSerialPattern().IsMatch(EffectivePushSdkSerialNumber))
        {
            throw new InvalidOperationException("Every Gateway:Terminals:PushSdkSerialNumber must contain only letters, digits, dots, underscores, or hyphens and be at most 160 characters.");
        }

        if (string.IsNullOrWhiteSpace(Username) || Username.Length > 64)
        {
            throw new InvalidOperationException($"The gateway username for terminal '{SerialNumber}' must be between 1 and 64 characters.");
        }

        if (!EnvironmentVariableNamePattern().IsMatch(PasswordEnvironmentVariable))
        {
            throw new InvalidOperationException($"The password environment variable for terminal '{SerialNumber}' is invalid.");
        }

        var password = Environment.GetEnvironmentVariable(PasswordEnvironmentVariable);
        if (string.IsNullOrEmpty(password) || password.Length > 64)
        {
            throw new InvalidOperationException($"The password environment variable '{PasswordEnvironmentVariable}' for terminal '{SerialNumber}' must contain between 1 and 64 characters.");
        }

        if (LoginPasswordDigest is not ("sha256" or "sha1"))
        {
            throw new InvalidOperationException($"Gateway:Terminals:{SerialNumber}:LoginPasswordDigest must be exactly 'sha256' or 'sha1'.");
        }
    }

    [GeneratedRegex("\\A[A-Za-z0-9._-]{1,160}\\z", RegexOptions.CultureInvariant)]
    private static partial Regex TerminalSerialPattern();

    [GeneratedRegex("\\A[A-Z][A-Z0-9_]*\\z", RegexOptions.CultureInvariant)]
    private static partial Regex EnvironmentVariableNamePattern();
}
