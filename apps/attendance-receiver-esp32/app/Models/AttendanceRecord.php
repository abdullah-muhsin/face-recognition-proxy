<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class AttendanceRecord extends Model
{
    protected $fillable = [
        'source_schema',
        'bridge_firmware',
        'ingestion_source',
        'terminal_id',
        'bridge_identifier',
        'legacy_device_base_url',
        'legacy_device_username',
        'terminal_name',
        'terminal_model',
        'terminal_serial_number',
        'terminal_mac_address',
        'legacy_event_serial_number',
        'vendor_event_id',
        'occurred_at',
        'received_at',
        'legacy_event_major',
        'legacy_event_minor',
        'employee_number',
        'employee_name',
        'verification_method',
        'attendance_status',
        'attendance_status_value',
        'source_payload_hash',
        'picture_expected',
        'picture_path',
        'picture_content_type',
        'picture_bytes',
        'picture_sha256',
        'legacy_raw_event',
        'source_payload',
    ];

    protected function casts(): array
    {
        return [
            'legacy_event_serial_number' => 'integer',
            'occurred_at' => 'immutable_datetime',
            'received_at' => 'immutable_datetime',
            'legacy_event_major' => 'integer',
            'legacy_event_minor' => 'integer',
            'attendance_status_value' => 'integer',
            'picture_expected' => 'boolean',
            'picture_bytes' => 'integer',
            'legacy_raw_event' => 'array',
            'source_payload' => 'array',
        ];
    }

    public function terminal(): BelongsTo
    {
        return $this->belongsTo(AttendanceTerminal::class, 'terminal_id');
    }
}
