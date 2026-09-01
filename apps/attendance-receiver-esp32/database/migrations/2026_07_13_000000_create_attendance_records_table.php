<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('attendance_records', function (Blueprint $table): void {
            $table->id();
            $table->string('source_schema', 80);
            $table->string('bridge_firmware', 40)->nullable();
            $table->string('bridge_identifier', 64)->nullable();
            $table->string('legacy_device_base_url', 255)->nullable();
            $table->string('legacy_device_username', 80)->nullable();
            $table->string('terminal_name', 120)->nullable();
            $table->string('terminal_model', 80)->nullable();
            $table->string('terminal_serial_number', 160);
            $table->string('terminal_mac_address', 40)->nullable();
            $table->unsignedBigInteger('legacy_event_serial_number')->nullable();
            $table->dateTimeTz('occurred_at')->index();
            $table->timestampTz('received_at');
            $table->unsignedSmallInteger('legacy_event_major')->nullable();
            $table->unsignedSmallInteger('legacy_event_minor')->nullable();
            $table->string('employee_number', 80)->nullable()->index();
            $table->string('employee_name', 160)->nullable();
            $table->string('verification_method', 80)->nullable();
            $table->string('attendance_status', 80)->nullable();
            $table->unsignedSmallInteger('attendance_status_value')->nullable();
            $table->boolean('picture_expected')->default(false);
            $table->string('picture_path')->nullable();
            $table->string('picture_content_type', 120)->nullable();
            $table->unsignedInteger('picture_bytes')->nullable();
            $table->string('picture_sha256', 64)->nullable();
            $table->json('legacy_raw_event')->nullable();
            $table->json('source_payload');
            $table->timestamps();

            $table->unique(
                ['bridge_identifier', 'terminal_serial_number', 'legacy_event_serial_number'],
                'attendance_records_esp32_identity_unique',
            );
            $table->index('received_at');
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('attendance_records');
    }
};
