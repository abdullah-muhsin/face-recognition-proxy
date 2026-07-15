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
        <header class="page-header">
            <div>
                <h1>Attendance Records</h1>
            </div>
            <div class="header-actions">
                <a class="button secondary" href="{{ route('attendance-records.index') }}">Refresh</a>
                <form class="inline-form" method="post" action="{{ route('attendance-records.wipe') }}" onsubmit="return confirm('Delete all attendance records and stored pictures?');">
                    @csrf
                    <button class="danger" type="submit">Wipe Records</button>
                </form>
            </div>
        </header>

        @if (session('status'))
            <div class="notice" role="status">{{ session('status') }}</div>
        @endif

        <section class="stats">
            <div class="stat">
                <span>Records</span>
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
                <span>Latest</span>
                <strong>{{ $latestRecord?->event_serial_no ?? '-' }}</strong>
                <small>{{ $latestRecord?->event_time?->format('M j H:i') ?? 'No events' }}</small>
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
                        <a class="button ghost" href="{{ route('attendance-records.index') }}">Clear</a>
                    @endif
                </form>
                <div class="result-count">{{ number_format($records->total()) }} shown</div>
            </div>

            <div class="table-wrap">
                <table>
                    <thead>
                        <tr>
                            <th>Face</th>
                            <th>Employee</th>
                            <th>Event</th>
                            <th>Time</th>
                            <th>Status</th>
                            <th>Device</th>
                            <th>Bridge</th>
                            <th>Raw</th>
                        </tr>
                    </thead>
                    <tbody>
                        @forelse ($records as $record)
                            <tr>
                                <td>
                                    @if ($record->picture_path)
                                        <a class="picture-link" href="{{ route('attendance-records.picture', $record) }}" target="_blank" rel="noopener">
                                            <img
                                                class="record-picture"
                                                src="{{ route('attendance-records.picture', $record) }}"
                                                alt="Attendance record #{{ $record->event_serial_no }} picture"
                                            >
                                        </a>
                                        <span class="tiny">{{ number_format($record->picture_bytes ?? 0) }} bytes</span>
                                    @elseif ($record->picture_expected)
                                        <span class="picture-missing">Pending</span>
                                    @else
                                        <span class="picture-missing muted">None</span>
                                    @endif
                                </td>
                                <td>
                                    <strong>{{ $record->employee_name ?: '-' }}</strong>
                                    <span class="line-muted">{{ $record->employee_no ?: '-' }}</span>
                                </td>
                                <td>
                                    <code>#{{ $record->event_serial_no }}</code><br>
                                    <span class="line-muted">major {{ $record->major ?? '-' }} / minor {{ $record->minor ?? '-' }}</span>
                                </td>
                                <td>
                                    {{ $record->event_time?->format('Y-m-d H:i:s P') ?? '-' }}<br>
                                    <span class="line-muted">recv {{ $record->created_at->format('H:i:s') }}</span>
                                </td>
                                <td>
                                    <span class="badge">{{ $record->attendance_status ?: 'undefined' }}</span>
                                    <span class="line-muted">value {{ $record->status_value ?? 0 }}</span>
                                    <span class="line-muted">{{ $record->current_verify_mode ?: '-' }}</span>
                                </td>
                                <td>
                                    <strong>{{ $record->device_name ?: $record->device_model ?: '-' }}</strong>
                                    <span class="line-muted">{{ $record->device_model ?: '-' }}</span>
                                    <code>{{ $record->device_serial_number ?: $record->device_key }}</code>
                                </td>
                                <td>
                                    <code>{{ $record->bridge_id }}</code><br>
                                    <span class="line-muted">{{ $record->firmware ?: '-' }}</span>
                                </td>
                                <td>
                                    <details>
                                        <summary>JSON</summary>
                                        <pre>{{ json_encode($record->raw_event, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE) }}</pre>
                                    </details>
                                </td>
                            </tr>
                        @empty
                            <tr>
                                <td class="empty" colspan="8">No attendance records received yet.</td>
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
