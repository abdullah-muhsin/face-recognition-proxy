<?php

use App\Http\Controllers\AttendanceRecordsController;
use App\Http\Controllers\OperatorSessionController;
use Illuminate\Support\Facades\Route;

Route::middleware('guest')->group(function (): void {
    Route::get('/login', [OperatorSessionController::class, 'create'])->name('login');
    Route::post('/login', [OperatorSessionController::class, 'store'])
        ->middleware('throttle:5,1')
        ->name('login.store');
});

Route::middleware('auth')->group(function (): void {
    Route::get('/', [AttendanceRecordsController::class, 'index']);
    Route::get('/attendance-records', [AttendanceRecordsController::class, 'index'])
        ->name('attendance-records.index');
    Route::post('/attendance-records/wipe', [AttendanceRecordsController::class, 'wipe'])
        ->name('attendance-records.wipe');
    Route::get('/attendance-records/{attendanceRecord}/picture', [AttendanceRecordsController::class, 'picture'])
        ->name('attendance-records.picture');
    Route::post('/logout', [OperatorSessionController::class, 'destroy'])->name('logout');
});
