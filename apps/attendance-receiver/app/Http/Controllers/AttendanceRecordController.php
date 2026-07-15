<?php

namespace App\Http\Controllers;

use App\Models\AttendanceRecord;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Facades\Storage;
use Illuminate\View\View;

class AttendanceRecordController extends Controller
{
    private const MAX_PICTURE_BYTES = 73728;
    private const MAX_PICTURE_BASE64_CHARACTERS = 98304;

    public function index(Request $request): View
    {
        $search = trim((string) $request->query('search', ''));

        $records = AttendanceRecord::query()
            ->when($search !== '', function ($query) use ($search): void {
                $query->where(function ($query) use ($search): void {
                    $query->where('employee_no', 'like', "%{$search}%")
                        ->orWhere('employee_name', 'like', "%{$search}%")
                        ->orWhere('device_serial_number', 'like', "%{$search}%")
                        ->orWhere('bridge_id', 'like', "%{$search}%")
                        ->orWhere('event_serial_no', $search);
                });
            })
            ->latest('event_time')
            ->latest('id')
            ->paginate(50)
            ->withQueryString();

        return view('attendance-records.index', [
            'records' => $records,
            'search' => $search,
            'totalRecords' => AttendanceRecord::count(),
            'latestRecord' => AttendanceRecord::query()->latest('event_time')->latest('id')->first(),
            'uniqueEmployees' => AttendanceRecord::query()->whereNotNull('employee_no')->distinct('employee_no')->count('employee_no'),
            'uniqueDevices' => AttendanceRecord::query()->distinct('device_key')->count('device_key'),
        ]);
    }

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
            'event.picture' => ['nullable', 'array'],
            'event.picture.contentType' => ['nullable', 'string', 'max:120'],
            'event.picture.encoding' => ['nullable', 'string', 'in:base64'],
            'event.picture.bytes' => ['nullable', 'integer', 'min:1', 'max:'.self::MAX_PICTURE_BYTES],
            'event.picture.data' => ['nullable', 'string', 'max:'.self::MAX_PICTURE_BASE64_CHARACTERS],
            'event.raw' => ['required', 'array'],
        ]);

        $device = $validated['device'];
        $event = $validated['event'];
        $picture = $this->decodePicture($event);
        $payload = $this->payloadForStorage($request->all());
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
                'raw_event' => $event['raw'],
                'payload' => $payload,
            ],
        );

        $this->storePicture($record, $picture);

        return response()->json([
            'ok' => true,
            'created' => $record->wasRecentlyCreated,
            'id' => $record->id,
            'event_serial_no' => $record->event_serial_no,
            'picture_stored' => filled($record->picture_path),
        ], $record->wasRecentlyCreated ? 201 : 200);
    }

    public function picture(AttendanceRecord $attendanceRecord): Response
    {
        abort_unless(filled($attendanceRecord->picture_path), 404);
        abort_unless(Storage::disk('local')->exists($attendanceRecord->picture_path), 404);

        return response(Storage::disk('local')->get($attendanceRecord->picture_path), 200, [
            'Content-Type' => $attendanceRecord->picture_content_type ?: 'image/jpeg',
            'Cache-Control' => 'private, max-age=3600',
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

    private function parseEventTime(?string $value): ?CarbonImmutable
    {
        if (! filled($value)) {
            return null;
        }

        return CarbonImmutable::parse($value);
    }

    private function decodePicture(array $event): ?array
    {
        $picture = $event['picture'] ?? null;
        if (! is_array($picture)) {
            abort_if(filled($event['pictureURL'] ?? null), 422, 'Picture data is required when pictureURL is present.');

            return null;
        }

        foreach (['contentType', 'encoding', 'bytes', 'data'] as $field) {
            abort_unless(array_key_exists($field, $picture) && filled($picture[$field]), 422, "Picture {$field} is required.");
        }

        abort_unless($picture['encoding'] === 'base64', 422, 'Unsupported picture encoding.');

        $binary = base64_decode((string) $picture['data'], true);
        abort_if($binary === false, 422, 'Invalid picture data.');
        abort_if(strlen($binary) > self::MAX_PICTURE_BYTES, 422, 'Picture data is too large.');
        abort_unless((int) $picture['bytes'] === strlen($binary), 422, 'Picture byte count does not match data.');

        $contentType = $this->pictureContentType((string) $picture['contentType'], $binary);

        return [
            'binary' => $binary,
            'content_type' => $contentType,
        ];
    }

    private function storePicture(AttendanceRecord $record, ?array $picture): void
    {
        if ($picture === null) {
            return;
        }

        $binary = $picture['binary'];
        $contentType = $picture['content_type'];
        $extension = $contentType === 'image/png' ? 'png' : 'jpg';
        $path = "attendance-record-pictures/{$record->id}.{$extension}";

        if (filled($record->picture_path) && $record->picture_path !== $path) {
            Storage::disk('local')->delete($record->picture_path);
        }

        Storage::disk('local')->put($path, $binary);
        $record->forceFill([
            'picture_path' => $path,
            'picture_content_type' => $contentType,
            'picture_bytes' => strlen($binary),
        ])->save();
    }

    private function pictureContentType(string $contentType, string $binary): string
    {
        $contentType = strtolower(trim(explode(';', $contentType)[0] ?? ''));

        abort_unless(in_array($contentType, ['image/jpeg', 'image/png'], true), 422, 'Unsupported picture content type.');
        abort_unless($this->binaryMatchesContentType($contentType, $binary), 422, 'Picture content type does not match data.');

        return $contentType;
    }

    private function binaryMatchesContentType(string $contentType, string $binary): bool
    {
        return match ($contentType) {
            'image/jpeg' => str_starts_with($binary, "\xFF\xD8\xFF"),
            'image/png' => str_starts_with($binary, "\x89PNG\r\n\x1A\n"),
            default => false,
        };
    }

    private function payloadForStorage(array $payload): array
    {
        if (isset($payload['event']['picture']['data'])) {
            unset($payload['event']['picture']['data']);
            $payload['event']['picture']['stored_separately'] = true;
        }

        return $payload;
    }
}
