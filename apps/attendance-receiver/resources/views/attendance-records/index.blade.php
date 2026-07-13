<!DOCTYPE html>
<html lang="{{ str_replace('_', '-', app()->getLocale()) }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="20">
    <title>Attendance Records</title>
    <style>
        :root {
            color: #182230;
            background: #f5f7fa;
            font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }

        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
        }

        .shell {
            max-width: 1440px;
            margin: 0 auto;
            padding: 24px;
        }

        header {
            display: flex;
            align-items: end;
            justify-content: space-between;
            gap: 16px;
            margin-bottom: 18px;
        }

        h1 {
            margin: 0 0 4px;
            font-size: 24px;
            line-height: 1.2;
        }

        .muted {
            color: #667085;
            font-size: 13px;
        }

        .stats {
            display: grid;
            grid-template-columns: repeat(4, minmax(150px, 1fr));
            gap: 12px;
            margin-bottom: 14px;
        }

        .stat,
        .panel {
            background: #ffffff;
            border: 1px solid #d7dde6;
            border-radius: 8px;
        }

        .stat {
            padding: 14px;
        }

        .stat span {
            display: block;
            color: #667085;
            font-size: 12px;
            font-weight: 700;
            text-transform: uppercase;
        }

        .stat strong {
            display: block;
            margin-top: 6px;
            font-size: 22px;
        }

        .toolbar {
            display: flex;
            justify-content: space-between;
            gap: 12px;
            padding: 14px;
            border-bottom: 1px solid #d7dde6;
        }

        form {
            display: flex;
            gap: 8px;
            width: min(560px, 100%);
        }

        input {
            width: 100%;
            border: 1px solid #b8c0cc;
            border-radius: 6px;
            padding: 10px 12px;
            font-size: 14px;
        }

        button,
        .button {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            min-height: 38px;
            border: 0;
            border-radius: 6px;
            padding: 9px 13px;
            background: #155eef;
            color: #ffffff;
            font-size: 14px;
            font-weight: 700;
            text-decoration: none;
            white-space: nowrap;
            cursor: pointer;
        }

        .button.secondary {
            color: #344054;
            background: #eef2f6;
        }

        .table-wrap {
            overflow-x: auto;
        }

        table {
            width: 100%;
            border-collapse: collapse;
            min-width: 1080px;
        }

        th,
        td {
            border-bottom: 1px solid #e4e7ec;
            padding: 11px 12px;
            text-align: left;
            vertical-align: top;
            font-size: 13px;
        }

        th {
            color: #475467;
            background: #f8fafc;
            font-size: 12px;
            font-weight: 800;
            text-transform: uppercase;
        }

        code {
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
            font-size: 12px;
        }

        .badge {
            display: inline-flex;
            border-radius: 999px;
            padding: 3px 8px;
            background: #eef2f6;
            font-size: 12px;
            font-weight: 700;
            white-space: nowrap;
        }

        details {
            max-width: 360px;
        }

        summary {
            color: #155eef;
            cursor: pointer;
            font-weight: 700;
        }

        pre {
            max-height: 260px;
            overflow: auto;
            margin: 8px 0 0;
            border: 1px solid #d7dde6;
            border-radius: 6px;
            padding: 10px;
            background: #101828;
            color: #f9fafb;
            font-size: 12px;
            line-height: 1.5;
        }

        .empty {
            padding: 44px 16px;
            text-align: center;
            color: #667085;
        }

        .pagination {
            padding: 12px 14px;
        }

        @media (max-width: 800px) {
            .shell {
                padding: 14px;
            }

            header,
            .toolbar,
            form {
                align-items: stretch;
                flex-direction: column;
            }

            .stats {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }
        }
    </style>
</head>
<body>
    <div class="shell">
        <header>
            <div>
                <h1>Attendance Records</h1>
                <div class="muted">Auto-refreshes every 20 seconds. Endpoint: <code>POST /api/attendance-records</code></div>
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
