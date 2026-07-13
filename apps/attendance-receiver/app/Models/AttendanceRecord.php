<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class AttendanceRecord extends Model
{
    protected $fillable = [
        'schema',
        'firmware',
        'bridge_id',
        'device_key',
        'device_base_url',
        'device_username',
        'device_name',
        'device_model',
        'device_serial_number',
        'device_mac_address',
        'event_serial_no',
        'event_time',
        'major',
        'minor',
        'employee_no',
        'employee_name',
        'current_verify_mode',
        'attendance_status',
        'status_value',
        'picture_url',
        'raw_event',
        'payload',
    ];

    protected function casts(): array
    {
        return [
            'event_serial_no' => 'integer',
            'event_time' => 'immutable_datetime',
            'major' => 'integer',
            'minor' => 'integer',
            'status_value' => 'integer',
            'raw_event' => 'array',
            'payload' => 'array',
        ];
    }
}
