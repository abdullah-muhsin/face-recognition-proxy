<?php

namespace Tests\Feature;

use App\Models\AttendanceRecord;
use App\Models\AttendanceTerminal;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Storage;
use Tests\TestCase;

class PushSdkAttendanceRecordIngestionTest extends TestCase
{
    use RefreshDatabase;

    private const GATEWAY_TOKEN = '0123456789abcdef0123456789abcdef';

    public function test_it_accepts_only_registered_terminals_and_deduplicates_by_terminal_and_source_event_id(): void
    {
        Storage::fake('local');
        config(['attendance.push_sdk_gateway.token' => self::GATEWAY_TOKEN]);
        $terminal = $this->terminal();
        $payload = $this->payload($terminal);

        $first = $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $payload);
        $first->assertCreated()
            ->assertJsonPath('created', true)
            ->assertJsonPath('source_event_id', 'event-00001')
            ->assertJsonPath('picture_upload_required', true)
            ->assertJsonPath('picture_upload_path', '/api/internal/push-sdk/attendance-records/1/picture');

        $second = $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $payload);
        $second->assertOk()
            ->assertJsonPath('created', false);

        $this->assertSame(1, AttendanceRecord::count());
        $record = AttendanceRecord::firstOrFail();
        $this->assertSame($terminal->id, $record->terminal_id);
        $this->assertSame('event-00001', $record->vendor_event_id);
        $this->assertSame('2026-08-31 09:15:30', $record->occurred_at?->utc()->format('Y-m-d H:i:s'));
        $this->assertSame('1001', $record->employee_number);
        $this->assertSame('face', $record->verification_method);
        $this->assertSame('check_in', $record->attendance_status);

        $picture = "\xFF\xD8\xFF\xD9";
        $this->withToken(self::GATEWAY_TOKEN)
            ->call('PUT', $first->json('picture_upload_path'), [], [], [], [
                'CONTENT_TYPE' => 'image/jpeg',
                'HTTP_AUTHORIZATION' => 'Bearer '.self::GATEWAY_TOKEN,
            ], $picture)
            ->assertOk()
            ->assertJsonPath('picture_sha256', hash('sha256', $picture));

        $record->refresh();
        $this->assertSame(4, $record->picture_bytes);
        Storage::disk('local')->assertExists($record->picture_path);
    }

    public function test_it_rejects_an_unregistered_terminal_and_a_different_event_with_the_same_identity(): void
    {
        config(['attendance.push_sdk_gateway.token' => self::GATEWAY_TOKEN]);
        $terminal = $this->terminal();
        $payload = $this->payload($terminal);

        $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $payload)
            ->assertCreated();

        $changedPayload = $payload;
        $changedPayload['event']['employee_number'] = '1002';
        $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $changedPayload)
            ->assertConflict();

        $unknownTerminalPayload = $payload;
        $unknownTerminalPayload['terminal_serial_number'] = 'UNKNOWN-TERMINAL';
        $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $unknownTerminalPayload)
            ->assertForbidden();
    }

    public function test_it_requires_the_configured_gateway_token_and_a_strict_contract(): void
    {
        config(['attendance.push_sdk_gateway.token' => self::GATEWAY_TOKEN]);
        $payload = $this->payload($this->terminal());

        $this->postJson('/api/internal/push-sdk/attendance-records', $payload)
            ->assertUnauthorized();

        $payload['event']['unexpected'] = 'not accepted';
        $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $payload)
            ->assertUnprocessable();

        $payload = $this->payload($this->terminal('SECOND-TERMINAL'));
        $payload['occurred_at'] = '2026-08-31T12:15:30+03:00';
        $this->withToken(self::GATEWAY_TOKEN)
            ->postJson('/api/internal/push-sdk/attendance-records', $payload)
            ->assertUnprocessable();
    }

    private function terminal(string $serial = 'K1T342MFWXE1TEST0001'): AttendanceTerminal
    {
        return AttendanceTerminal::query()->create([
            'serial_number' => $serial,
            'display_name' => 'Front Entrance',
            'model' => 'DS-K1T342MFWX-E1',
            'time_zone' => 'Asia/Baghdad',
            'is_active' => true,
        ]);
    }

    private function payload(AttendanceTerminal $terminal): array
    {
        return [
            'schema' => 'attendance.push-sdk.gateway.v1',
            'terminal_serial_number' => $terminal->serial_number,
            'source_event_id' => 'event-00001',
            'occurred_at' => '2026-08-31T09:15:30Z',
            'event' => [
                'employee_number' => '1001',
                'employee_name' => 'Test User',
                'verification_method' => 'face',
                'attendance_status' => 'check_in',
                'status_value' => 1,
                'picture_expected' => true,
            ],
        ];
    }
}
