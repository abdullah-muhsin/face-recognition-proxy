<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class AttendanceRecord extends Model
{
    protected $fillable = [
        'source_schema',
        'terminal_id',
        'terminal_name',
        'terminal_model',
        'terminal_serial_number',
        'vendor_event_id',
        'occurred_at',
        'received_at',
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
        'source_payload',
    ];

    protected function casts(): array
    {
        return [
            'occurred_at' => 'immutable_datetime',
            'received_at' => 'immutable_datetime',
            'attendance_status_value' => 'integer',
            'picture_expected' => 'boolean',
            'picture_bytes' => 'integer',
            'source_payload' => 'array',
        ];
    }

    public function terminal(): BelongsTo
    {
        return $this->belongsTo(AttendanceTerminal::class, 'terminal_id');
    }
}
