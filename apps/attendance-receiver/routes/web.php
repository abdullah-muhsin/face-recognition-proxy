<?php

use App\Http\Controllers\AttendanceRecordController;
use Illuminate\Support\Facades\Route;

Route::get('/', [AttendanceRecordController::class, 'index']);
Route::get('/attendance-records', [AttendanceRecordController::class, 'index'])
    ->name('attendance-records.index');
