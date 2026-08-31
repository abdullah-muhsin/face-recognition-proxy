<?php

return [
    'picture_max_bytes' => (int) env('ATTENDANCE_PICTURE_MAX_BYTES', 2 * 1024 * 1024),

    'push_sdk_gateway' => [
        // This is solely for the gateway-to-Laravel private hop. It is never
        // a terminal credential and must be set in every production deployment.
        'gateway_token' => env('ATTENDANCE_PUSH_GATEWAY_TOKEN', ''),
    ],
];
