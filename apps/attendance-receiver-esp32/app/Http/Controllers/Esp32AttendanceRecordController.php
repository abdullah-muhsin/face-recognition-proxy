<?php

namespace App\Http\Controllers;

use App\Models\AttendanceRecord;
use App\Services\AttendanceRecordPictureStorage;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class Esp32AttendanceRecordController extends Controller
{
    public function store(Request $request): JsonResponse
    {
        $this->authorizeBridge($request);

        $validated = $request->validate([
            'schema' => ['required', 'string', 'max:80'],
            'firmware' => ['nullable', 'string', 'max:40'],
            'bridge' => ['required', 'array'],
            'bridge.id' => ['required', 'string', 'max:64'],
            'device' => ['required', 'array'],
            'device.base_url' => ['required', 'string', 'max:255'],
            'device.username' => ['nullable', 'string', 'max:80'],
            'device.name' => ['nullable', 'string', 'max:120'],
            'device.model' => ['nullable', 'string', 'max:80'],
            'device.serial_number' => ['required', 'string', 'max:160'],
            'device.mac_address' => ['nullable', 'string', 'max:40'],
            'event' => ['required', 'array'],
            'event.serialNo' => ['required', 'integer', 'min:1'],
            'event.major' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.minor' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.time' => ['required', 'date_format:Y-m-d\\TH:i:sP'],
            'event.employeeNoString' => ['nullable', 'string', 'max:80'],
            'event.name' => ['nullable', 'string', 'max:160'],
            'event.currentVerifyMode' => ['nullable', 'string', 'max:80'],
            'event.attendanceStatus' => ['nullable', 'string', 'max:80'],
            'event.statusValue' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.picture_available' => ['required', 'boolean'],
            'event.pictureURL' => ['prohibited'],
            'event.picture' => ['prohibited'],
            'event.raw' => ['required', 'array'],
            'event.raw.pictureURL' => ['prohibited'],
            'event.raw.picture' => ['prohibited'],
        ]);

        $device = $validated['device'];
        $event = $validated['event'];
        $rawEvent = $request->input('event.raw');
        abort_unless(is_array($rawEvent), 422, 'The event.raw field must be an object.');
        $pictureExpected = (bool) $event['picture_available'];
        $deviceKey = $device['serial_number'];
        $payload = $validated;
        $payload['event']['raw'] = $rawEvent;

        $record = AttendanceRecord::firstOrNew([
            'bridge_identifier' => $validated['bridge']['id'],
            'terminal_serial_number' => $deviceKey,
            'legacy_event_serial_number' => $event['serialNo'],
        ]);

        abort_if($record->exists && (bool) $record->picture_expected !== $pictureExpected, 409, 'Picture expectation changed for existing event.');

        $record->fill([
            'source_schema' => $validated['schema'],
            'bridge_firmware' => $validated['firmware'] ?? null,
            'ingestion_source' => 'esp32',
            'received_at' => $record->received_at ?? now(),
            'legacy_device_base_url' => $device['base_url'],
            'legacy_device_username' => $device['username'] ?? null,
            'terminal_name' => $device['name'] ?? null,
            'terminal_model' => $device['model'] ?? null,
            'terminal_serial_number' => $device['serial_number'],
            'terminal_mac_address' => $device['mac_address'] ?? null,
            'occurred_at' => CarbonImmutable::createFromFormat('!Y-m-d\\TH:i:sP', $event['time']),
            'legacy_event_major' => $event['major'] ?? null,
            'legacy_event_minor' => $event['minor'] ?? null,
            'employee_number' => $event['employeeNoString'] ?? null,
            'employee_name' => $event['name'] ?? null,
            'verification_method' => $event['currentVerifyMode'] ?? null,
            'attendance_status' => $event['attendanceStatus'] ?? null,
            'attendance_status_value' => $event['statusValue'] ?? null,
            'picture_expected' => $pictureExpected,
            'legacy_raw_event' => $rawEvent,
            'source_payload' => $payload,
        ]);
        $record->save();

        $pictureUploadRequired = $record->picture_expected && ! filled($record->picture_path);

        return response()->json([
            'ok' => true,
            'created' => $record->wasRecentlyCreated,
            'id' => $record->id,
            'event_serial_no' => $record->legacy_event_serial_number,
            'picture_upload_required' => $pictureUploadRequired,
            'picture_upload_url' => $pictureUploadRequired
                ? rtrim($request->url(), '/')."/{$record->id}/picture"
                : null,
            'picture_stored' => filled($record->picture_path),
        ], $record->wasRecentlyCreated ? 201 : 200);
    }

    public function storePicture(Request $request, AttendanceRecord $attendanceRecord, AttendanceRecordPictureStorage $pictureStorage): JsonResponse
    {
        $this->authorizeBridge($request);
        abort_unless($attendanceRecord->ingestion_source === 'esp32', 404);

        $picture = $pictureStorage->store($attendanceRecord, $request);

        return response()->json([
            'ok' => true,
            'id' => $attendanceRecord->id,
            'picture_stored' => true,
            'picture_bytes' => $picture['bytes'],
            'picture_sha256' => $picture['sha256'],
        ]);
    }

    private function authorizeBridge(Request $request): void
    {
        $expectedToken = (string) config('services.attendance_bridge.token', '');
        if ($expectedToken === '') {
            return;
        }

        $providedToken = $request->bearerToken() ?: $request->header('X-Bridge-Token', '');
        abort_unless(hash_equals($expectedToken, (string) $providedToken), 401);
    }
}
