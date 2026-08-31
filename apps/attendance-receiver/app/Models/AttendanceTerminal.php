<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

class AttendanceTerminal extends Model
{
    protected $fillable = [
        'serial_number',
        'display_name',
        'model',
        'time_zone',
        'is_active',
    ];

    protected function casts(): array
    {
        return [
            'is_active' => 'boolean',
        ];
    }

    public function records(): HasMany
    {
        return $this->hasMany(AttendanceRecord::class, 'terminal_id');
    }
}
