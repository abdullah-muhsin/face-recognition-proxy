<!DOCTYPE html>
<html lang="{{ str_replace('_', '-', app()->getLocale()) }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Sign in · Attendance Receiver</title>
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body>
    <main class="login-shell">
        <section class="login-panel">
            <h1>Attendance Receiver</h1>
            <p>Sign in to view attendance records and stored images.</p>

            <form method="post" action="{{ route('login.store') }}">
                @csrf
                <label>
                    Email
                    <input name="email" type="email" value="{{ old('email') }}" autocomplete="username" required autofocus>
                </label>
                <label>
                    Password
                    <input name="password" type="password" autocomplete="current-password" required>
                </label>
                @error('email')
                    <p class="form-error">{{ $message }}</p>
                @enderror
                <button type="submit">Sign in</button>
            </form>
        </section>
    </main>
</body>
</html>
