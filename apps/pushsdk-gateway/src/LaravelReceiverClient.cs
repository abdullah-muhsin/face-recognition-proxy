using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace PushSdkGateway;

public sealed partial class LaravelReceiverClient
{
    private readonly HttpClient _httpClient;
    private readonly string _bearerToken;

    public LaravelReceiverClient(HttpClient httpClient, GatewayOptions options)
    {
        _httpClient = httpClient;
        _bearerToken = Environment.GetEnvironmentVariable(options.Laravel.BearerTokenEnvironmentVariable)!;
    }

    public async Task DeliverAsync(LeasedDelivery delivery, CancellationToken cancellationToken)
    {
        using var createRequest = new HttpRequestMessage(HttpMethod.Post, "/api/internal/push-sdk/attendance-records")
        {
            Content = new StringContent(delivery.PayloadJson, Encoding.UTF8, "application/json"),
        };
        createRequest.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _bearerToken);

        using var createResponse = await _httpClient.SendAsync(createRequest, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        var createBody = await createResponse.Content.ReadAsByteArrayAsync(cancellationToken);
        if (createResponse.StatusCode is not (System.Net.HttpStatusCode.OK or System.Net.HttpStatusCode.Created))
        {
            throw new LaravelDeliveryException($"Laravel record creation returned HTTP {(int)createResponse.StatusCode}: {ResponseSummary(createBody)}");
        }

        using var createDocument = ParseResponse(createBody, "Laravel record creation did not return JSON.");
        var root = createDocument.RootElement;
        RequireObject(root, "Laravel record creation response must be an object.");
        var pictureUploadRequired = RequireBoolean(root, "picture_upload_required");

        if (delivery.Picture is null)
        {
            if (pictureUploadRequired)
            {
                throw new LaravelDeliveryException("Laravel requested a picture for an event that has no persisted picture.");
            }

            return;
        }

        if (!pictureUploadRequired)
        {
            if (!RequireBoolean(root, "picture_stored"))
            {
                throw new LaravelDeliveryException("Laravel did not request or confirm storage of the persisted picture.");
            }

            return;
        }

        var uploadPath = RequireString(root, "picture_upload_path");
        if (!PictureUploadPathPattern().IsMatch(uploadPath))
        {
            throw new LaravelDeliveryException("Laravel returned an invalid picture upload path.");
        }

        using var pictureRequest = new HttpRequestMessage(HttpMethod.Put, uploadPath)
        {
            Content = new ByteArrayContent(delivery.Picture),
        };
        pictureRequest.Content.Headers.ContentType = new MediaTypeHeaderValue("image/jpeg");
        pictureRequest.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _bearerToken);

        using var pictureResponse = await _httpClient.SendAsync(pictureRequest, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        var pictureResponseBody = await pictureResponse.Content.ReadAsByteArrayAsync(cancellationToken);
        if (!pictureResponse.IsSuccessStatusCode)
        {
            throw new LaravelDeliveryException($"Laravel picture upload returned HTTP {(int)pictureResponse.StatusCode}: {ResponseSummary(pictureResponseBody)}");
        }

        using var pictureDocument = ParseResponse(pictureResponseBody, "Laravel picture upload did not return JSON.");
        if (!RequireBoolean(pictureDocument.RootElement, "picture_stored"))
        {
            throw new LaravelDeliveryException("Laravel picture upload did not confirm picture storage.");
        }
    }

    private static JsonDocument ParseResponse(byte[] body, string message)
    {
        try
        {
            return JsonDocument.Parse(body);
        }
        catch (JsonException exception)
        {
            throw new LaravelDeliveryException(message, exception);
        }
    }

    private static void RequireObject(JsonElement element, string message)
    {
        if (element.ValueKind != JsonValueKind.Object)
        {
            throw new LaravelDeliveryException(message);
        }
    }

    private static bool RequireBoolean(JsonElement objectElement, string property)
    {
        if (!objectElement.TryGetProperty(property, out var value) || value.ValueKind is not (JsonValueKind.True or JsonValueKind.False))
        {
            throw new LaravelDeliveryException($"Laravel response is missing boolean '{property}'.");
        }

        return value.GetBoolean();
    }

    private static string RequireString(JsonElement objectElement, string property)
    {
        if (!objectElement.TryGetProperty(property, out var value) || value.ValueKind != JsonValueKind.String || string.IsNullOrEmpty(value.GetString()))
        {
            throw new LaravelDeliveryException($"Laravel response is missing string '{property}'.");
        }

        return value.GetString()!;
    }

    private static string ResponseSummary(byte[] responseBody)
    {
        return Encoding.UTF8.GetString(responseBody, 0, Math.Min(responseBody.Length, 500)).Replace('\n', ' ').Replace('\r', ' ');
    }

    [GeneratedRegex("\\A/api/internal/push-sdk/attendance-records/[1-9][0-9]*/picture\\z", RegexOptions.CultureInvariant)]
    private static partial Regex PictureUploadPathPattern();
}

public sealed class LaravelDeliveryException : Exception
{
    public LaravelDeliveryException(string message, Exception? innerException = null)
        : base(message, innerException)
    {
    }
}
