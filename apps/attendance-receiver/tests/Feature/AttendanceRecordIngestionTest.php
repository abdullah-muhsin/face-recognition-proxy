<?php

namespace Tests\Feature;

use App\Models\AttendanceRecord;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Storage;
use Tests\TestCase;

class AttendanceRecordIngestionTest extends TestCase
{
    use RefreshDatabase;

    public function test_it_stores_hikvision_attendance_record_payloads_idempotently(): void
    {
        Storage::fake('local');
        $payload = $this->payload();

        $first = $this->postJson('/api/attendance-records', $payload);
        $first->assertCreated()
            ->assertJsonPath('ok', true)
            ->assertJsonPath('created', true)
            ->assertJsonPath('event_serial_no', 75)
            ->assertJsonPath('picture_upload_required', true)
            ->assertJsonPath('picture_stored', false);

        $uploadUrl = $first->json('picture_upload_url');
        $this->assertIsString($uploadUrl);

        $second = $this->postJson('/api/attendance-records', $payload);
        $second->assertOk()
            ->assertJsonPath('created', false)
            ->assertJsonPath('picture_upload_required', true);

        $this->assertSame(1, AttendanceRecord::count());
        $record = AttendanceRecord::firstOrFail();
        $this->assertSame('esp32-test', $record->bridge_id);
        $this->assertSame('DS-K1A340FWX20240102V010207ENJ59360966', $record->device_key);
        $this->assertSame('1001', $record->employee_no);
        $this->assertSame('faceOrFpOrCardOrPw', $record->current_verify_mode);
        $this->assertTrue($record->picture_expected);
        $this->assertNull($record->picture_content_type);
        $this->assertNull($record->picture_bytes);
        $this->assertNull($record->picture_sha256);
        $this->assertSame(75, $record->raw_event['serialNo']);
        $this->assertArrayNotHasKey('pictureURL', $record->raw_event);
        $this->assertArrayNotHasKey('picture', $record->payload['event']);

        $picture = "\xFF\xD8\xFF\xD9";
        $this->putJpeg($uploadUrl, $picture)
            ->assertOk()
            ->assertJsonPath('picture_stored', true)
            ->assertJsonPath('picture_bytes', strlen($picture))
            ->assertJsonPath('picture_sha256', hash('sha256', $picture));

        $record->refresh();
        $this->assertSame('image/jpeg', $record->picture_content_type);
        $this->assertSame(4, $record->picture_bytes);
        $this->assertSame(hash('sha256', $picture), $record->picture_sha256);
        Storage::disk('local')->assertExists($record->picture_path);

        $third = $this->postJson('/api/attendance-records', $payload);
        $third->assertOk()
            ->assertJsonPath('created', false)
            ->assertJsonPath('picture_upload_required', false)
            ->assertJsonPath('picture_upload_url', null)
            ->assertJsonPath('picture_stored', true);

        $this->get(route('attendance-records.picture', $record))
            ->assertOk()
            ->assertHeader('Content-Type', 'image/jpeg');
    }

    public function test_it_accepts_configured_bridge_token(): void
    {
        Storage::fake('local');
        config(['services.attendance_bridge.token' => 'secret-token']);

        $this->postJson('/api/attendance-records', $this->payload())
            ->assertUnauthorized();

        $response = $this->withToken('secret-token')
            ->postJson('/api/attendance-records', $this->payload())
            ->assertCreated();

        $this->putJpeg($response->json('picture_upload_url'), "\xFF\xD8\xFF\xD9")
            ->assertUnauthorized();

        $this->withToken('secret-token')
            ->call('PUT', $response->json('picture_upload_url'), [], [], [], [
                'CONTENT_TYPE' => 'image/jpeg',
                'HTTP_AUTHORIZATION' => 'Bearer secret-token',
            ], "\xFF\xD8\xFF\xD9")
            ->assertOk();
    }

    public function test_it_streams_device_pictures_larger_than_64_kib(): void
    {
        Storage::fake('local');

        $picture = "\xFF\xD8\xFF".str_repeat("\x00", 66805 - 3);
        $payload = $this->payload();
        $payload['event']['serialNo'] = 118;
        $payload['event']['raw']['serialNo'] = 118;

        $response = $this->postJson('/api/attendance-records', $payload)
            ->assertCreated()
            ->assertJsonPath('event_serial_no', 118);

        $this->putJpeg($response->json('picture_upload_url'), $picture)
            ->assertOk()
            ->assertJsonPath('picture_bytes', strlen($picture))
            ->assertJsonPath('picture_sha256', hash('sha256', $picture));

        $record = AttendanceRecord::firstOrFail();
        $this->assertSame(66805, $record->picture_bytes);
        $this->assertSame(hash('sha256', $picture), $record->picture_sha256);
        Storage::disk('local')->assertExists($record->picture_path);
    }

    public function test_it_rejects_inline_picture_payloads_without_creating_a_record(): void
    {
        Storage::fake('local');

        $payload = $this->payload();
        $payload['event']['picture'] = [
            'contentType' => 'image/jpeg',
            'encoding' => 'base64',
            'bytes' => 4,
            'data' => base64_encode("\xFF\xD8\xFF\xD9"),
        ];

        $this->postJson('/api/attendance-records', $payload)
            ->assertUnprocessable();

        $this->assertSame(0, AttendanceRecord::count());
        Storage::disk('local')->assertMissing('attendance-record-pictures/1.jpg');
    }

    public function test_it_rejects_picture_urls_inside_raw_event_payloads_without_creating_a_record(): void
    {
        Storage::fake('local');

        $payload = $this->payload();
        $payload['event']['raw']['pictureURL'] = 'http://192.168.1.200/example.jpg';

        $this->postJson('/api/attendance-records', $payload)
            ->assertUnprocessable();

        $this->assertSame(0, AttendanceRecord::count());
    }

    public function test_it_accepts_events_that_do_not_have_device_picture_references(): void
    {
        Storage::fake('local');

        $payload = $this->payload();
        $payload['event']['picture_available'] = false;

        $this->postJson('/api/attendance-records', $payload)
            ->assertCreated()
            ->assertJsonPath('picture_upload_required', false)
            ->assertJsonPath('picture_upload_url', null)
            ->assertJsonPath('picture_stored', false);

        $record = AttendanceRecord::firstOrFail();
        $this->assertFalse($record->picture_expected);
        $this->assertNull($record->picture_path);
        $this->assertNull($record->picture_content_type);
        $this->assertNull($record->picture_bytes);
    }

    public function test_picture_upload_requires_jpeg_bytes(): void
    {
        Storage::fake('local');

        $response = $this->postJson('/api/attendance-records', $this->payload())
            ->assertCreated();

        $this->call('PUT', $response->json('picture_upload_url'), [], [], [], [
            'CONTENT_TYPE' => 'application/octet-stream',
        ], "\xFF\xD8\xFF\xD9")->assertUnsupportedMediaType();

        $this->putJpeg($response->json('picture_upload_url'), 'not-a-jpeg')
            ->assertUnprocessable();

        $record = AttendanceRecord::firstOrFail();
        Storage::disk('local')->assertMissing($record->picture_path ?? 'attendance-record-pictures/1.jpg');
    }

    private function payload(): array
    {
        return [
            'schema' => 'hikvision.acs_event.v1',
            'firmware' => '0.1.0',
            'bridge' => [
                'id' => 'esp32-test',
            ],
            'device' => [
                'base_url' => 'http://192.168.1.200',
                'username' => 'admin',
                'name' => 'Time and Attendance Terminal',
                'model' => 'DS-K1A340FWX',
                'serial_number' => 'DS-K1A340FWX20240102V010207ENJ59360966',
                'mac_address' => '40:24:b2:e7:46:04',
            ],
            'event' => [
                'serialNo' => 75,
                'major' => 5,
                'minor' => 75,
                'time' => '2026-07-06T00:38:32+03:00',
                'employeeNoString' => '1001',
                'name' => 'Test User',
                'currentVerifyMode' => 'faceOrFpOrCardOrPw',
                'attendanceStatus' => 'undefined',
                'statusValue' => 0,
                'picture_available' => true,
                'raw' => [
                    'serialNo' => 75,
                    'employeeNoString' => '1001',
                    'attendanceStatus' => 'undefined',
                ],
            ],
        ];
    }

    private function putJpeg(string $url, string $picture)
    {
        return $this->call('PUT', parse_url($url, PHP_URL_PATH) ?: $url, [], [], [], [
            'CONTENT_TYPE' => 'image/jpeg',
        ], $picture);
    }
}
