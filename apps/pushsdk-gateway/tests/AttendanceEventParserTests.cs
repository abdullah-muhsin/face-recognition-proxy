using System.Text;

namespace PushSdkGateway.Tests;

public sealed class AttendanceEventParserTests
{
    [Fact]
    public void ParsesActiveAccessControllerJsonEventIntoTheReceiverContract()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var body = TestProtocol.BuildEventEnvelope("event-json-1", "jsonData", TestProtocol.AccessEventJson());

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));
        var delivery = Assert.IsType<DeliveryEvent>(parsed.Delivery);

        Assert.Equal("event-json-1", parsed.VendorEventId);
        Assert.Equal("jsonData", parsed.DataFormat);
        Assert.Equal(TestEnvironment.TerminalSerialNumber, delivery.Event.TerminalSerialNumber);
        Assert.Equal("1001", delivery.Event.EmployeeNumber);
        Assert.Equal("Amina Karim", delivery.Event.EmployeeName);
        Assert.Equal("face", delivery.Event.VerificationMethod);
        Assert.Equal("checkIn", delivery.Event.AttendanceStatus);
        Assert.Equal(1, delivery.Event.StatusValue);
        Assert.False(delivery.Event.PictureExpected);
        Assert.Null(delivery.Picture);
        Assert.Equal(new DateTimeOffset(2026, 9, 1, 5, 15, 30, TimeSpan.Zero), delivery.Event.OccurredAtUtc);
    }

    [Fact]
    public void ParsesBoundaryEventWithItsReceivedJpegOnly()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var metadata = TestProtocol.AccessEventJson("2002");
        var picture = new byte[] { 0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x00, 0x00, 0xff, 0xd9 };
        var rawBoundary = BuildBoundaryPayload(metadata, picture);
        var body = TestProtocol.BuildEventEnvelope("event-boundary-1", "boundaryData", rawBoundary);

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));
        var delivery = Assert.IsType<DeliveryEvent>(parsed.Delivery);

        Assert.Equal("boundaryData", parsed.DataFormat);
        Assert.True(delivery.Event.PictureExpected);
        Assert.Equal(picture, delivery.Picture);
        Assert.Equal("2002", delivery.Event.EmployeeNumber);
    }

    [Fact]
    public void ParsesTheDirectBoundaryEnvelopeSentByTheTerminal()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var metadata = TestProtocol.AccessEventJson("2003");
        var rawBoundary = BuildBoundaryPayload(
            metadata,
            null,
            includeHttpStatusLine: false,
            includeContentLengths: false,
            initialBlankLines: 1);
        var body = TestProtocol.BuildEventEnvelope("event-direct-boundary-1", "boundaryData", rawBoundary);

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));
        var delivery = Assert.IsType<DeliveryEvent>(parsed.Delivery);

        Assert.Equal("2003", delivery.Event.EmployeeNumber);
        Assert.False(delivery.Event.PictureExpected);
        Assert.Null(delivery.Picture);
    }

    [Fact]
    public void ParsesTheTerminalHeaderlessBoundaryMetadata()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var metadata = TestProtocol.AccessEventJson("2004");
        var rawBoundary = BuildBoundaryPayload(
            metadata,
            null,
            includeHttpStatusLine: false,
            includeContentLengths: false,
            includePartHeaders: false,
            includeTerminalDelimiter: false);
        var body = TestProtocol.BuildEventEnvelope("event-headerless-boundary-1", "boundaryData", rawBoundary);

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));
        var delivery = Assert.IsType<DeliveryEvent>(parsed.Delivery);

        Assert.Equal("2004", delivery.Event.EmployeeNumber);
        Assert.False(delivery.Event.PictureExpected);
        Assert.Null(delivery.Picture);
    }

    [Fact]
    public void AcknowledgesAnOpaqueTerminalBoundaryEntry()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var rawBoundary = BuildBoundaryPayload(
            new byte[] { 0xff, 0xd8, 0xff, 0xe0, 0x00 },
            null,
            includeHttpStatusLine: false,
            includeContentLengths: false,
            includePartHeaders: false,
            includeTerminalDelimiter: false);
        var body = TestProtocol.BuildEventEnvelope("event-opaque-boundary-1", "boundaryData", rawBoundary);

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));

        Assert.Equal("boundaryData", parsed.DataFormat);
        Assert.Null(parsed.Delivery);
    }

    [Fact]
    public void RecordsAnActiveAccessEventWithoutAnAttendanceScheduleStatus()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var raw = Encoding.UTF8.GetBytes("""
            {
              "eventType":"AccessControllerEvent",
              "eventState":"active",
              "dateTime":"2026-09-01T08:15:30Z",
              "AccessControllerEvent":{"employeeNoString":"2005","currentVerifyMode":"face"}
            }
            """);
        var body = TestProtocol.BuildEventEnvelope("event-free-attendance-1", "jsonData", raw);

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));
        var delivery = Assert.IsType<DeliveryEvent>(parsed.Delivery);

        Assert.Equal("2005", delivery.Event.EmployeeNumber);
        Assert.Equal("undefined", delivery.Event.AttendanceStatus);
        Assert.Null(delivery.Event.StatusValue);
    }

    [Fact]
    public void RejectsAnAccessEventMissingItsRequiredEmployeeNumber()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var raw = Encoding.UTF8.GetBytes("""
            {
              "eventType":"AccessControllerEvent",
              "eventState":"active",
              "dateTime":"2026-09-01T08:15:30Z",
              "AccessControllerEvent":{"currentVerifyMode":"face","attendanceStatus":"checkIn"}
            }
            """);
        var body = TestProtocol.BuildEventEnvelope("event-invalid-1", "jsonData", raw);

        var exception = Assert.Throws<ProtocolException>(() => parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));

        Assert.Equal(422, exception.StatusCode);
    }

    [Fact]
    public void AcknowledgesAndDeduplicatesTheDocumentedNoDataEventFormat()
    {
        using var environment = new TestEnvironment();
        var parser = new AttendanceEventParser(environment.CreateOptions());
        var body = TestProtocol.BuildEventEnvelope("event-nodata-1", "noData", Array.Empty<byte>());

        var parsed = Assert.Single(parser.ParseBatch(TestEnvironment.TerminalSerialNumber, body));

        Assert.Equal("noData", parsed.DataFormat);
        Assert.Null(parsed.Delivery);
    }

    private static byte[] BuildBoundaryPayload(
        byte[] metadata,
        byte[]? picture,
        bool includeHttpStatusLine = true,
        bool includeContentLengths = true,
        int initialBlankLines = 0,
        bool includePartHeaders = true,
        bool includeTerminalDelimiter = true)
    {
        const string boundary = "hikvision-boundary-001";
        var multipart = new MemoryStream();
        WriteAscii(multipart, $"--{boundary}\r\n");
        if (includePartHeaders)
        {
            WriteAscii(multipart, "Content-Disposition: form-data; name=\"metadata\"; filename=\"event.json\"\r\n");
            WriteAscii(multipart, "Content-Type: application/json\r\n");
            if (includeContentLengths)
            {
                WriteAscii(multipart, $"Content-Length: {metadata.Length}\r\n");
            }
            WriteAscii(multipart, "\r\n");
        }
        multipart.Write(metadata);
        if (includePartHeaders)
        {
            WriteAscii(multipart, "\r\n");
        }
        if (picture is not null)
        {
            WriteAscii(multipart, $"--{boundary}\r\n");
            WriteAscii(multipart, "Content-Disposition: form-data; name=\"picture\"; filename=\"picture.jpg\"\r\n");
            WriteAscii(multipart, "Content-Type: image/jpeg\r\n");
            if (includeContentLengths)
            {
                WriteAscii(multipart, $"Content-Length: {picture.Length}\r\n");
            }
            WriteAscii(multipart, "\r\n");
            multipart.Write(picture);
            WriteAscii(multipart, "\r\n");
        }
        if (includeTerminalDelimiter)
        {
            WriteAscii(multipart, $"--{boundary}--");
        }

        var multipartBytes = multipart.ToArray();
        using var payload = new MemoryStream();
        if (includeHttpStatusLine)
        {
            WriteAscii(payload, "HTTP/1.1 200 OK\r\n");
        }
        WriteAscii(payload, $"Content-Type: multipart/form-data; boundary={boundary}\r\n");
        if (includeContentLengths)
        {
            WriteAscii(payload, $"Content-Length: {multipartBytes.Length}\r\n");
        }
        WriteAscii(payload, "\r\n");
        for (var index = 0; index < initialBlankLines; index++)
        {
            WriteAscii(payload, "\r\n");
        }
        payload.Write(multipartBytes);
        return payload.ToArray();
    }

    private static void WriteAscii(Stream destination, string value)
    {
        destination.Write(Encoding.ASCII.GetBytes(value));
    }
}
