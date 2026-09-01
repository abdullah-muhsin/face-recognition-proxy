<?php

namespace Tests\Feature;

use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Hash;
use Tests\TestCase;

class OperatorAuthenticationTest extends TestCase
{
    use RefreshDatabase;

    public function test_attendance_data_is_not_available_without_an_operator_session(): void
    {
        $this->get('/')
            ->assertRedirect(route('login'));
    }

    public function test_an_operator_can_sign_in_and_sign_out(): void
    {
        User::factory()->create([
            'email' => 'operator@example.com',
            'password' => Hash::make('correct-horse-battery-staple'),
        ]);

        $this->post(route('login.store'), [
            'email' => 'operator@example.com',
            'password' => 'correct-horse-battery-staple',
        ])->assertRedirect(route('attendance-records.index'));

        $this->get(route('attendance-records.index'))->assertOk();
        $this->post(route('logout'))->assertRedirect(route('login'));
        $this->get(route('attendance-records.index'))->assertRedirect(route('login'));
    }
}
