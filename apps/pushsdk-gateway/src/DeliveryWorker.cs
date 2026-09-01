namespace PushSdkGateway;

public sealed class DeliveryWorker : BackgroundService
{
    private readonly GatewayDatabase _database;
    private readonly LaravelReceiverClient _receiver;
    private readonly GatewayOptions _options;
    private readonly ILogger<DeliveryWorker> _logger;

    public DeliveryWorker(
        GatewayDatabase database,
        LaravelReceiverClient receiver,
        GatewayOptions options,
        ILogger<DeliveryWorker> logger)
    {
        _database = database;
        _receiver = receiver;
        _options = options;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        var nextPurgeAtUtc = DateTimeOffset.UtcNow;
        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                var nowUtc = DateTimeOffset.UtcNow;
                if (nowUtc >= nextPurgeAtUtc)
                {
                    await _database.PurgeDeliveredAsync(nowUtc.AddDays(-_options.DeliveredEventRetentionDays), stoppingToken);
                    nextPurgeAtUtc = nowUtc.AddHours(1);
                }

                var delivery = await _database.ClaimDeliveryAsync(
                    nowUtc,
                    TimeSpan.FromSeconds(_options.DeliveryLeaseSeconds),
                    stoppingToken);
                if (delivery is null)
                {
                    await Task.Delay(TimeSpan.FromSeconds(1), stoppingToken);
                    continue;
                }

                try
                {
                    await _receiver.DeliverAsync(delivery, stoppingToken);
                    await _database.MarkDeliveredAsync(delivery.Id, DateTimeOffset.UtcNow, stoppingToken);
                    _logger.LogInformation("Delivered attendance event {VendorEventId} from terminal {TerminalSerialNumber}.", delivery.VendorEventId, delivery.TerminalSerialNumber);
                }
                catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
                {
                    throw;
                }
                catch (Exception exception)
                {
                    await _database.ReleaseDeliveryAsync(delivery.Id, delivery.Attempts, exception.Message, DateTimeOffset.UtcNow, stoppingToken);
                    _logger.LogWarning(exception, "Delivery of attendance event {VendorEventId} from terminal {TerminalSerialNumber} failed; it will be retried.", delivery.VendorEventId, delivery.TerminalSerialNumber);
                }
            }
            catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
            {
                break;
            }
            catch (Exception exception)
            {
                _logger.LogError(exception, "The Push SDK delivery worker encountered an unexpected error.");
                await Task.Delay(TimeSpan.FromSeconds(5), stoppingToken);
            }
        }
    }
}
