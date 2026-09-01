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
            $table->foreignId('terminal_id')->constrained('attendance_terminals')->cascadeOnDelete();
            $table->string('source_schema', 80);
            $table->string('terminal_name', 120)->nullable();
            $table->string('terminal_model', 80)->nullable();
            $table->string('terminal_serial_number', 160);
            $table->string('vendor_event_id', 160);
            $table->dateTimeTz('occurred_at')->index();
            $table->timestampTz('received_at');
            $table->string('employee_number', 80)->index();
            $table->string('employee_name', 160)->nullable();
            $table->string('verification_method', 80);
            $table->string('attendance_status', 80);
            $table->unsignedSmallInteger('attendance_status_value')->nullable();
            $table->boolean('picture_expected')->default(false);
            $table->string('picture_path')->nullable();
            $table->string('picture_content_type', 120)->nullable();
            $table->unsignedInteger('picture_bytes')->nullable();
            $table->string('picture_sha256', 64)->nullable();
            $table->string('source_payload_hash', 64);
            $table->json('source_payload');
            $table->timestamps();

            $table->unique(
                ['terminal_id', 'vendor_event_id'],
                'attendance_records_terminal_vendor_event_unique',
            );
            $table->index('received_at');
            $table->index('vendor_event_id');
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('attendance_records');
    }
};
