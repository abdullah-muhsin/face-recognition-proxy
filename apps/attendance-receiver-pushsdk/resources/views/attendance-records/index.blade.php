<!DOCTYPE html>
<html lang="{{ str_replace('_', '-', app()->getLocale()) }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="20">
    <title>Push SDK Attendance Demo</title>
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body>
    <div class="shell">
        <header class="page-header">
            <div>
                <h1>Push SDK Attendance Demo</h1>
            </div>
            <div class="header-actions">
                <a class="button secondary" href="{{ route('attendance-records.index') }}">Refresh</a>
                <form method="post" action="{{ route('logout') }}">
                    @csrf
                    <button class="button ghost" type="submit">Sign out</button>
                </form>
                <details class="wipe-control">
                    <summary>Data operations</summary>
                    <form method="post" action="{{ route('attendance-records.wipe') }}">
                        @csrf
                        <label>
                            Type <code>WIPE</code> to confirm
                            <input name="confirmation" autocomplete="off" required>
                        </label>
                        <label>
                            Your password
                            <input name="password" type="password" autocomplete="current-password" required>
                        </label>
                        <button class="danger" type="submit">Wipe all records</button>
                    </form>
                </details>
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
                <span>Terminals</span>
                <strong>{{ number_format($uniqueTerminals) }}</strong>
            </div>
            <div class="stat">
                <span>Latest</span>
                <strong>{{ $latestRecord?->vendor_event_id ?? '-' }}</strong>
                <small>{{ $latestRecord?->occurred_at?->format('M j H:i') ?? 'No events' }}</small>
            </div>
        </section>

        <section class="panel">
            <div class="toolbar">
                <form method="get" action="{{ route('attendance-records.index') }}">
                    <input
                        name="search"
                        value="{{ $search }}"
                        placeholder="Search employee, terminal, or event"
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
                            <th>Terminal</th>
                            <th>Payload</th>
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
                                                alt="Attendance record #{{ $record->vendor_event_id }} picture"
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
                                    <span class="line-muted">{{ $record->employee_number ?: '-' }}</span>
                                </td>
                                <td>
                                    <code>#{{ $record->vendor_event_id }}</code><br>
                                    <span class="line-muted">attendance event</span>
                                </td>
                                <td>
                                    {{ $record->occurred_at?->format('Y-m-d H:i:s P') ?? '-' }}<br>
                                    <span class="line-muted">recv {{ ($record->received_at ?? $record->created_at)->format('H:i:s') }}</span>
                                </td>
                                <td>
                                    <span class="badge">{{ $record->attendance_status ?: 'undefined' }}</span>
                                    <span class="line-muted">value {{ $record->attendance_status_value ?? 0 }}</span>
                                    <span class="line-muted">{{ $record->verification_method ?: '-' }}</span>
                                </td>
                                <td>
                                    <strong>{{ $record->terminal?->display_name ?: $record->terminal_name ?: $record->terminal_model ?: '-' }}</strong>
                                    <span class="line-muted">{{ $record->terminal_model ?: '-' }}</span>
                                    <code>{{ $record->terminal_serial_number }}</code>
                                </td>
                                <td>
                                    <details>
                                        <summary>JSON</summary>
                                        <pre>{{ json_encode($record->source_payload, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE) }}</pre>
                                    </details>
                                </td>
                            </tr>
                        @empty
                            <tr>
                                <td class="empty" colspan="7">No attendance records received yet.</td>
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
