<?php

namespace App\Http\Controllers;

use App\Models\AttendanceRecord;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Storage;
use Illuminate\View\View;
use Symfony\Component\HttpFoundation\StreamedResponse;

class AttendanceRecordController extends Controller
{
    private const MAX_PICTURE_BYTES = 2097152;

    private const PICTURE_CONTENT_TYPE = 'image/jpeg';

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

    public function wipe(): RedirectResponse
    {
        $recordCount = AttendanceRecord::count();

        AttendanceRecord::truncate();
        Storage::disk('local')->deleteDirectory('attendance-record-pictures');
        Storage::disk('local')->makeDirectory('attendance-record-pictures');

        return redirect()
            ->route('attendance-records.index')
            ->with('status', $recordCount === 1 ? 'Wiped 1 record.' : "Wiped {$recordCount} records.");
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
            'event.picture_available' => ['required', 'boolean'],
            'event.pictureURL' => ['prohibited'],
            'event.picture' => ['prohibited'],
            'event.raw' => ['required', 'array'],
            'event.raw.pictureURL' => ['prohibited'],
            'event.raw.picture' => ['prohibited'],
        ]);

        $device = $validated['device'];
        $event = $validated['event'];
        $rawEvent = $request->input('event.raw', []);
        $pictureExpected = (bool) $event['picture_available'];
        $payload = $validated;
        $payload['event']['raw'] = $rawEvent;
        $deviceKey = filled($device['serial_number'] ?? null)
            ? $device['serial_number']
            : $device['base_url'];

        $record = AttendanceRecord::firstOrNew([
            'bridge_id' => $validated['bridge']['id'],
            'device_key' => $deviceKey,
            'event_serial_no' => $event['serialNo'],
        ]);

        abort_if($record->exists && (bool) $record->picture_expected !== $pictureExpected, 409, 'Picture expectation changed for existing event.');

        $record->fill([
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
            'picture_expected' => $pictureExpected,
            'raw_event' => $rawEvent,
            'payload' => $payload,
        ]);
        $record->save();

        $pictureUploadRequired = $record->picture_expected && ! filled($record->picture_path);

        return response()->json([
            'ok' => true,
            'created' => $record->wasRecentlyCreated,
            'id' => $record->id,
            'event_serial_no' => $record->event_serial_no,
            'picture_upload_required' => $pictureUploadRequired,
            'picture_upload_url' => $pictureUploadRequired
                ? rtrim($request->url(), '/')."/{$record->id}/picture"
                : null,
            'picture_stored' => filled($record->picture_path),
        ], $record->wasRecentlyCreated ? 201 : 200);
    }

    public function storePicture(Request $request, AttendanceRecord $attendanceRecord): JsonResponse
    {
        $this->authorizeBridge($request);

        abort_unless($attendanceRecord->picture_expected, 409, 'Picture upload is not expected for this event.');
        abort_unless($request->headers->get('Content-Type') === self::PICTURE_CONTENT_TYPE, 415, 'Picture content type must be image/jpeg.');

        $input = $request->getContent(true);
        abort_unless(is_resource($input), 500, 'Unable to read picture body.');

        $temp = tmpfile();
        if (! is_resource($temp)) {
            fclose($input);
            abort(500, 'Unable to create temporary picture file.');
        }

        $bytes = 0;
        $head = '';
        $hash = hash_init('sha256');

        while (! feof($input)) {
            $chunk = fread($input, 8192);
            if ($chunk === false) {
                fclose($input);
                fclose($temp);
                abort(400, 'Unable to read picture body.');
            }
            if ($chunk === '') {
                continue;
            }

            $bytes += strlen($chunk);
            if ($bytes > self::MAX_PICTURE_BYTES) {
                fclose($input);
                fclose($temp);
                abort(413, 'Picture data is too large.');
            }

            if (strlen($head) < 3) {
                $head .= substr($chunk, 0, 3 - strlen($head));
            }
            hash_update($hash, $chunk);
            fwrite($temp, $chunk);
        }
        fclose($input);

        abort_if($bytes === 0, 422, 'Picture data is required.');
        abort_unless(str_starts_with($head, "\xFF\xD8\xFF"), 422, 'Picture data is not a JPEG image.');

        $pictureHash = hash_final($hash);
        abort_if(filled($attendanceRecord->picture_sha256) && $attendanceRecord->picture_sha256 !== $pictureHash, 409, 'Picture content changed for existing event.');

        rewind($temp);
        $path = "attendance-record-pictures/{$attendanceRecord->id}.jpg";
        if (! Storage::disk('local')->writeStream($path, $temp)) {
            fclose($temp);
            abort(500, 'Unable to store picture.');
        }
        fclose($temp);

        $attendanceRecord->forceFill([
            'picture_path' => $path,
            'picture_content_type' => self::PICTURE_CONTENT_TYPE,
            'picture_bytes' => $bytes,
            'picture_sha256' => $pictureHash,
        ])->save();

        return response()->json([
            'ok' => true,
            'id' => $attendanceRecord->id,
            'picture_stored' => true,
            'picture_bytes' => $bytes,
            'picture_sha256' => $pictureHash,
        ]);
    }

    public function picture(AttendanceRecord $attendanceRecord): StreamedResponse
    {
        abort_unless(filled($attendanceRecord->picture_path), 404);
        abort_unless(Storage::disk('local')->exists($attendanceRecord->picture_path), 404);

        return response()->stream(function () use ($attendanceRecord): void {
            $stream = Storage::disk('local')->readStream($attendanceRecord->picture_path);
            if (! is_resource($stream)) {
                return;
            }

            while (! feof($stream)) {
                $chunk = fread($stream, 8192);
                if ($chunk === false) {
                    break;
                }
                echo $chunk;
            }

            fclose($stream);
        }, 200, [
            'Content-Type' => $attendanceRecord->picture_content_type ?: self::PICTURE_CONTENT_TYPE,
            'Content-Length' => (string) ($attendanceRecord->picture_bytes ?? Storage::disk('local')->size($attendanceRecord->picture_path)),
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
}
