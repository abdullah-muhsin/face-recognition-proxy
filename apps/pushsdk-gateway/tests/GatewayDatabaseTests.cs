using System.Text.Json;

namespace PushSdkGateway.Tests;

public sealed class GatewayDatabaseTests
{
    [Fact]
    public async Task StoresEachVendorEventOnceAndRemovesDeliveryDataAfterSuccess()
    {
        using var environment = new TestEnvironment();
        var options = environment.CreateOptions();
        options.Validate();
        var parser = new AttendanceEventParser(options);
        var database = new GatewayDatabase(options);
        await database.InitializeAsync(CancellationToken.None);
        var events = parser.ParseBatch(
            TestEnvironment.TerminalSerialNumber,
            TestProtocol.BuildEventEnvelope("event-deduplicated-1", "jsonData", TestProtocol.AccessEventJson()));

        var initial = await database.PersistEventsAsync(TestEnvironment.TerminalSerialNumber, events, DateTimeOffset.UtcNow, CancellationToken.None);
        var retry = await database.PersistEventsAsync(TestEnvironment.TerminalSerialNumber, events, DateTimeOffset.UtcNow, CancellationToken.None);

        Assert.True(Assert.Single(initial).Created);
        Assert.False(Assert.Single(retry).Created);
        var delivery = await database.ClaimDeliveryAsync(DateTimeOffset.UtcNow, TimeSpan.FromMinutes(2), CancellationToken.None);
        var claimed = Assert.IsType<LeasedDelivery>(delivery);
        using (var payload = JsonDocument.Parse(claimed.PayloadJson))
        {
            Assert.Equal("event-deduplicated-1", payload.RootElement.GetProperty("source_event_id").GetString());
        }

        await database.MarkDeliveredAsync(claimed.Id, DateTimeOffset.UtcNow, CancellationToken.None);

        Assert.Null(await database.ClaimDeliveryAsync(DateTimeOffset.UtcNow.AddMinutes(3), TimeSpan.FromMinutes(2), CancellationToken.None));
    }
}
