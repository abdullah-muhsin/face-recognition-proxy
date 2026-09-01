namespace PushSdkGateway;

public sealed class DeviceRegistry
{
    private readonly IReadOnlyDictionary<string, RegisteredTerminal> _terminals;

    public DeviceRegistry(GatewayOptions options)
    {
        _terminals = options.Terminals
            .Select(terminal => new RegisteredTerminal(
                terminal.SerialNumber,
                terminal.Username,
                Environment.GetEnvironmentVariable(terminal.PasswordEnvironmentVariable)!,
                terminal.LoginPasswordDigest))
            .ToDictionary(terminal => terminal.SerialNumber, StringComparer.Ordinal);
    }

    public bool TryGet(string serialNumber, out RegisteredTerminal terminal)
    {
        return _terminals.TryGetValue(serialNumber, out terminal!);
    }
}

public sealed record RegisteredTerminal(
    string SerialNumber,
    string Username,
    string Password,
    string LoginPasswordDigest);
