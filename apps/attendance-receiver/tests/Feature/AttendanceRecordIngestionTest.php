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
            ->assertJsonPath('event_serial_no', 75);

        $second = $this->postJson('/api/attendance-records', $payload);
        $second->assertOk()
            ->assertJsonPath('created', false);

        $this->assertSame(1, AttendanceRecord::count());
        $record = AttendanceRecord::firstOrFail();
        $this->assertSame('esp32-test', $record->bridge_id);
        $this->assertSame('DS-K1A340FWX20240102V010207ENJ59360966', $record->device_key);
        $this->assertSame('1001', $record->employee_no);
        $this->assertSame('faceOrFpOrCardOrPw', $record->current_verify_mode);
        $this->assertSame('image/jpeg', $record->picture_content_type);
        $this->assertSame(4, $record->picture_bytes);
        $this->assertSame(75, $record->raw_event['serialNo']);
        $this->assertArrayNotHasKey('data', $record->payload['event']['picture']);
        $this->assertTrue($record->payload['event']['picture']['stored_separately']);
        Storage::disk('local')->assertExists($record->picture_path);

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

        $this->withToken('secret-token')
            ->postJson('/api/attendance-records', $this->payload())
            ->assertCreated();
    }

    public function test_it_rejects_incomplete_picture_payloads_without_creating_a_record(): void
    {
        Storage::fake('local');

        $payload = $this->payload();
        unset($payload['event']['picture']['data']);

        $this->postJson('/api/attendance-records', $payload)
            ->assertUnprocessable();

        $this->assertSame(0, AttendanceRecord::count());
        Storage::disk('local')->assertMissing('attendance-record-pictures/1.jpg');
    }

    public function test_it_accepts_events_that_do_not_have_device_picture_references(): void
    {
        Storage::fake('local');

        $payload = $this->payload();
        unset($payload['event']['pictureURL'], $payload['event']['picture']);

        $this->postJson('/api/attendance-records', $payload)
            ->assertCreated()
            ->assertJsonPath('picture_stored', false);

        $record = AttendanceRecord::firstOrFail();
        $this->assertNull($record->picture_path);
        $this->assertNull($record->picture_content_type);
        $this->assertNull($record->picture_bytes);
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
                'pictureURL' => 'http://192.168.1.200/LOCALS/pic/acsLinkCap/test.jpeg@test',
                'picture' => [
                    'contentType' => 'image/jpeg',
                    'encoding' => 'base64',
                    'bytes' => 4,
                    'data' => base64_encode("\xFF\xD8\xFF\xD9"),
                ],
                'raw' => [
                    'serialNo' => 75,
                    'employeeNoString' => '1001',
                    'attendanceStatus' => 'undefined',
                ],
            ],
        ];
    }
}
