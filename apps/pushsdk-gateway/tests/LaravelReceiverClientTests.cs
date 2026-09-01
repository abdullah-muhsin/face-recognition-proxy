using System.Net;
using System.Net.Http.Headers;
using System.Text;

namespace PushSdkGateway.Tests;

public sealed class LaravelReceiverClientTests
{
    [Fact]
    public async Task PostsTheExactRecordThenUploadsTheReceivedJpeg()
    {
        using var environment = new TestEnvironment();
        var options = environment.CreateOptions();
        options.Validate();
        var handler = new RecordingHandler(
            "{\"picture_upload_required\":true,\"picture_upload_path\":\"/api/internal/push-sdk/attendance-records/42/picture\"}",
            "{\"picture_stored\":true}");
        using var httpClient = new HttpClient(handler) { BaseAddress = options.Laravel.ParseBaseUri() };
        var client = new LaravelReceiverClient(httpClient, options);
        var picture = new byte[] { 0xff, 0xd8, 0xff, 0xe0, 0xff, 0xd9 };

        await client.DeliverAsync(new LeasedDelivery(
            1,
            TestEnvironment.TerminalSerialNumber,
            "vendor-event-42",
            "{\"schema\":\"attendance.push-sdk.gateway.v1\"}",
            picture,
            1),
            CancellationToken.None);

        Assert.Equal(2, handler.Requests.Count);
        Assert.Equal(HttpMethod.Post, handler.Requests[0].Method);
        Assert.Equal("/api/internal/push-sdk/attendance-records", handler.Requests[0].PathAndQuery);
        Assert.Equal("Bearer", handler.Requests[0].AuthorizationScheme);
        Assert.Equal("application/json", handler.Requests[0].ContentType);
        Assert.Equal(HttpMethod.Put, handler.Requests[1].Method);
        Assert.Equal("/api/internal/push-sdk/attendance-records/42/picture", handler.Requests[1].PathAndQuery);
        Assert.Equal("image/jpeg", handler.Requests[1].ContentType);
        Assert.Equal(picture, handler.Requests[1].Body);
    }

    private sealed class RecordingHandler : HttpMessageHandler
    {
        private readonly Queue<string> _responses;

        public RecordingHandler(params string[] responses)
        {
            _responses = new Queue<string>(responses);
        }

        public List<RecordedRequest> Requests { get; } = [];

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            Requests.Add(new RecordedRequest(
                request.Method,
                request.RequestUri!.PathAndQuery,
                request.Headers.Authorization?.Scheme,
                request.Content?.Headers.ContentType?.MediaType,
                request.Content is null ? Array.Empty<byte>() : await request.Content.ReadAsByteArrayAsync(cancellationToken)));
            return new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(_responses.Dequeue(), Encoding.UTF8, "application/json"),
            };
        }
    }

    private sealed record RecordedRequest(
        HttpMethod Method,
        string PathAndQuery,
        string? AuthorizationScheme,
        string? ContentType,
        byte[] Body);
}
