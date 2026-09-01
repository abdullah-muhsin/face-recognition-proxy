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
}
