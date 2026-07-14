<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('attendance_records', function (Blueprint $table): void {
            $table->string('picture_path')->nullable()->after('picture_url');
            $table->string('picture_content_type', 120)->nullable()->after('picture_path');
            $table->unsignedInteger('picture_bytes')->nullable()->after('picture_content_type');
        });
    }

    public function down(): void
    {
        Schema::table('attendance_records', function (Blueprint $table): void {
            $table->dropColumn([
                'picture_path',
                'picture_content_type',
                'picture_bytes',
            ]);
        });
    }
};
