<!DOCTYPE html>
<html lang="{{ str_replace('_', '-', app()->getLocale()) }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="20">
    <title>Attendance Records</title>
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body>
    <div class="shell">
        <header>
            <div>
                <h1>Attendance Records</h1>
                <div class="muted">Auto-refreshes every 20 seconds. Endpoint: <code>POST {{ url('/api/attendance-records') }}</code></div>
            </div>
            <a class="button secondary" href="{{ route('attendance-records.index') }}">Refresh</a>
        </header>

        <section class="stats">
            <div class="stat">
                <span>Total Records</span>
                <strong>{{ number_format($totalRecords) }}</strong>
            </div>
            <div class="stat">
                <span>Employees</span>
                <strong>{{ number_format($uniqueEmployees) }}</strong>
            </div>
            <div class="stat">
                <span>Devices</span>
                <strong>{{ number_format($uniqueDevices) }}</strong>
            </div>
            <div class="stat">
                <span>Latest Serial</span>
                <strong>{{ $latestRecord?->event_serial_no ?? '-' }}</strong>
            </div>
        </section>

        <section class="panel">
            <div class="toolbar">
                <form method="get" action="{{ route('attendance-records.index') }}">
                    <input
                        name="search"
                        value="{{ $search }}"
                        placeholder="Search employee, device serial, bridge, or event serial"
                    >
                    <button type="submit">Search</button>
                    @if ($search !== '')
                        <a class="button secondary" href="{{ route('attendance-records.index') }}">Clear</a>
                    @endif
                </form>
                <div class="muted">{{ $records->total() }} matching records</div>
            </div>

            <div class="table-wrap">
                <table>
                    <thead>
                        <tr>
                            <th>Event</th>
                            <th>Time</th>
                            <th>Employee</th>
                            <th>Picture</th>
                            <th>Status</th>
                            <th>Verify Mode</th>
                            <th>Device</th>
                            <th>Bridge</th>
                            <th>Raw</th>
                        </tr>
                    </thead>
                    <tbody>
                        @forelse ($records as $record)
                            <tr>
                                <td>
                                    <code>#{{ $record->event_serial_no }}</code><br>
                                    <span class="muted">major {{ $record->major ?? '-' }} / minor {{ $record->minor ?? '-' }}</span>
                                </td>
                                <td>
                                    {{ $record->event_time?->format('Y-m-d H:i:s P') ?? '-' }}<br>
                                    <span class="muted">received {{ $record->created_at->format('Y-m-d H:i:s') }}</span>
                                </td>
                                <td>
                                    <strong>{{ $record->employee_name ?: '-' }}</strong><br>
                                    <span class="muted">{{ $record->employee_no ?: '-' }}</span>
                                </td>
                                <td>
                                    @if ($record->picture_path)
                                        <a href="{{ route('attendance-records.picture', $record) }}" target="_blank" rel="noopener">
                                            <img
                                                class="record-picture"
                                                src="{{ route('attendance-records.picture', $record) }}"
                                                alt="Attendance record #{{ $record->event_serial_no }} picture"
                                            >
                                        </a>
                                        <br>
                                        <span class="muted">{{ number_format($record->picture_bytes ?? 0) }} bytes</span>
                                    @else
                                        <span class="muted">-</span>
                                    @endif
                                </td>
                                <td>
                                    <span class="badge">{{ $record->attendance_status ?: 'undefined' }}</span><br>
                                    <span class="muted">value {{ $record->status_value ?? 0 }}</span>
                                </td>
                                <td>{{ $record->current_verify_mode ?: '-' }}</td>
                                <td>
                                    {{ $record->device_model ?: '-' }}<br>
                                    <code>{{ $record->device_serial_number ?: $record->device_key }}</code>
                                </td>
                                <td>
                                    <code>{{ $record->bridge_id }}</code><br>
                                    <span class="muted">{{ $record->firmware ?: '-' }}</span>
                                </td>
                                <td>
                                    <details>
                                        <summary>View JSON</summary>
                                        <pre>{{ json_encode($record->raw_event, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE) }}</pre>
                                    </details>
                                </td>
                            </tr>
                        @empty
                            <tr>
                                <td class="empty" colspan="9">No attendance records received yet.</td>
                            </tr>
                        @endforelse
                    </tbody>
                </table>
            </div>

            <div class="pagination">
                {{ $records->links() }}
            </div>
        </section>
    </div>
</body>
</html>
