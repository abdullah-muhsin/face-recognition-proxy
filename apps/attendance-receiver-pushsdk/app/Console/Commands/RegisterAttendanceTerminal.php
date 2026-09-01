<?php

namespace App\Console\Commands;

use App\Models\AttendanceTerminal;
use DateTimeZone;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Validator;

class RegisterAttendanceTerminal extends Command
{
    protected $signature = 'attendance:terminal:register
                            {serial : The immutable terminal serial number}
                            {--name= : Human-readable terminal name}
                            {--model=DS-K1T342MFWX-E1 : Exact terminal model}
                            {--time-zone=Asia/Baghdad : IANA time zone configured on the terminal}';

    protected $description = 'Register an active terminal that the Push SDK gateway may submit for';

    public function handle(): int
    {
        $terminal = [
            'serial_number' => (string) $this->argument('serial'),
            'display_name' => (string) ($this->option('name') ?: $this->ask('Terminal name')),
            'model' => (string) $this->option('model'),
            'time_zone' => (string) $this->option('time-zone'),
        ];

        $validation = Validator::make($terminal, [
            'serial_number' => ['required', 'string', 'max:160', 'regex:/\A[A-Za-z0-9._-]+\z/D', 'unique:attendance_terminals,serial_number'],
            'display_name' => ['required', 'string', 'max:120'],
            'model' => ['required', 'string', 'max:80'],
            'time_zone' => ['required', 'in:'.implode(',', DateTimeZone::listIdentifiers())],
        ]);

        if ($validation->fails()) {
            foreach ($validation->errors()->all() as $message) {
                $this->error($message);
            }

            return self::FAILURE;
        }

        AttendanceTerminal::query()->create($terminal);
        $this->info("Registered terminal {$terminal['serial_number']}.");

        return self::SUCCESS;
    }
}
