<?php

namespace App\Http\Controllers;

use App\Models\AttendanceRecord;
use App\Models\AttendanceTerminal;
use App\Services\AttendanceRecordPictureStorage;
use Carbon\CarbonImmutable;
use Illuminate\Database\QueryException;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Validation\ValidationException;
use Throwable;

class PushSdkGatewayAttendanceRecordController extends Controller
{
    private const SOURCE_SCHEMA = 'attendance.push-sdk.gateway.v1';

    private const INGESTION_SOURCE = 'push_sdk';

    public function store(Request $request): JsonResponse
    {
        $this->authorizeGateway($request);

        $validated = $request->validate([
            'schema' => ['required', 'in:'.self::SOURCE_SCHEMA],
            'terminal_serial_number' => ['required', 'string', 'max:160', 'regex:/\A[A-Za-z0-9._-]+\z/D'],
            'source_event_id' => ['required', 'string', 'max:160', 'regex:/\A[A-Za-z0-9._:-]+\z/D'],
            'occurred_at' => ['required', 'string', 'max:20'],
            'event' => ['required', 'array:employee_number,employee_name,verification_method,attendance_status,status_value,picture_expected'],
            'event.employee_number' => ['required', 'string', 'max:80'],
            'event.employee_name' => ['nullable', 'string', 'max:160'],
            'event.verification_method' => ['required', 'string', 'max:80'],
            'event.attendance_status' => ['required', 'string', 'max:80'],
            'event.status_value' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.picture_expected' => ['required', 'boolean'],
        ]);

        $occurredAt = $this->parseUtcTimestamp($validated['occurred_at']);
        $terminal = AttendanceTerminal::query()
            ->where('serial_number', $validated['terminal_serial_number'])
            ->where('is_active', true)
            ->first();

        abort_unless($terminal instanceof AttendanceTerminal, 403, 'Terminal is not registered or active.');

        $event = [
            'employee_number' => $validated['event']['employee_number'],
            'employee_name' => $validated['event']['employee_name'] ?? null,
            'verification_method' => $validated['event']['verification_method'],
            'attendance_status' => $validated['event']['attendance_status'],
            'status_value' => $validated['event']['status_value'] ?? null,
            'picture_expected' => (bool) $validated['event']['picture_expected'],
        ];
        $payload = [
            'schema' => self::SOURCE_SCHEMA,
            'terminal_serial_number' => $terminal->serial_number,
            'source_event_id' => $validated['source_event_id'],
            'occurred_at' => $occurredAt->format('Y-m-d\\TH:i:s\\Z'),
            'event' => $event,
        ];
        $sourcePayloadHash = hash('sha256', json_encode($payload, JSON_THROW_ON_ERROR));

        [$record, $created] = $this->createOrFindByVendorEventIdentity(
            terminal: $terminal,
            vendorEventId: $validated['source_event_id'],
            occurredAt: $occurredAt,
            event: $event,
            payload: $payload,
            sourcePayloadHash: $sourcePayloadHash,
        );

        abort_if(
            ! hash_equals((string) $record->source_payload_hash, $sourcePayloadHash),
            409,
            'A different event was submitted with the same terminal and source event ID.',
        );

        $pictureUploadRequired = $record->picture_expected && ! filled($record->picture_path);

        return response()->json([
            'ok' => true,
            'created' => $created,
            'id' => $record->id,
            'source_event_id' => $record->vendor_event_id,
            'picture_upload_required' => $pictureUploadRequired,
            'picture_upload_path' => $pictureUploadRequired
                ? "/api/internal/push-sdk/attendance-records/{$record->id}/picture"
                : null,
            'picture_stored' => filled($record->picture_path),
        ], $created ? 201 : 200);
    }

    public function storePicture(
        Request $request,
        AttendanceRecord $attendanceRecord,
        AttendanceRecordPictureStorage $pictureStorage,
    ): JsonResponse {
        $this->authorizeGateway($request);
        abort_unless($attendanceRecord->ingestion_source === self::INGESTION_SOURCE, 404);

        $picture = $pictureStorage->store($attendanceRecord, $request);

        return response()->json([
            'ok' => true,
            'id' => $attendanceRecord->id,
            'picture_stored' => true,
            'picture_bytes' => $picture['bytes'],
            'picture_sha256' => $picture['sha256'],
        ]);
    }

    /**
     * @param  array{employee_number: string, employee_name: ?string, verification_method: string, attendance_status: string, status_value: ?int, picture_expected: bool}  $event
     * @param  array{schema: string, terminal_serial_number: string, source_event_id: string, occurred_at: string, event: array<string, mixed>}  $payload
     * @return array{AttendanceRecord, bool}
     */
    private function createOrFindByVendorEventIdentity(
        AttendanceTerminal $terminal,
        string $vendorEventId,
        CarbonImmutable $occurredAt,
        array $event,
        array $payload,
        string $sourcePayloadHash,
    ): array {
        $attributes = [
            'source_schema' => self::SOURCE_SCHEMA,
            'ingestion_source' => self::INGESTION_SOURCE,
            'terminal_id' => $terminal->id,
            'bridge_identifier' => null,
            'legacy_device_base_url' => null,
            'legacy_device_username' => null,
            'terminal_name' => $terminal->display_name,
            'terminal_model' => $terminal->model,
            'terminal_serial_number' => $terminal->serial_number,
            'terminal_mac_address' => null,
            'legacy_event_serial_number' => null,
            'vendor_event_id' => $vendorEventId,
            'occurred_at' => $occurredAt,
            'received_at' => now(),
            'legacy_event_major' => null,
            'legacy_event_minor' => null,
            'employee_number' => $event['employee_number'],
            'employee_name' => $event['employee_name'],
            'verification_method' => $event['verification_method'],
            'attendance_status' => $event['attendance_status'],
            'attendance_status_value' => $event['status_value'],
            'source_payload_hash' => $sourcePayloadHash,
            'picture_expected' => $event['picture_expected'],
            'legacy_raw_event' => null,
            'source_payload' => $payload,
        ];

        try {
            return [AttendanceRecord::query()->create($attributes), true];
        } catch (QueryException $exception) {
            if (! in_array((string) $exception->getCode(), ['23000', '23505'], true)) {
                throw $exception;
            }

            $record = AttendanceRecord::query()
                ->where('terminal_id', $terminal->id)
                ->where('vendor_event_id', $vendorEventId)
                ->first();

            if (! $record instanceof AttendanceRecord) {
                throw $exception;
            }

            return [$record, false];
        }
    }

    private function authorizeGateway(Request $request): void
    {
        $expectedToken = (string) config('attendance.push_sdk_gateway.token', '');
        abort_unless(strlen($expectedToken) >= 32, 503, 'Push SDK gateway authentication must use a secret of at least 32 characters.');
        abort_unless(hash_equals($expectedToken, (string) $request->bearerToken()), 401);
    }

    private function parseUtcTimestamp(string $value): CarbonImmutable
    {
        if (! preg_match('/\A\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\z/D', $value)) {
            throw ValidationException::withMessages([
                'occurred_at' => 'The occurred_at field must be an RFC 3339 UTC timestamp with second precision.',
            ]);
        }

        try {
            $timestamp = CarbonImmutable::createFromFormat('!Y-m-d\\TH:i:s\\Z', $value, 'UTC');
        } catch (Throwable) {
            $timestamp = false;
        }

        if (! $timestamp instanceof CarbonImmutable || $timestamp->format('Y-m-d\\TH:i:s\\Z') !== $value) {
            throw ValidationException::withMessages([
                'occurred_at' => 'The occurred_at field must be a valid UTC timestamp.',
            ]);
        }

        return $timestamp;
    }
}
