<?php

use App\Http\Controllers\Esp32AttendanceRecordController;
use App\Http\Controllers\PushSdkGatewayAttendanceRecordController;
use Illuminate\Support\Facades\Route;

// ESP32/ISAPI transport. Retained as a legacy integration and deliberately
// separate from the strict gateway-only Push SDK contract below.
Route::post('/attendance-records', [Esp32AttendanceRecordController::class, 'store']);
Route::put('/attendance-records/{attendanceRecord}/picture', [Esp32AttendanceRecordController::class, 'storePicture']);

Route::prefix('/internal/push-sdk')->group(function (): void {
    Route::post('/attendance-records', [PushSdkGatewayAttendanceRecordController::class, 'store']);
    Route::put('/attendance-records/{attendanceRecord}/picture', [PushSdkGatewayAttendanceRecordController::class, 'storePicture']);
});
