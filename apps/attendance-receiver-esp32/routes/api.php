<?php

use App\Http\Controllers\Esp32AttendanceRecordController;
use Illuminate\Support\Facades\Route;

// ESP32 bridge / local ISAPI polling demo.
Route::post('/attendance-records', [Esp32AttendanceRecordController::class, 'store']);
Route::put('/attendance-records/{attendanceRecord}/picture', [Esp32AttendanceRecordController::class, 'storePicture']);
