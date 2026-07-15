<?php

use App\Http\Controllers\AttendanceRecordController;
use Illuminate\Support\Facades\Route;

Route::post('/attendance-records', [AttendanceRecordController::class, 'store']);
Route::put('/attendance-records/{attendanceRecord}/picture', [AttendanceRecordController::class, 'storePicture']);
