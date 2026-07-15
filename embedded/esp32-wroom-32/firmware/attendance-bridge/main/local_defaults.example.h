#pragma once

#define DEFAULT_STA_SSID "your-attendance-wifi"
#define DEFAULT_STA_PASSWORD "your-attendance-wifi-password"
// Hosted demo receiver. Override this if you run a private or local receiver.
#define DEFAULT_RECEIVER_URL "http://209.42.26.200:8001/api/attendance-records"
#define DEFAULT_RECEIVER_TOKEN ""

// Most ESP32-WROOM development boards expose the onboard blue LED on GPIO2.
#define BOARD_BLUE_LED_GPIO 2
#define BOARD_BLUE_LED_ACTIVE_LOW 0
