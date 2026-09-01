<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ExampleTest extends TestCase
{
    use RefreshDatabase;

    public function test_the_attendance_dashboard_is_public(): void
    {
        $this->get('/')
            ->assertOk()
            ->assertSee('Push SDK Attendance Demo');
    }

    public function test_the_dashboard_preserves_https_behind_the_reverse_proxy(): void
    {
        $this->withServerVariables([
            'REMOTE_ADDR' => '127.0.0.1',
            'HTTP_HOST' => 'attendance.example.test',
        ])
            ->withHeader('X-Forwarded-Proto', 'https')
            ->get('/')
            ->assertOk()
            ->assertSee('https://localhost/attendance-records');
    }
}
