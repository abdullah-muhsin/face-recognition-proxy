<?php

use App\Http\Controllers\AttendanceRecordsController;
use Illuminate\Support\Facades\Route;

Route::get('/', [AttendanceRecordsController::class, 'index']);
Route::get('/attendance-records', [AttendanceRecordsController::class, 'index'])
    ->name('attendance-records.index');
Route::post('/attendance-records/wipe', [AttendanceRecordsController::class, 'wipe'])
    ->name('attendance-records.wipe');
Route::get('/attendance-records/{attendanceRecord}/picture', [AttendanceRecordsController::class, 'picture'])
    ->name('attendance-records.picture');
