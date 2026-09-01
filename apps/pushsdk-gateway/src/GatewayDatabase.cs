using Microsoft.Data.Sqlite;
using System.Globalization;
using System.Security.Cryptography;
using System.Text.Json;

namespace PushSdkGateway;

public sealed class GatewayDatabase
{
    private const string Pending = "pending";
    private const string Leased = "leased";
    private const string Delivered = "delivered";
    private const string Ignored = "ignored";

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        WriteIndented = false,
    };

    private readonly string _connectionString;

    public GatewayDatabase(GatewayOptions options)
    {
        Directory.CreateDirectory(options.DataDirectory);
        var databasePath = Path.Combine(options.DataDirectory, "pushsdk-gateway.sqlite");
        _connectionString = new SqliteConnectionStringBuilder
        {
            DataSource = databasePath,
            Mode = SqliteOpenMode.ReadWriteCreate,
            Pooling = true,
        }.ToString();
    }

    public async Task InitializeAsync(CancellationToken cancellationToken)
    {
        await using var connection = await OpenConnectionAsync(cancellationToken);
        await ExecuteAsync(connection, "PRAGMA journal_mode = WAL;", cancellationToken);
        await ExecuteAsync(connection, "PRAGMA foreign_keys = ON;", cancellationToken);
        await ExecuteAsync(connection, "PRAGMA busy_timeout = 5000;", cancellationToken);
        await ExecuteAsync(connection, """
            CREATE TABLE IF NOT EXISTS gateway_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                terminal_serial_number TEXT NOT NULL,
                vendor_event_id TEXT NOT NULL,
                data_format TEXT NOT NULL,
                raw_payload_sha256 TEXT NOT NULL,
                delivery_payload_json TEXT NULL,
                picture BLOB NULL,
                picture_sha256 TEXT NULL,
                state TEXT NOT NULL CHECK (state IN ('pending', 'leased', 'delivered', 'ignored')),
                received_at_utc TEXT NOT NULL,
                attempts INTEGER NOT NULL DEFAULT 0,
                next_attempt_at_utc TEXT NULL,
                lease_expires_at_utc TEXT NULL,
                last_error TEXT NULL,
                delivered_at_utc TEXT NULL,
                CONSTRAINT gateway_events_terminal_vendor_event_unique UNIQUE (terminal_serial_number, vendor_event_id)
            );
            """, cancellationToken);
        await ExecuteAsync(connection, "CREATE INDEX IF NOT EXISTS gateway_events_delivery_index ON gateway_events (state, next_attempt_at_utc, lease_expires_at_utc, id);", cancellationToken);
    }

    public async Task<IReadOnlyList<InboundPersistenceResult>> PersistEventsAsync(
        string terminalSerialNumber,
        IReadOnlyList<ParsedVendorEvent> events,
        DateTimeOffset receivedAtUtc,
        CancellationToken cancellationToken)
    {
        await using var connection = await OpenConnectionAsync(cancellationToken);
        await BeginImmediateAsync(connection, cancellationToken);

        try
        {
            var results = new List<InboundPersistenceResult>(events.Count);
            foreach (var parsedEvent in events)
            {
                var result = await InsertEventAsync(connection, terminalSerialNumber, parsedEvent, receivedAtUtc, cancellationToken);
                results.Add(result);
            }

            await CommitAsync(connection, cancellationToken);
            return results;
        }
        catch
        {
            await RollbackAsync(connection, cancellationToken);
            throw;
        }
    }

    public async Task<LeasedDelivery?> ClaimDeliveryAsync(DateTimeOffset nowUtc, TimeSpan leaseDuration, CancellationToken cancellationToken)
    {
        await using var connection = await OpenConnectionAsync(cancellationToken);
        await BeginImmediateAsync(connection, cancellationToken);

        try
        {
            long? id;
            await using (var select = connection.CreateCommand())
            {
                select.CommandText = """
                    SELECT id
                    FROM gateway_events
                    WHERE (state = $pending AND next_attempt_at_utc <= $now)
                       OR (state = $leased AND lease_expires_at_utc <= $now)
                    ORDER BY id
                    LIMIT 1;
                    """;
                select.Parameters.AddWithValue("$pending", Pending);
                select.Parameters.AddWithValue("$leased", Leased);
                select.Parameters.AddWithValue("$now", Timestamp(nowUtc));
                var result = await select.ExecuteScalarAsync(cancellationToken);
                id = result is long eventId ? eventId : null;
            }

            if (id is null)
            {
                await CommitAsync(connection, cancellationToken);
                return null;
            }

            var leaseExpiresAtUtc = nowUtc.Add(leaseDuration);
            await using (var update = connection.CreateCommand())
            {
                update.CommandText = """
                    UPDATE gateway_events
                    SET state = $leased,
                        attempts = attempts + 1,
                        lease_expires_at_utc = $leaseExpiresAtUtc,
                        last_error = NULL
                    WHERE id = $id;
                    """;
                update.Parameters.AddWithValue("$leased", Leased);
                update.Parameters.AddWithValue("$leaseExpiresAtUtc", Timestamp(leaseExpiresAtUtc));
                update.Parameters.AddWithValue("$id", id.Value);
                await update.ExecuteNonQueryAsync(cancellationToken);
            }

            LeasedDelivery? delivery;
            await using (var read = connection.CreateCommand())
            {
                read.CommandText = """
                    SELECT id, terminal_serial_number, vendor_event_id, delivery_payload_json, picture, attempts
                    FROM gateway_events
                    WHERE id = $id AND state = $leased;
                """;
                read.Parameters.AddWithValue("$id", id.Value);
                read.Parameters.AddWithValue("$leased", Leased);
                await using var reader = await read.ExecuteReaderAsync(cancellationToken);
                if (!await reader.ReadAsync(cancellationToken))
                {
                    throw new InvalidOperationException("A claimed gateway event could not be read.");
                }

                delivery = new LeasedDelivery(
                    reader.GetInt64(0),
                    reader.GetString(1),
                    reader.GetString(2),
                    reader.GetString(3),
                    reader.IsDBNull(4) ? null : reader.GetFieldValue<byte[]>(4),
                    reader.GetInt32(5));
            }

            await CommitAsync(connection, cancellationToken);
            return delivery;
        }
        catch
        {
            await RollbackAsync(connection, cancellationToken);
            throw;
        }
    }

    public async Task MarkDeliveredAsync(long id, DateTimeOffset deliveredAtUtc, CancellationToken cancellationToken)
    {
        await using var connection = await OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = """
            UPDATE gateway_events
            SET state = $delivered,
                delivery_payload_json = NULL,
                picture = NULL,
                lease_expires_at_utc = NULL,
                last_error = NULL,
                delivered_at_utc = $deliveredAtUtc
            WHERE id = $id AND state = $leased;
            """;
        command.Parameters.AddWithValue("$delivered", Delivered);
        command.Parameters.AddWithValue("$deliveredAtUtc", Timestamp(deliveredAtUtc));
        command.Parameters.AddWithValue("$id", id);
        command.Parameters.AddWithValue("$leased", Leased);
        if (await command.ExecuteNonQueryAsync(cancellationToken) != 1)
        {
            throw new InvalidOperationException("The delivery lease was lost before it could be marked delivered.");
        }
    }

    public async Task ReleaseDeliveryAsync(long id, int attempts, string error, DateTimeOffset nowUtc, CancellationToken cancellationToken)
    {
        var delaySeconds = Math.Min(300, 1 << Math.Min(attempts, 8));
        await using var connection = await OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = """
            UPDATE gateway_events
            SET state = $pending,
                next_attempt_at_utc = $nextAttemptAtUtc,
                lease_expires_at_utc = NULL,
                last_error = $error
            WHERE id = $id AND state = $leased;
            """;
        command.Parameters.AddWithValue("$pending", Pending);
        command.Parameters.AddWithValue("$nextAttemptAtUtc", Timestamp(nowUtc.AddSeconds(delaySeconds)));
        command.Parameters.AddWithValue("$error", error[..Math.Min(error.Length, 1000)]);
        command.Parameters.AddWithValue("$id", id);
        command.Parameters.AddWithValue("$leased", Leased);
        if (await command.ExecuteNonQueryAsync(cancellationToken) != 1)
        {
            throw new InvalidOperationException("The delivery lease was lost before it could be released.");
        }
    }

    public async Task PurgeDeliveredAsync(DateTimeOffset beforeUtc, CancellationToken cancellationToken)
    {
        await using var connection = await OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = "DELETE FROM gateway_events WHERE state IN ($delivered, $ignored) AND received_at_utc < $before;";
        command.Parameters.AddWithValue("$delivered", Delivered);
        command.Parameters.AddWithValue("$ignored", Ignored);
        command.Parameters.AddWithValue("$before", Timestamp(beforeUtc));
        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static async Task<InboundPersistenceResult> InsertEventAsync(
        SqliteConnection connection,
        string terminalSerialNumber,
        ParsedVendorEvent parsedEvent,
        DateTimeOffset receivedAtUtc,
        CancellationToken cancellationToken)
    {
        var delivery = parsedEvent.Delivery;
        await using var command = connection.CreateCommand();
        command.CommandText = """
            INSERT INTO gateway_events (
                terminal_serial_number,
                vendor_event_id,
                data_format,
                raw_payload_sha256,
                delivery_payload_json,
                picture,
                picture_sha256,
                state,
                received_at_utc,
                next_attempt_at_utc
            ) VALUES (
                $terminalSerialNumber,
                $vendorEventId,
                $dataFormat,
                $rawPayloadSha256,
                $deliveryPayloadJson,
                $picture,
                $pictureSha256,
                $state,
                $receivedAtUtc,
                $nextAttemptAtUtc
            );
            """;
        command.Parameters.AddWithValue("$terminalSerialNumber", terminalSerialNumber);
        command.Parameters.AddWithValue("$vendorEventId", parsedEvent.VendorEventId);
        command.Parameters.AddWithValue("$dataFormat", parsedEvent.DataFormat);
        command.Parameters.AddWithValue("$rawPayloadSha256", parsedEvent.RawPayloadSha256);
        command.Parameters.AddWithValue("$deliveryPayloadJson", delivery is null ? DBNull.Value : SerializeDeliveryPayload(parsedEvent.VendorEventId, delivery.Event));
        command.Parameters.AddWithValue("$picture", delivery?.Picture is null ? DBNull.Value : delivery.Picture);
        command.Parameters.AddWithValue("$pictureSha256", delivery?.Picture is null ? DBNull.Value : Convert.ToHexString(SHA256.HashData(delivery.Picture)).ToLowerInvariant());
        command.Parameters.AddWithValue("$state", delivery is null ? Ignored : Pending);
        command.Parameters.AddWithValue("$receivedAtUtc", Timestamp(receivedAtUtc));
        command.Parameters.AddWithValue("$nextAttemptAtUtc", delivery is null ? DBNull.Value : Timestamp(receivedAtUtc));

        try
        {
            await command.ExecuteNonQueryAsync(cancellationToken);
            return new InboundPersistenceResult(parsedEvent.VendorEventId, true);
        }
        catch (SqliteException exception) when (exception.SqliteErrorCode == 19)
        {
            return new InboundPersistenceResult(parsedEvent.VendorEventId, false);
        }
    }

    private static string SerializeDeliveryPayload(string sourceEventId, CanonicalAttendanceEvent attendanceEvent)
    {
        return JsonSerializer.Serialize(new
        {
            schema = "attendance.push-sdk.gateway.v1",
            terminal_serial_number = attendanceEvent.TerminalSerialNumber,
            source_event_id = sourceEventId,
            occurred_at = attendanceEvent.OccurredAtUtc.ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", CultureInfo.InvariantCulture),
            @event = new
            {
                employee_number = attendanceEvent.EmployeeNumber,
                employee_name = attendanceEvent.EmployeeName,
                verification_method = attendanceEvent.VerificationMethod,
                attendance_status = attendanceEvent.AttendanceStatus,
                status_value = attendanceEvent.StatusValue,
                picture_expected = attendanceEvent.PictureExpected,
            },
        }, JsonOptions);
    }

    private async Task<SqliteConnection> OpenConnectionAsync(CancellationToken cancellationToken)
    {
        var connection = new SqliteConnection(_connectionString);
        await connection.OpenAsync(cancellationToken);
        return connection;
    }

    private static async Task ExecuteAsync(SqliteConnection connection, string sql, CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.CommandText = sql;
        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static async Task BeginImmediateAsync(SqliteConnection connection, CancellationToken cancellationToken)
    {
        await ExecuteAsync(connection, "BEGIN IMMEDIATE;", cancellationToken);
    }

    private static async Task CommitAsync(SqliteConnection connection, CancellationToken cancellationToken)
    {
        await ExecuteAsync(connection, "COMMIT;", cancellationToken);
    }

    private static async Task RollbackAsync(SqliteConnection connection, CancellationToken cancellationToken)
    {
        try
        {
            await ExecuteAsync(connection, "ROLLBACK;", cancellationToken);
        }
        catch (SqliteException)
        {
            // The transaction may have already been committed or rolled back by SQLite after a failed statement.
        }
    }

    private static string Timestamp(DateTimeOffset value)
    {
        return value.ToUniversalTime().ToString("O", CultureInfo.InvariantCulture);
    }
}

public sealed record InboundPersistenceResult(string VendorEventId, bool Created);

public sealed record LeasedDelivery(
    long Id,
    string TerminalSerialNumber,
    string VendorEventId,
    string PayloadJson,
    byte[]? Picture,
    int Attempts);
