using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;
using System.Xml.Linq;

namespace PushSdkGateway;

public sealed partial class AttendanceEventParser
{
    private static readonly UTF8Encoding StrictUtf8 = new(false, true);

    private readonly int _maxPictureBytes;

    public AttendanceEventParser(GatewayOptions options)
    {
        _maxPictureBytes = options.MaxPictureBytes;
    }

    public IReadOnlyList<ParsedVendorEvent> ParseBatch(string terminalSerialNumber, byte[] body)
    {
        using var document = ParseJson(body, "The event envelope is not valid JSON.");
        var root = document.RootElement;
        RequireObject(root, "The event envelope must be an object.");

        var eventCount = RequireInt(root, "eventNum");
        if (eventCount is < 0 or > 20)
        {
            throw new ProtocolException(400, "eventNum must be between 0 and 20.");
        }

        if (!root.TryGetProperty("eventList", out var eventList) || eventList.ValueKind != JsonValueKind.Array)
        {
            throw new ProtocolException(400, "eventList must be an array.");
        }

        if (eventList.GetArrayLength() != eventCount)
        {
            throw new ProtocolException(400, "eventNum must equal the number of entries in eventList.");
        }

        var parsedEvents = new List<ParsedVendorEvent>(eventCount);
        var eventIds = new HashSet<string>(StringComparer.Ordinal);
        foreach (var item in eventList.EnumerateArray())
        {
            RequireObject(item, "Every eventList item must be an object.");
            var eventId = RequireString(item, "UUID", 64);
            if (!VendorEventIdPattern().IsMatch(eventId))
            {
                throw new ProtocolException(400, "Every event UUID must contain only letters, digits, dots, underscores, colons, or hyphens.");
            }

            if (!eventIds.Add(eventId))
            {
                throw new ProtocolException(400, "eventList must not contain a duplicate UUID.");
            }

            var dataFormat = RequireString(item, "dataFormat", 32);
            if (!item.TryGetProperty("data", out var dataValue) || dataValue.ValueKind != JsonValueKind.String)
            {
                throw new ProtocolException(400, $"Event '{eventId}' data must be a base64 string.");
            }

            var encodedData = dataValue.GetString()!;
            byte[] rawData;
            try
            {
                rawData = Convert.FromBase64String(encodedData);
            }
            catch (FormatException exception)
            {
                throw new ProtocolException(400, $"Event '{eventId}' data is not valid base64.", exception);
            }

            var parsed = ParseEventData(terminalSerialNumber, eventId, dataFormat, rawData);
            parsedEvents.Add(parsed with { RawPayloadSha256 = Convert.ToHexString(SHA256.HashData(rawData)).ToLowerInvariant() });
        }

        return parsedEvents;
    }

    private ParsedVendorEvent ParseEventData(string terminalSerialNumber, string eventId, string dataFormat, byte[] rawData)
    {
        return dataFormat switch
        {
            "jsonData" => ParseJsonEvent(terminalSerialNumber, eventId, rawData),
            "xmlData" => ParseXmlEvent(terminalSerialNumber, eventId, rawData),
            "boundaryData" => ParseBoundaryEvent(terminalSerialNumber, eventId, rawData),
            "noData" when rawData.Length == 0 => ParsedVendorEvent.Ignored(eventId, "noData"),
            "noData" => throw new ProtocolException(422, $"Event '{eventId}' noData payload must be empty."),
            _ => throw new ProtocolException(400, $"Event '{eventId}' has an unsupported dataFormat."),
        };
    }

    private ParsedVendorEvent ParseJsonEvent(string terminalSerialNumber, string eventId, byte[] rawData)
    {
        using var document = ParseJson(rawData, $"Event '{eventId}' jsonData is not valid JSON.");
        return TranslateJsonEvent(terminalSerialNumber, eventId, document.RootElement, null, "jsonData");
    }

    private ParsedVendorEvent ParseXmlEvent(string terminalSerialNumber, string eventId, byte[] rawData)
    {
        XDocument document;
        try
        {
            document = XDocument.Parse(StrictUtf8.GetString(rawData), LoadOptions.None);
        }
        catch (Exception exception) when (exception is System.Xml.XmlException or DecoderFallbackException)
        {
            throw new ProtocolException(400, $"Event '{eventId}' xmlData is not valid UTF-8 XML.", exception);
        }

        var root = document.Root ?? throw new ProtocolException(400, $"Event '{eventId}' xmlData has no root element.");
        if (root.Name.LocalName != "EventNotificationAlert")
        {
            throw new ProtocolException(422, $"Event '{eventId}' XML root must be EventNotificationAlert.");
        }

        if (RequiredElementValue(root, "eventType", eventId) != "AccessControllerEvent")
        {
            return ParsedVendorEvent.Ignored(eventId, "xmlData");
        }

        if (RequiredElementValue(root, "eventState", eventId) != "active")
        {
            return ParsedVendorEvent.Ignored(eventId, "xmlData");
        }

        var eventData = RequiredElement(root, "AccessControllerEvent", eventId);
        return ParsedVendorEvent.ForDelivery(
            eventId,
            "xmlData",
            new CanonicalAttendanceEvent(
                terminalSerialNumber,
                ParseTimestamp(RequiredElementValue(root, "dateTime", eventId), eventId),
                RequiredElementValue(eventData, "employeeNoString", eventId, 80),
                OptionalElementValue(eventData, "name", eventId, 160),
                RequiredElementValue(eventData, "currentVerifyMode", eventId, 80),
                RequiredElementValue(eventData, "attendanceStatus", eventId, 80),
                OptionalElementInt(eventData, "statusValue", eventId),
                false),
            null);
    }

    private ParsedVendorEvent ParseBoundaryEvent(string terminalSerialNumber, string eventId, byte[] rawData)
    {
        var parts = ParseBoundary(rawData, eventId);
        var metadataParts = parts.Where(part => HasMediaType(part.ContentType, "application/json") || HasMediaType(part.ContentType, "application/xml")).ToList();
        var pictureParts = parts.Where(part => HasMediaType(part.ContentType, "image/jpeg")).ToList();

        if (metadataParts.Count != 1 || pictureParts.Count > 1 || parts.Count != metadataParts.Count + pictureParts.Count)
        {
            throw new ProtocolException(422, $"Event '{eventId}' boundaryData must contain exactly one JSON or XML metadata part and at most one JPEG picture part.");
        }

        var picture = pictureParts.Count == 1 ? pictureParts[0].Content : null;
        if (picture is not null)
        {
            ValidateJpeg(picture, eventId);
        }

        var metadata = metadataParts[0];
        return MediaType(metadata.ContentType) switch
        {
            "application/json" => ParseBoundaryJson(terminalSerialNumber, eventId, metadata.Content, picture),
            "application/xml" => ParseBoundaryXml(terminalSerialNumber, eventId, metadata.Content, picture),
            _ => throw new InvalidOperationException("Boundary metadata content type was validated before dispatch."),
        };
    }

    private ParsedVendorEvent ParseBoundaryJson(string terminalSerialNumber, string eventId, byte[] metadata, byte[]? picture)
    {
        using var document = ParseJson(metadata, $"Event '{eventId}' boundary JSON metadata is not valid JSON.");
        return TranslateJsonEvent(terminalSerialNumber, eventId, document.RootElement, picture, "boundaryData");
    }

    private ParsedVendorEvent ParseBoundaryXml(string terminalSerialNumber, string eventId, byte[] metadata, byte[]? picture)
    {
        var parsed = ParseXmlEvent(terminalSerialNumber, eventId, metadata);
        return parsed.Delivery is null
            ? parsed with { DataFormat = "boundaryData" }
            : parsed with
            {
                DataFormat = "boundaryData",
                Delivery = parsed.Delivery with
                {
                    Event = parsed.Delivery.Event with { PictureExpected = picture is not null },
                    Picture = picture,
                },
            };
    }

    private ParsedVendorEvent TranslateJsonEvent(
        string terminalSerialNumber,
        string eventId,
        JsonElement root,
        byte[]? picture,
        string dataFormat)
    {
        RequireObject(root, $"Event '{eventId}' payload must be a JSON object.");
        if (RequireString(root, "eventType", 80) != "AccessControllerEvent")
        {
            return ParsedVendorEvent.Ignored(eventId, dataFormat);
        }

        if (RequireString(root, "eventState", 80) != "active")
        {
            return ParsedVendorEvent.Ignored(eventId, dataFormat);
        }

        if (!root.TryGetProperty("AccessControllerEvent", out var accessControllerEvent))
        {
            throw new ProtocolException(422, $"Event '{eventId}' is missing AccessControllerEvent.");
        }

        RequireObject(accessControllerEvent, $"Event '{eventId}' AccessControllerEvent must be an object.");
        return ParsedVendorEvent.ForDelivery(
            eventId,
            dataFormat,
            new CanonicalAttendanceEvent(
                terminalSerialNumber,
                ParseTimestamp(RequireString(root, "dateTime", 32), eventId),
                RequireString(accessControllerEvent, "employeeNoString", 80),
                OptionalString(accessControllerEvent, "name", 160),
                RequireString(accessControllerEvent, "currentVerifyMode", 80),
                RequireString(accessControllerEvent, "attendanceStatus", 80),
                OptionalInt(accessControllerEvent, "statusValue"),
                picture is not null),
            picture);
    }

    private static IReadOnlyList<BoundaryPart> ParseBoundary(byte[] rawData, string eventId)
    {
        var headerTerminator = "\r\n\r\n"u8.ToArray();
        var headerEnd = IndexOf(rawData, headerTerminator, 0);
        if (headerEnd < 0)
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData is missing the HTTP header terminator.");
        }

        var headerLines = Encoding.ASCII.GetString(rawData, 0, headerEnd).Split("\r\n", StringSplitOptions.None);
        IEnumerable<string> contentHeaders;
        if (headerLines.FirstOrDefault() == "HTTP/1.1 200 OK")
        {
            // Accepted for compatibility with the later vendor demo, which
            // wraps the multipart body in a serialized HTTP response.
            contentHeaders = headerLines.Skip(1);
        }
        else if (headerLines.FirstOrDefault()?.StartsWith("Content-Type:", StringComparison.OrdinalIgnoreCase) == true)
        {
            // DS-K1T342MFWX-E1 sends the documented direct multipart form:
            // Content-Type and Content-Length headers, followed by its body.
            contentHeaders = headerLines;
        }
        else
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData must start with Content-Type or HTTP/1.1 200 OK.");
        }

        var headers = ParseHeaders(contentHeaders, eventId);
        if (!headers.TryGetValue("Content-Type", out var contentType))
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData must contain a Content-Type header.");
        }

        var contentOffset = headerEnd + headerTerminator.Length;
        var contentEnd = rawData.Length;
        if (headers.TryGetValue("Content-Length", out var contentLengthText))
        {
            if (!int.TryParse(contentLengthText, NumberStyles.None, CultureInfo.InvariantCulture, out var contentLength)
                || contentLength < 0
                || rawData.Length - contentOffset != contentLength)
            {
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData Content-Length does not match its payload.");
            }
        }

        var boundary = ParseBoundaryValue(contentType, eventId);
        var delimiter = Encoding.ASCII.GetBytes("--" + boundary);
        var terminalDelimiter = Encoding.ASCII.GetBytes("--" + boundary + "--");
        var position = contentOffset;

        if (!StartsWith(rawData, position, delimiter))
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData must start with its multipart delimiter.");
        }

        var parts = new List<BoundaryPart>();
        while (position < contentEnd)
        {
            if (StartsWith(rawData, position, terminalDelimiter))
            {
                var terminalEnd = position + terminalDelimiter.Length;
                if (terminalEnd == contentEnd)
                {
                    break;
                }
                if (terminalEnd + 2 == contentEnd
                    && rawData[terminalEnd] == '\r'
                    && rawData[terminalEnd + 1] == '\n')
                {
                    break;
                }
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData contains bytes after the terminal delimiter.");
            }

            if (!StartsWith(rawData, position, delimiter))
            {
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData part delimiter is invalid.");
            }

            position += delimiter.Length;
            RequireCrLf(rawData, ref position, eventId);
            var partHeaderEnd = IndexOf(rawData, headerTerminator, position);
            if (partHeaderEnd < 0)
            {
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData part is missing its header terminator.");
            }

            var partHeaders = ParseHeaders(Encoding.ASCII.GetString(rawData, position, partHeaderEnd - position).Split("\r\n"), eventId);
            if (!partHeaders.TryGetValue("Content-Type", out var partContentType))
            {
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData part is missing Content-Type.");
            }

            position = partHeaderEnd + headerTerminator.Length;
            byte[] content;
            if (partHeaders.TryGetValue("Content-Length", out var partLengthText))
            {
                if (!int.TryParse(partLengthText, NumberStyles.None, CultureInfo.InvariantCulture, out var partLength)
                    || partLength < 0
                    || position + partLength > contentEnd)
                {
                    throw new ProtocolException(400, $"Event '{eventId}' boundaryData part exceeds the declared payload length.");
                }

                content = rawData.AsSpan(position, partLength).ToArray();
                position += partLength;
                RequireCrLf(rawData, ref position, eventId);
            }
            else
            {
                var nextDelimiter = IndexOfBoundaryDelimiter(rawData, delimiter, position);
                if (nextDelimiter < 0 || nextDelimiter >= contentEnd)
                {
                    throw new ProtocolException(400, $"Event '{eventId}' boundaryData part is missing its next delimiter.");
                }

                content = rawData.AsSpan(position, nextDelimiter - position).ToArray();
                position = nextDelimiter + 2;
            }

            parts.Add(new BoundaryPart(partContentType, content));
        }

        return parts;
    }

    private static Dictionary<string, string> ParseHeaders(IEnumerable<string> lines, string eventId)
    {
        var headers = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
        foreach (var line in lines)
        {
            var separator = line.IndexOf(':');
            if (separator <= 0 || separator == line.Length - 1)
            {
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData contains an invalid header line.");
            }

            var name = line[..separator];
            var value = line[(separator + 1)..].TrimStart(' ');
            if (!headers.TryAdd(name, value))
            {
                throw new ProtocolException(400, $"Event '{eventId}' boundaryData contains a duplicate '{name}' header.");
            }
        }

        return headers;
    }

    private static string ParseBoundaryValue(string contentType, string eventId)
    {
        const string prefix = "multipart/form-data; boundary=";
        if (!contentType.StartsWith(prefix, StringComparison.Ordinal) || contentType.Length == prefix.Length)
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData Content-Type must be multipart/form-data with a boundary.");
        }

        var boundary = contentType[prefix.Length..];
        if (!BoundaryPattern().IsMatch(boundary))
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData boundary is invalid.");
        }

        return boundary;
    }

    private static bool HasMediaType(string contentType, string expectedMediaType)
    {
        return string.Equals(MediaType(contentType), expectedMediaType, StringComparison.OrdinalIgnoreCase);
    }

    private static string MediaType(string contentType)
    {
        return contentType.Split(';', 2, StringSplitOptions.TrimEntries)[0];
    }

    private void ValidateJpeg(byte[] picture, string eventId)
    {
        if (picture.Length < 4 || picture.Length > _maxPictureBytes
            || !picture.AsSpan(0, 3).SequenceEqual(new byte[] { 0xff, 0xd8, 0xff })
            || !picture.AsSpan(^2).SequenceEqual(new byte[] { 0xff, 0xd9 }))
        {
            throw new ProtocolException(422, $"Event '{eventId}' picture must be a complete JPEG no larger than {_maxPictureBytes} bytes.");
        }
    }

    private static void RequireCrLf(byte[] source, ref int position, string eventId)
    {
        if (position + 2 > source.Length || source[position] != '\r' || source[position + 1] != '\n')
        {
            throw new ProtocolException(400, $"Event '{eventId}' boundaryData is missing a required CRLF.");
        }

        position += 2;
    }

    private static bool StartsWith(byte[] source, int offset, byte[] value)
    {
        return offset >= 0 && offset + value.Length <= source.Length && source.AsSpan(offset, value.Length).SequenceEqual(value);
    }

    private static int IndexOf(byte[] source, byte[] value, int startIndex)
    {
        var offset = source.AsSpan(startIndex).IndexOf(value);
        return offset >= 0 ? startIndex + offset : -1;
    }

    private static int IndexOfBoundaryDelimiter(byte[] source, byte[] delimiter, int startIndex)
    {
        var prefixedDelimiter = new byte[delimiter.Length + 2];
        prefixedDelimiter[0] = (byte)'\r';
        prefixedDelimiter[1] = (byte)'\n';
        delimiter.CopyTo(prefixedDelimiter, 2);
        return IndexOf(source, prefixedDelimiter, startIndex);
    }

    private static JsonDocument ParseJson(byte[] value, string errorMessage)
    {
        try
        {
            return JsonDocument.Parse(value);
        }
        catch (JsonException exception)
        {
            throw new ProtocolException(400, errorMessage, exception);
        }
    }

    private static DateTimeOffset ParseTimestamp(string value, string eventId)
    {
        if (!Rfc3339Pattern().IsMatch(value))
        {
            throw new ProtocolException(422, $"Event '{eventId}' dateTime must be an RFC 3339 timestamp with second precision and a UTC offset.");
        }

        var isUtc = value.EndsWith('Z');
        var format = isUtc ? "yyyy-MM-dd'T'HH:mm:ss'Z'" : "yyyy-MM-dd'T'HH:mm:sszzz";
        var styles = isUtc ? DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal : DateTimeStyles.None;
        if (!DateTimeOffset.TryParseExact(value, format, CultureInfo.InvariantCulture, styles, out var timestamp))
        {
            throw new ProtocolException(422, $"Event '{eventId}' dateTime is not a valid RFC 3339 timestamp.");
        }

        return timestamp.ToUniversalTime();
    }

    private static string RequireString(JsonElement objectElement, string property, int maximumLength)
    {
        if (!objectElement.TryGetProperty(property, out var value) || value.ValueKind != JsonValueKind.String)
        {
            throw new ProtocolException(422, $"The '{property}' field is required and must be a string.");
        }

        var text = value.GetString()!;
        if (text.Length == 0 || text.Length > maximumLength)
        {
            throw new ProtocolException(422, $"The '{property}' field must contain between 1 and {maximumLength} characters.");
        }

        return text;
    }

    private static string? OptionalString(JsonElement objectElement, string property, int maximumLength)
    {
        if (!objectElement.TryGetProperty(property, out var value) || value.ValueKind == JsonValueKind.Null)
        {
            return null;
        }

        if (value.ValueKind != JsonValueKind.String || value.GetString()!.Length > maximumLength)
        {
            throw new ProtocolException(422, $"The '{property}' field must be a string no longer than {maximumLength} characters when present.");
        }

        return value.GetString();
    }

    private static int? OptionalInt(JsonElement objectElement, string property)
    {
        if (!objectElement.TryGetProperty(property, out var value) || value.ValueKind == JsonValueKind.Null)
        {
            return null;
        }

        if (!value.TryGetInt32(out var integer) || integer is < 0 or > 65535)
        {
            throw new ProtocolException(422, $"The '{property}' field must be an integer between 0 and 65535 when present.");
        }

        return integer;
    }

    private static int RequireInt(JsonElement objectElement, string property)
    {
        if (!objectElement.TryGetProperty(property, out var value) || !value.TryGetInt32(out var integer))
        {
            throw new ProtocolException(400, $"The '{property}' field is required and must be an integer.");
        }

        return integer;
    }

    private static void RequireObject(JsonElement element, string message)
    {
        if (element.ValueKind != JsonValueKind.Object)
        {
            throw new ProtocolException(422, message);
        }
    }

    private static XElement RequiredElement(XElement parent, string name, string eventId)
    {
        return parent.Elements().SingleOrDefault(element => element.Name.LocalName == name)
            ?? throw new ProtocolException(422, $"Event '{eventId}' XML payload is missing {name}.");
    }

    private static string RequiredElementValue(XElement parent, string name, string eventId, int maximumLength = 32)
    {
        var value = RequiredElement(parent, name, eventId).Value;
        if (value.Length == 0 || value.Length > maximumLength)
        {
            throw new ProtocolException(422, $"Event '{eventId}' XML {name} must contain between 1 and {maximumLength} characters.");
        }

        return value;
    }

    private static string? OptionalElementValue(XElement parent, string name, string eventId, int maximumLength)
    {
        var elements = parent.Elements().Where(element => element.Name.LocalName == name).ToList();
        if (elements.Count == 0)
        {
            return null;
        }

        if (elements.Count != 1 || elements[0].Value.Length > maximumLength)
        {
            throw new ProtocolException(422, $"Event '{eventId}' XML {name} must occur once and be no longer than {maximumLength} characters when present.");
        }

        return elements[0].Value;
    }

    private static int? OptionalElementInt(XElement parent, string name, string eventId)
    {
        var value = OptionalElementValue(parent, name, eventId, 5);
        if (value is null)
        {
            return null;
        }

        if (!int.TryParse(value, NumberStyles.None, CultureInfo.InvariantCulture, out var number) || number is < 0 or > 65535)
        {
            throw new ProtocolException(422, $"Event '{eventId}' XML {name} must be an integer between 0 and 65535 when present.");
        }

        return number;
    }

    [GeneratedRegex("\\A[A-Za-z0-9._:-]{1,64}\\z", RegexOptions.CultureInvariant)]
    private static partial Regex VendorEventIdPattern();

    [GeneratedRegex("\\A[A-Za-z0-9'()+_,./:=? -]{1,70}\\z", RegexOptions.CultureInvariant)]
    private static partial Regex BoundaryPattern();

    [GeneratedRegex("\\A\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:Z|[+-]\\d{2}:\\d{2})\\z", RegexOptions.CultureInvariant)]
    private static partial Regex Rfc3339Pattern();
}

public sealed record ParsedVendorEvent(
    string VendorEventId,
    string DataFormat,
    string RawPayloadSha256,
    DeliveryEvent? Delivery)
{
    public static ParsedVendorEvent Ignored(string vendorEventId, string dataFormat) => new(vendorEventId, dataFormat, string.Empty, null);

    public static ParsedVendorEvent ForDelivery(string vendorEventId, string dataFormat, CanonicalAttendanceEvent attendanceEvent, byte[]? picture) =>
        new(vendorEventId, dataFormat, string.Empty, new DeliveryEvent(attendanceEvent, picture));
}

public sealed record DeliveryEvent(CanonicalAttendanceEvent Event, byte[]? Picture);

public sealed record CanonicalAttendanceEvent(
    string TerminalSerialNumber,
    DateTimeOffset OccurredAtUtc,
    string EmployeeNumber,
    string? EmployeeName,
    string VerificationMethod,
    string AttendanceStatus,
    int? StatusValue,
    bool PictureExpected);

public sealed record BoundaryPart(string ContentType, byte[] Content);
