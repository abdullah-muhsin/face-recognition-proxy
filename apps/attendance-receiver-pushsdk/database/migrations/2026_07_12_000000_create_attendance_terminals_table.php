<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('attendance_terminals', function (Blueprint $table): void {
            $table->id();
            $table->string('serial_number', 160)->unique();
            $table->string('display_name', 120);
            $table->string('model', 80);
            $table->string('time_zone', 64);
            $table->boolean('is_active')->default(true);
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('attendance_terminals');
    }
};
