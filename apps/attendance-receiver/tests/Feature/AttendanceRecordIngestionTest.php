<?php

namespace Tests\Feature;

use App\Models\AttendanceRecord;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class AttendanceRecordIngestionTest extends TestCase
{
    use RefreshDatabase;

    public function test_it_stores_hikvision_attendance_record_payloads_idempotently(): void
    {
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
        $this->assertSame(75, $record->raw_event['serialNo']);
    }

    public function test_it_accepts_configured_bridge_token(): void
    {
        config(['services.attendance_bridge.token' => 'secret-token']);

        $this->postJson('/api/attendance-records', $this->payload())
            ->assertUnauthorized();

        $this->withToken('secret-token')
            ->postJson('/api/attendance-records', $this->payload())
            ->assertCreated();
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
                'base_url' => 'http://192.168.1.3',
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
                'pictureURL' => 'http://192.168.1.3/LOCALS/pic/acsLinkCap/test.jpeg@test',
                'raw' => [
                    'serialNo' => 75,
                    'employeeNoString' => '1001',
                    'attendanceStatus' => 'undefined',
                ],
            ],
        ];
    }
}
