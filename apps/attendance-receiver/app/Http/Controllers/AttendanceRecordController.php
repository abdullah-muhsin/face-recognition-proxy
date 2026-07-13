<?php

namespace App\Http\Controllers;

use App\Models\AttendanceRecord;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class AttendanceRecordController extends Controller
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
            'device.serial_number' => ['nullable', 'string', 'max:160'],
            'device.mac_address' => ['nullable', 'string', 'max:40'],
            'event' => ['required', 'array'],
            'event.serialNo' => ['required', 'integer', 'min:1'],
            'event.major' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.minor' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.time' => ['nullable', 'string', 'max:40'],
            'event.employeeNoString' => ['nullable', 'string', 'max:80'],
            'event.name' => ['nullable', 'string', 'max:160'],
            'event.currentVerifyMode' => ['nullable', 'string', 'max:80'],
            'event.attendanceStatus' => ['nullable', 'string', 'max:80'],
            'event.statusValue' => ['nullable', 'integer', 'min:0', 'max:65535'],
            'event.pictureURL' => ['nullable', 'string', 'max:2048'],
            'event.raw' => ['nullable', 'array'],
        ]);

        $device = $validated['device'];
        $event = $validated['event'];
        $deviceKey = filled($device['serial_number'] ?? null)
            ? $device['serial_number']
            : $device['base_url'];

        $record = AttendanceRecord::updateOrCreate(
            [
                'bridge_id' => $validated['bridge']['id'],
                'device_key' => $deviceKey,
                'event_serial_no' => $event['serialNo'],
            ],
            [
                'schema' => $validated['schema'],
                'firmware' => $validated['firmware'] ?? null,
                'device_base_url' => $device['base_url'],
                'device_username' => $device['username'] ?? null,
                'device_name' => $device['name'] ?? null,
                'device_model' => $device['model'] ?? null,
                'device_serial_number' => $device['serial_number'] ?? null,
                'device_mac_address' => $device['mac_address'] ?? null,
                'event_time' => $this->parseEventTime($event['time'] ?? null),
                'major' => $event['major'] ?? null,
                'minor' => $event['minor'] ?? null,
                'employee_no' => $event['employeeNoString'] ?? null,
                'employee_name' => $event['name'] ?? null,
                'current_verify_mode' => $event['currentVerifyMode'] ?? null,
                'attendance_status' => $event['attendanceStatus'] ?? null,
                'status_value' => $event['statusValue'] ?? null,
                'picture_url' => $event['pictureURL'] ?? null,
                'raw_event' => $event['raw'] ?? $event,
                'payload' => $request->all(),
            ],
        );

        return response()->json([
            'ok' => true,
            'created' => $record->wasRecentlyCreated,
            'id' => $record->id,
            'event_serial_no' => $record->event_serial_no,
        ], $record->wasRecentlyCreated ? 201 : 200);
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

    private function parseEventTime(?string $value): ?CarbonImmutable
    {
        if (! filled($value)) {
            return null;
        }

        return CarbonImmutable::parse($value);
    }
}
