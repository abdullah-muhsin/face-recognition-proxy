<?php

namespace App\Http\Controllers;

use App\Models\AttendanceRecord;
use App\Services\AttendanceRecordPictureStorage;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Storage;
use Illuminate\View\View;
use Symfony\Component\HttpFoundation\StreamedResponse;

class AttendanceRecordsController extends Controller
{
    public function index(Request $request): View
    {
        $search = trim((string) $request->query('search', ''));

        $records = AttendanceRecord::query()
            ->with('terminal')
            ->when($search !== '', function ($query) use ($search): void {
                $query->where(function ($query) use ($search): void {
                    $query->where('employee_number', 'like', "%{$search}%")
                        ->orWhere('employee_name', 'like', "%{$search}%")
                        ->orWhere('terminal_serial_number', 'like', "%{$search}%")
                        ->orWhere('vendor_event_id', 'like', "%{$search}%");
                });
            })
            ->latest('occurred_at')
            ->latest('id')
            ->paginate(50)
            ->withQueryString();

        return view('attendance-records.index', [
            'records' => $records,
            'search' => $search,
            'totalRecords' => AttendanceRecord::count(),
            'latestRecord' => AttendanceRecord::query()->latest('occurred_at')->latest('id')->first(),
            'uniqueEmployees' => AttendanceRecord::query()->whereNotNull('employee_number')->distinct('employee_number')->count('employee_number'),
            'uniqueTerminals' => AttendanceRecord::query()->distinct('terminal_serial_number')->count('terminal_serial_number'),
        ]);
    }

    public function wipe(Request $request): RedirectResponse
    {
        $request->validate([
            'confirmation' => ['required', 'in:WIPE'],
            'password' => ['required', 'current_password'],
        ]);

        $recordCount = AttendanceRecord::query()->count();

        AttendanceRecord::query()->delete();
        Storage::disk('local')->deleteDirectory('attendance-record-pictures');
        Storage::disk('local')->makeDirectory('attendance-record-pictures');

        return redirect()
            ->route('attendance-records.index')
            ->with('status', $recordCount === 1 ? 'Wiped 1 record and its stored pictures.' : "Wiped {$recordCount} records and their stored pictures.");
    }

    public function picture(AttendanceRecord $attendanceRecord): StreamedResponse
    {
        abort_unless(filled($attendanceRecord->picture_path), 404);
        abort_unless(Storage::disk('local')->exists($attendanceRecord->picture_path), 404);

        return response()->stream(function () use ($attendanceRecord): void {
            $stream = Storage::disk('local')->readStream($attendanceRecord->picture_path);
            if (! is_resource($stream)) {
                return;
            }

            while (! feof($stream)) {
                $chunk = fread($stream, 8192);
                if ($chunk === false) {
                    break;
                }

                echo $chunk;
            }

            fclose($stream);
        }, 200, [
            'Content-Type' => $attendanceRecord->picture_content_type ?: AttendanceRecordPictureStorage::CONTENT_TYPE,
            'Content-Length' => (string) ($attendanceRecord->picture_bytes ?? Storage::disk('local')->size($attendanceRecord->picture_path)),
            'Cache-Control' => 'private, max-age=3600',
        ]);
    }
}
