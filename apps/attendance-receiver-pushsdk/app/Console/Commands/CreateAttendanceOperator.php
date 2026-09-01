<?php

namespace App\Console\Commands;

use App\Models\User;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Hash;
use Illuminate\Support\Facades\Validator;

class CreateAttendanceOperator extends Command
{
    protected $signature = 'attendance:operator:create
                            {email : The operator email address}
                            {--name= : Display name for the operator}';

    protected $description = 'Create a login for the attendance receiver web interface';

    public function handle(): int
    {
        $email = (string) $this->argument('email');
        $name = (string) ($this->option('name') ?: $this->ask('Operator name'));

        $validation = Validator::make([
            'email' => $email,
            'name' => $name,
        ], [
            'email' => ['required', 'email', 'max:255', 'unique:users,email'],
            'name' => ['required', 'string', 'max:255'],
        ]);

        if ($validation->fails()) {
            foreach ($validation->errors()->all() as $message) {
                $this->error($message);
            }

            return self::FAILURE;
        }

        $password = (string) $this->secret('Operator password (at least 16 characters)');
        $confirmation = (string) $this->secret('Confirm operator password');
        if (mb_strlen($password) < 16) {
            $this->error('The operator password must be at least 16 characters.');

            return self::FAILURE;
        }
        if (! hash_equals($password, $confirmation)) {
            $this->error('The passwords do not match.');

            return self::FAILURE;
        }

        User::query()->create([
            'name' => $name,
            'email' => $email,
            'password' => Hash::make($password),
        ]);

        $this->info("Created attendance operator {$email}.");

        return self::SUCCESS;
    }
}
