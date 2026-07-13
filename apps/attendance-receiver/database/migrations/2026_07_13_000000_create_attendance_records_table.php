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
            $table->string('schema', 80);
            $table->string('firmware', 40)->nullable();
            $table->string('bridge_id', 64);
            $table->string('device_key', 255);
            $table->string('device_base_url');
            $table->string('device_username', 80)->nullable();
            $table->string('device_name', 120)->nullable();
            $table->string('device_model', 80)->nullable();
            $table->string('device_serial_number', 160)->nullable();
            $table->string('device_mac_address', 40)->nullable();
            $table->unsignedBigInteger('event_serial_no');
            $table->dateTimeTz('event_time')->nullable()->index();
            $table->unsignedSmallInteger('major')->nullable();
            $table->unsignedSmallInteger('minor')->nullable();
            $table->string('employee_no', 80)->nullable()->index();
            $table->string('employee_name', 160)->nullable();
            $table->string('current_verify_mode', 80)->nullable();
            $table->string('attendance_status', 80)->nullable();
            $table->unsignedSmallInteger('status_value')->nullable();
            $table->text('picture_url')->nullable();
            $table->json('raw_event');
            $table->json('payload');
            $table->timestamps();

            $table->unique(['bridge_id', 'device_key', 'event_serial_no'], 'attendance_records_identity_unique');
            $table->index(['device_key', 'event_serial_no']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('attendance_records');
    }
};
