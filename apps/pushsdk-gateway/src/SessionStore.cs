using System.Collections.Concurrent;

namespace PushSdkGateway;

public sealed class SessionStore
{
    private readonly ConcurrentDictionary<string, DeviceSession> _sessions = new(StringComparer.Ordinal);
    private readonly ConcurrentDictionary<string, LoginLock> _loginLocks = new(StringComparer.Ordinal);

    public DeviceSession BeginAuthentication(RegisteredTerminal terminal, int iterations, int? encryptionSecurityVersion)
    {
        var session = DeviceSession.Create(terminal, iterations, encryptionSecurityVersion);
        _sessions[terminal.SerialNumber] = session;
        return session;
    }

    public bool TryGetAuthenticated(string terminalSerialNumber, out DeviceSession session)
    {
        if (_sessions.TryGetValue(terminalSerialNumber, out session!) && session.IsAuthenticated)
        {
            return true;
        }

        session = null!;
        return false;
    }

    public bool TryGetSession(string terminalSerialNumber, out DeviceSession session)
    {
        return _sessions.TryGetValue(terminalSerialNumber, out session!);
    }

    public void Remove(string terminalSerialNumber)
    {
        _sessions.TryRemove(terminalSerialNumber, out _);
    }

    public LoginLockStatus GetLoginLockStatus(string deviceAddress, DateTimeOffset now)
    {
        if (!_loginLocks.TryGetValue(deviceAddress, out var loginLock))
        {
            return LoginLockStatus.Unlocked(5);
        }

        lock (loginLock)
        {
            if (loginLock.LockedUntilUtc is { } lockedUntilUtc && lockedUntilUtc > now)
            {
                return LoginLockStatus.Locked((int)Math.Ceiling((lockedUntilUtc - now).TotalSeconds));
            }

            if (loginLock.LockedUntilUtc is not null)
            {
                loginLock.LockedUntilUtc = null;
                loginLock.FailedAttempts = 0;
            }

            return LoginLockStatus.Unlocked(Math.Max(0, 5 - loginLock.FailedAttempts));
        }
    }

    public LoginLockStatus RecordLoginFailure(string deviceAddress, DateTimeOffset now, int lockoutSeconds)
    {
        var loginLock = _loginLocks.GetOrAdd(deviceAddress, static _ => new LoginLock());

        lock (loginLock)
        {
            loginLock.FailedAttempts++;
            if (loginLock.FailedAttempts >= 5)
            {
                loginLock.FailedAttempts = 0;
                loginLock.LockedUntilUtc = now.AddSeconds(lockoutSeconds);
                return LoginLockStatus.Locked(lockoutSeconds);
            }

            return LoginLockStatus.Unlocked(5 - loginLock.FailedAttempts);
        }
    }

    public void RecordLoginSuccess(string deviceAddress)
    {
        _loginLocks.TryRemove(deviceAddress, out _);
    }

    private sealed class LoginLock
    {
        public int FailedAttempts { get; set; }

        public DateTimeOffset? LockedUntilUtc { get; set; }
    }
}

public sealed class DeviceSession
{
    private DeviceSession(
        RegisteredTerminal terminal,
        string challenge,
        string salt,
        int iterations,
        int? encryptionSecurityVersion,
        DateTimeOffset authInfoIssuedAtUtc)
    {
        Terminal = terminal;
        Challenge = challenge;
        Salt = salt;
        Iterations = iterations;
        EncryptionSecurityVersion = encryptionSecurityVersion;
        AuthInfoIssuedAtUtc = authInfoIssuedAtUtc;
    }

    public RegisteredTerminal Terminal { get; }

    public SemaphoreSlim SerialGate { get; } = new(1, 1);

    public string Challenge { get; set; }

    public string Salt { get; }

    public int Iterations { get; }

    public int? EncryptionSecurityVersion { get; }

    public DateTimeOffset AuthInfoIssuedAtUtc { get; }

    public bool LoginAttempted { get; set; }

    public bool IsAuthenticated { get; set; }

    public static DeviceSession Create(RegisteredTerminal terminal, int iterations, int? encryptionSecurityVersion)
    {
        return new DeviceSession(
            terminal,
            PushCrypto.RandomAlphaNumeric(64),
            PushCrypto.RandomAlphaNumeric(64),
            iterations,
            encryptionSecurityVersion,
            DateTimeOffset.UtcNow);
    }
}

public sealed record LoginLockStatus(bool IsLocked, int UnlockSeconds, int RemainingAttempts)
{
    public static LoginLockStatus Locked(int unlockSeconds) => new(true, unlockSeconds, 0);

    public static LoginLockStatus Unlocked(int remainingAttempts) => new(false, 0, remainingAttempts);
}
