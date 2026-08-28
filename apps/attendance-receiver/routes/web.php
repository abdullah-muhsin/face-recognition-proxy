<?php

use App\Http\Controllers\AttendanceRecordController;
use Illuminate\Support\Facades\Route;

Route::get('/', [AttendanceRecordController::class, 'index']);
Route::get('/attendance-records', [AttendanceRecordController::class, 'index'])
    ->name('attendance-records.index');
Route::post('/attendance-records/wipe', [AttendanceRecordController::class, 'wipe'])
    ->name('attendance-records.wipe');
Route::get('/attendance-records/{attendanceRecord}/picture', [AttendanceRecordController::class, 'picture'])
    ->name('attendance-records.picture');
