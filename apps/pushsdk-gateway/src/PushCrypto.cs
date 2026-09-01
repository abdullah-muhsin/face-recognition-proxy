using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;

namespace PushSdkGateway;

public static partial class PushCrypto
{
    public static string RandomAlphaNumeric(int length)
    {
        const string alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
        var characters = new char[length];

        for (var index = 0; index < characters.Length; index++)
        {
            characters[index] = alphabet[RandomNumberGenerator.GetInt32(alphabet.Length)];
        }

        return new string(characters);
    }

    public static string RandomHex(int length)
    {
        if (length % 2 != 0)
        {
            throw new ArgumentOutOfRangeException(nameof(length), "A hexadecimal random string must have an even length.");
        }

        return Convert.ToHexString(RandomNumberGenerator.GetBytes(length / 2)).ToLowerInvariant();
    }

    public static string CalculateLoginPassword(DeviceSession session)
    {
        var passwordMaterial = HexSha256(session.Terminal.Username + session.Salt + session.Terminal.Password) + session.Challenge;
        var algorithm = session.Terminal.LoginPasswordDigest == "sha256"
            ? HashAlgorithmName.SHA256
            : HashAlgorithmName.SHA1;
        var key = Rfc2898DeriveBytes.Pbkdf2(
            passwordMaterial,
            Encoding.UTF8.GetBytes(session.Salt),
            session.Iterations,
            algorithm,
            64);

        return Convert.ToHexString(key).ToLowerInvariant();
    }

    public static string CalculateCustomAuth(DeviceSession session)
    {
        var passwordHash = HexSha256(session.Terminal.Username + session.Salt + session.Terminal.Password);
        return HexSha256(passwordHash + session.Challenge);
    }

    public static EncryptionContext? ParseEncryptionContext(HttpRequest request, DeviceSession session)
    {
        var security = request.Query["security"].ToString();
        var iv = request.Query["iv"].ToString();
        var random = request.Query["random"].ToString();

        if (security.Length == 0 && iv.Length == 0 && random.Length == 0)
        {
            if (request.Query.Count != 0)
            {
                throw new ProtocolException(400, "Unexpected query parameters.");
            }

            return null;
        }

        if (request.Query.Count != 3 || security.Length == 0 || iv.Length == 0 || random.Length == 0)
        {
            throw new ProtocolException(400, "The security, iv, and random query parameters must be provided together.");
        }

        if (security is not ("3" or "4"))
        {
            throw new ProtocolException(400, "The security query parameter must be 3 or 4.");
        }

        if (session.EncryptionSecurityVersion is null || security != session.EncryptionSecurityVersion.Value.ToString(CultureInfo.InvariantCulture))
        {
            throw new ProtocolException(400, "The security query parameter does not match the negotiated Push SDK security version.");
        }

        if (!Hex32Pattern().IsMatch(iv))
        {
            throw new ProtocolException(400, "The iv query parameter must be exactly 32 hexadecimal characters.");
        }

        if (Encoding.UTF8.GetByteCount(random) != 16)
        {
            throw new ProtocolException(400, "The random query parameter must be exactly 16 UTF-8 bytes.");
        }

        var keyMaterial = Rfc2898DeriveBytes.Pbkdf2(
            HexSha256(session.Terminal.Username + session.Salt + session.Terminal.Password) + random,
            Encoding.UTF8.GetBytes(session.Salt),
            session.Iterations,
            HashAlgorithmName.SHA256,
            64);

        var keyLength = security == "3" ? 16 : 32;
        return new EncryptionContext(keyMaterial[..keyLength], Convert.FromHexString(iv));
    }

    public static byte[] Decrypt(EncryptionContext encryption, byte[] cipherText)
    {
        try
        {
            using var aes = Aes.Create();
            aes.Key = encryption.Key;
            aes.IV = encryption.InitializationVector;
            aes.Mode = CipherMode.CBC;
            aes.Padding = PaddingMode.PKCS7;
            using var decryptor = aes.CreateDecryptor();
            var base64Data = decryptor.TransformFinalBlock(cipherText, 0, cipherText.Length);
            return Convert.FromBase64String(Encoding.UTF8.GetString(base64Data));
        }
        catch (CryptographicException exception)
        {
            throw new ProtocolException(400, "Encrypted request data could not be decrypted.", exception);
        }
        catch (FormatException exception)
        {
            throw new ProtocolException(400, "Encrypted request data did not contain base64 text.", exception);
        }
    }

    public static byte[] Encrypt(EncryptionContext encryption, string json)
    {
        using var aes = Aes.Create();
        aes.Key = encryption.Key;
        aes.IV = encryption.InitializationVector;
        aes.Mode = CipherMode.CBC;
        aes.Padding = PaddingMode.PKCS7;
        using var encryptor = aes.CreateEncryptor();
        var base64Data = Encoding.UTF8.GetBytes(Convert.ToBase64String(Encoding.UTF8.GetBytes(json)));
        return encryptor.TransformFinalBlock(base64Data, 0, base64Data.Length);
    }

    public static bool FixedTimeEquals(string expected, string provided)
    {
        return expected.Length == provided.Length
            && CryptographicOperations.FixedTimeEquals(Encoding.UTF8.GetBytes(expected), Encoding.UTF8.GetBytes(provided));
    }

    private static string HexSha256(string value)
    {
        return Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(value))).ToLowerInvariant();
    }

    [GeneratedRegex("\\A[0-9A-Fa-f]{32}\\z", RegexOptions.CultureInvariant)]
    private static partial Regex Hex32Pattern();
}

public sealed record EncryptionContext(byte[] Key, byte[] InitializationVector);

public sealed class ProtocolException : Exception
{
    public ProtocolException(int statusCode, string message, Exception? innerException = null)
        : base(message, innerException)
    {
        StatusCode = statusCode;
    }

    public int StatusCode { get; }
}
