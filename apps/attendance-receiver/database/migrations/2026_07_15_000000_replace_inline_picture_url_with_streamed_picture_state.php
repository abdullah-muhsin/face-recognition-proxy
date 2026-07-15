<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('attendance_records', function (Blueprint $table): void {
            $table->boolean('picture_expected')->default(false)->after('status_value');
            $table->string('picture_sha256', 64)->nullable()->after('picture_bytes');
        });

        DB::table('attendance_records')
            ->whereNotNull('picture_path')
            ->update(['picture_expected' => true]);

        Schema::table('attendance_records', function (Blueprint $table): void {
            $table->dropColumn('picture_url');
        });
    }

    public function down(): void
    {
        Schema::table('attendance_records', function (Blueprint $table): void {
            $table->text('picture_url')->nullable()->after('status_value');
            $table->dropColumn([
                'picture_expected',
                'picture_sha256',
            ]);
        });
    }
};
