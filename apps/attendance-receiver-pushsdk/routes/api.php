<?php

use App\Http\Controllers\PushSdkGatewayAttendanceRecordController;
use Illuminate\Support\Facades\Route;

// This route is reachable only from the dedicated Push SDK gateway.
Route::prefix('/internal/push-sdk')->group(function (): void {
    Route::post('/attendance-records', [PushSdkGatewayAttendanceRecordController::class, 'store']);
    Route::put('/attendance-records/{attendanceRecord}/picture', [PushSdkGatewayAttendanceRecordController::class, 'storePicture']);
});
