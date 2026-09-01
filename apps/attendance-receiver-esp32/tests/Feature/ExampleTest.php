<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ExampleTest extends TestCase
{
    use RefreshDatabase;

    /**
     * A basic test example.
     */
    public function test_the_application_redirects_anonymous_users_to_sign_in(): void
    {
        $response = $this->get('/');

        $response->assertRedirect(route('login'));
    }
}
