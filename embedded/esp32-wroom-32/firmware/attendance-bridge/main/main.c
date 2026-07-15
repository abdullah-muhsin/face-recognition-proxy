#include <ctype.h>
#include <inttypes.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>

#include "cJSON.h"
#include "driver/gpio.h"
#include "esp_chip_info.h"
#include "esp_check.h"
#if CONFIG_MBEDTLS_CERTIFICATE_BUNDLE
#include "esp_crt_bundle.h"
#endif
#include "esp_event.h"
#include "esp_http_client.h"
#include "esp_http_server.h"
#include "esp_log.h"
#include "esp_mac.h"
#include "esp_netif.h"
#include "esp_system.h"
#include "esp_timer.h"
#include "esp_wifi.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include "freertos/semphr.h"
#include "freertos/task.h"
#include "mbedtls/base64.h"
#include "nvs.h"
#include "nvs_flash.h"

#if __has_include("local_defaults.h")
#include "local_defaults.h"
#endif

#define FIRMWARE_VERSION "0.1.0"

#ifndef DEFAULT_STA_SSID
#define DEFAULT_STA_SSID ""
#endif

#ifndef DEFAULT_STA_PASSWORD
#define DEFAULT_STA_PASSWORD ""
#endif

#ifndef DEFAULT_DEVICE_BASE_URL
#define DEFAULT_DEVICE_BASE_URL "http://192.168.1.200"
#endif

#ifndef DEFAULT_DEVICE_USERNAME
#define DEFAULT_DEVICE_USERNAME "admin"
#endif

#ifndef DEFAULT_DEVICE_PASSWORD
#define DEFAULT_DEVICE_PASSWORD "lara1234"
#endif

#ifndef DEFAULT_RECEIVER_URL
#define DEFAULT_RECEIVER_URL "http://192.168.1.2/attendance-receiver/api/attendance-records"
#endif

#ifndef DEFAULT_RECEIVER_TOKEN
#define DEFAULT_RECEIVER_TOKEN ""
#endif

#define DEFAULT_AP_CHANNEL 6
#define DEFAULT_POLL_INTERVAL_SECONDS 5
#define MAX_HIKVISION_RESULTS 10
#define MAX_PAGES_PER_CYCLE 5
#define HTTP_CAPTURE_DEVICE_BYTES (24 * 1024)
#define HTTP_CAPTURE_PICTURE_BYTES (64 * 1024)
#define HTTP_CAPTURE_RECEIVER_BYTES 2048
#define FORM_MAX_BYTES 4096
#define HTML_MAX_BYTES 14000

#ifndef BOARD_BLUE_LED_GPIO
#define BOARD_BLUE_LED_GPIO 2
#endif

#ifndef BOARD_BLUE_LED_ACTIVE_LOW
#define BOARD_BLUE_LED_ACTIVE_LOW 0
#endif

#define WIFI_CONNECTED_BIT BIT0
#define POLL_NOW_BIT BIT1

static const char *TAG = "attendance_bridge";
static const char *NVS_NAMESPACE = "bridge";

typedef struct {
    char bridge_id[32];
    char sta_ssid[33];
    char sta_password[65];
    char ap_ssid[33];
    char ap_password[65];
    bool ap_open;
    uint8_t ap_channel;
    char device_base_url[128];
    char device_username[40];
    char device_password[80];
    char receiver_url[192];
    char receiver_token[96];
    uint32_t poll_interval_seconds;
    uint32_t last_serial;
} bridge_config_t;

typedef struct {
    bool loaded;
    char device_name[72];
    char model[64];
    char serial_number[128];
    char mac_address[32];
} device_identity_t;

typedef struct {
    bool sta_connected;
    char sta_ip[16];
    int64_t boot_us;
    int64_t last_poll_us;
    int64_t last_delivery_us;
    uint32_t last_serial;
    uint32_t failed_polls;
    uint32_t failed_deliveries;
    char last_error[160];
} runtime_status_t;

typedef struct {
    char *data;
    int len;
    int capacity;
    bool overflow;
    char content_type[80];
} http_capture_t;

typedef struct {
    cJSON *item;
    uint32_t serial;
} event_ref_t;

typedef struct {
    uint8_t *data;
    size_t len;
    char content_type[80];
} event_picture_t;

typedef struct {
    char *data;
    size_t len;
    size_t capacity;
    bool overflow;
} json_buffer_t;

static bridge_config_t s_config;
static device_identity_t s_identity;
static runtime_status_t s_status;
static SemaphoreHandle_t s_state_lock;
static EventGroupHandle_t s_events;
static esp_netif_t *s_sta_netif;

static const char *json_string(cJSON *object, const char *key);
static uint32_t json_u32(cJSON *object, const char *key);

static void appendf(char *buffer, size_t capacity, size_t *used, const char *fmt, ...)
{
    if (*used >= capacity) {
        return;
    }

    va_list args;
    va_start(args, fmt);
    int written = vsnprintf(buffer + *used, capacity - *used, fmt, args);
    va_end(args);

    if (written < 0) {
        return;
    }

    size_t available = capacity - *used;
    if ((size_t)written >= available) {
        *used = capacity - 1;
    } else {
        *used += (size_t)written;
    }
}

static void copy_string(char *dest, size_t dest_size, const char *src)
{
    if (dest_size == 0) {
        return;
    }

    if (src == NULL) {
        dest[0] = '\0';
        return;
    }

    snprintf(dest, dest_size, "%s", src);
}

static bool starts_with(const char *value, const char *prefix)
{
    return value != NULL && strncmp(value, prefix, strlen(prefix)) == 0;
}

static bool valid_http_url(const char *value, bool allow_empty)
{
    if (value == NULL || value[0] == '\0') {
        return allow_empty;
    }

    return starts_with(value, "http://") || starts_with(value, "https://");
}

static uint32_t parse_u32_or(const char *value, uint32_t fallback)
{
    if (value == NULL || value[0] == '\0') {
        return fallback;
    }

    char *end = NULL;
    unsigned long parsed = strtoul(value, &end, 10);
    if (end == value || *end != '\0' || parsed > UINT32_MAX) {
        return fallback;
    }

    return (uint32_t)parsed;
}

static void blue_led_set(bool on)
{
#if BOARD_BLUE_LED_GPIO >= 0
    int active_level = BOARD_BLUE_LED_ACTIVE_LOW ? 0 : 1;
    gpio_set_level((gpio_num_t)BOARD_BLUE_LED_GPIO, on ? active_level : !active_level);
#else
    (void)on;
#endif
}

static void blue_led_init(void)
{
#if BOARD_BLUE_LED_GPIO >= 0
    gpio_config_t config = {
        .pin_bit_mask = 1ULL << BOARD_BLUE_LED_GPIO,
        .mode = GPIO_MODE_OUTPUT,
        .pull_up_en = GPIO_PULLUP_DISABLE,
        .pull_down_en = GPIO_PULLDOWN_DISABLE,
        .intr_type = GPIO_INTR_DISABLE,
    };
    ESP_ERROR_CHECK(gpio_config(&config));
    blue_led_set(false);
#endif
}

static void json_appendf(json_buffer_t *buffer, const char *fmt, ...)
{
    if (buffer->overflow || buffer->len >= buffer->capacity) {
        buffer->overflow = true;
        return;
    }

    va_list args;
    va_start(args, fmt);
    int written = vsnprintf(buffer->data + buffer->len, buffer->capacity - buffer->len, fmt, args);
    va_end(args);

    if (written < 0) {
        buffer->overflow = true;
        return;
    }

    if ((size_t)written >= buffer->capacity - buffer->len) {
        buffer->len = buffer->capacity > 0 ? buffer->capacity - 1 : 0;
        buffer->overflow = true;
        return;
    }

    buffer->len += (size_t)written;
}

static void json_append_char(json_buffer_t *buffer, char value)
{
    if (buffer->overflow || buffer->len + 1 >= buffer->capacity) {
        buffer->overflow = true;
        return;
    }

    buffer->data[buffer->len++] = value;
    buffer->data[buffer->len] = '\0';
}

static void json_append_quoted(json_buffer_t *buffer, const char *value)
{
    json_append_char(buffer, '"');
    for (const unsigned char *p = (const unsigned char *)(value != NULL ? value : ""); *p != '\0'; ++p) {
        switch (*p) {
        case '"':
            json_appendf(buffer, "\\\"");
            break;
        case '\\':
            json_appendf(buffer, "\\\\");
            break;
        case '\b':
            json_appendf(buffer, "\\b");
            break;
        case '\f':
            json_appendf(buffer, "\\f");
            break;
        case '\n':
            json_appendf(buffer, "\\n");
            break;
        case '\r':
            json_appendf(buffer, "\\r");
            break;
        case '\t':
            json_appendf(buffer, "\\t");
            break;
        default:
            if (*p < 0x20) {
                json_appendf(buffer, "\\u%04x", *p);
            } else {
                json_append_char(buffer, (char)*p);
            }
            break;
        }
    }
    json_append_char(buffer, '"');
}

static void json_append_string_field(json_buffer_t *buffer, const char *name, const char *value)
{
    json_append_quoted(buffer, name);
    json_append_char(buffer, ':');
    json_append_quoted(buffer, value);
}

static void json_append_u32_field(json_buffer_t *buffer, const char *name, uint32_t value)
{
    json_append_quoted(buffer, name);
    json_appendf(buffer, ":%" PRIu32, value);
}

static bool json_append_base64(json_buffer_t *buffer, const uint8_t *data, size_t len)
{
    if (buffer->overflow || data == NULL || len == 0) {
        return false;
    }

    size_t written = 0;
    int ret = mbedtls_base64_encode((unsigned char *)buffer->data + buffer->len,
                                    buffer->capacity - buffer->len,
                                    &written,
                                    data,
                                    len);
    if (ret != 0 || written >= buffer->capacity - buffer->len) {
        buffer->overflow = true;
        return false;
    }

    buffer->len += written;
    buffer->data[buffer->len] = '\0';
    return true;
}

static void html_escape(const char *input, char *output, size_t output_size)
{
    size_t used = 0;
    output[0] = '\0';

    for (const char *p = input != NULL ? input : ""; *p != '\0' && used + 1 < output_size; ++p) {
        const char *replacement = NULL;
        switch (*p) {
        case '&':
            replacement = "&amp;";
            break;
        case '<':
            replacement = "&lt;";
            break;
        case '>':
            replacement = "&gt;";
            break;
        case '"':
            replacement = "&quot;";
            break;
        case '\'':
            replacement = "&#39;";
            break;
        default:
            break;
        }

        if (replacement != NULL) {
            size_t len = strlen(replacement);
            if (used + len >= output_size) {
                break;
            }
            memcpy(output + used, replacement, len);
            used += len;
        } else {
            output[used++] = *p;
        }
    }

    output[used] = '\0';
}

static int hex_value(char c)
{
    if (c >= '0' && c <= '9') {
        return c - '0';
    }
    if (c >= 'a' && c <= 'f') {
        return c - 'a' + 10;
    }
    if (c >= 'A' && c <= 'F') {
        return c - 'A' + 10;
    }
    return -1;
}

static void url_decode_range(const char *start, size_t length, char *output, size_t output_size)
{
    size_t used = 0;
    for (size_t i = 0; i < length && used + 1 < output_size; ++i) {
        if (start[i] == '+') {
            output[used++] = ' ';
        } else if (start[i] == '%' && i + 2 < length) {
            int hi = hex_value(start[i + 1]);
            int lo = hex_value(start[i + 2]);
            if (hi >= 0 && lo >= 0) {
                output[used++] = (char)((hi << 4) | lo);
                i += 2;
            } else {
                output[used++] = start[i];
            }
        } else {
            output[used++] = start[i];
        }
    }
    output[used] = '\0';
}

static bool form_get(const char *body, const char *name, char *output, size_t output_size)
{
    const char *cursor = body;
    char key[48];

    while (cursor != NULL && *cursor != '\0') {
        const char *amp = strchr(cursor, '&');
        const char *end = amp != NULL ? amp : cursor + strlen(cursor);
        const char *eq = memchr(cursor, '=', (size_t)(end - cursor));

        if (eq != NULL) {
            url_decode_range(cursor, (size_t)(eq - cursor), key, sizeof(key));
            if (strcmp(key, name) == 0) {
                url_decode_range(eq + 1, (size_t)(end - eq - 1), output, output_size);
                return true;
            }
        }

        cursor = amp != NULL ? amp + 1 : NULL;
    }

    if (output_size > 0) {
        output[0] = '\0';
    }
    return false;
}

static bool form_has(const char *body, const char *name)
{
    char value[4];
    return form_get(body, name, value, sizeof(value));
}

static void status_set_error(const char *fmt, ...)
{
    if (s_state_lock == NULL) {
        return;
    }

    char message[sizeof(s_status.last_error)];
    va_list args;
    va_start(args, fmt);
    vsnprintf(message, sizeof(message), fmt, args);
    va_end(args);

    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    copy_string(s_status.last_error, sizeof(s_status.last_error), message);
    xSemaphoreGive(s_state_lock);
}

static void status_clear_error(void)
{
    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    s_status.last_error[0] = '\0';
    xSemaphoreGive(s_state_lock);
}

static void config_snapshot(bridge_config_t *out)
{
    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    *out = s_config;
    xSemaphoreGive(s_state_lock);
}

static void identity_snapshot(device_identity_t *out)
{
    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    *out = s_identity;
    xSemaphoreGive(s_state_lock);
}

static void status_snapshot(runtime_status_t *out)
{
    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    *out = s_status;
    xSemaphoreGive(s_state_lock);
}

static esp_err_t nvs_get_string_or(nvs_handle_t handle, const char *key, char *dest, size_t dest_size)
{
    size_t len = dest_size;
    esp_err_t err = nvs_get_str(handle, key, dest, &len);
    if (err == ESP_ERR_NVS_NOT_FOUND) {
        return ESP_OK;
    }
    return err;
}

static void config_set_defaults(bridge_config_t *config)
{
    memset(config, 0, sizeof(*config));

    uint8_t mac[6] = {0};
    ESP_ERROR_CHECK(esp_read_mac(mac, ESP_MAC_WIFI_STA));

    snprintf(config->bridge_id, sizeof(config->bridge_id), "esp32-%02x%02x%02x%02x%02x%02x",
             mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);
    snprintf(config->ap_ssid, sizeof(config->ap_ssid), "AttendanceBridge-%02X%02X%02X",
             mac[3], mac[4], mac[5]);

    copy_string(config->sta_ssid, sizeof(config->sta_ssid), DEFAULT_STA_SSID);
    copy_string(config->sta_password, sizeof(config->sta_password), DEFAULT_STA_PASSWORD);
    config->ap_password[0] = '\0';
    config->ap_open = true;
    config->ap_channel = DEFAULT_AP_CHANNEL;
    copy_string(config->device_base_url, sizeof(config->device_base_url), DEFAULT_DEVICE_BASE_URL);
    copy_string(config->device_username, sizeof(config->device_username), DEFAULT_DEVICE_USERNAME);
    copy_string(config->device_password, sizeof(config->device_password), DEFAULT_DEVICE_PASSWORD);
    copy_string(config->receiver_url, sizeof(config->receiver_url), DEFAULT_RECEIVER_URL);
    copy_string(config->receiver_token, sizeof(config->receiver_token), DEFAULT_RECEIVER_TOKEN);
    config->poll_interval_seconds = DEFAULT_POLL_INTERVAL_SECONDS;
    config->last_serial = 0;
}

static esp_err_t config_load(void)
{
    bridge_config_t config;
    config_set_defaults(&config);

    nvs_handle_t handle;
    esp_err_t err = nvs_open(NVS_NAMESPACE, NVS_READONLY, &handle);
    if (err == ESP_ERR_NVS_NOT_FOUND) {
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_config = config;
        s_status.last_serial = config.last_serial;
        xSemaphoreGive(s_state_lock);
        return ESP_OK;
    }
    ESP_RETURN_ON_ERROR(err, TAG, "open NVS");

    nvs_get_string_or(handle, "bridge_id", config.bridge_id, sizeof(config.bridge_id));
    nvs_get_string_or(handle, "sta_ssid", config.sta_ssid, sizeof(config.sta_ssid));
    nvs_get_string_or(handle, "sta_pass", config.sta_password, sizeof(config.sta_password));
    nvs_get_string_or(handle, "ap_ssid", config.ap_ssid, sizeof(config.ap_ssid));
    nvs_get_string_or(handle, "ap_pass", config.ap_password, sizeof(config.ap_password));
    nvs_get_string_or(handle, "dev_url", config.device_base_url, sizeof(config.device_base_url));
    nvs_get_string_or(handle, "dev_user", config.device_username, sizeof(config.device_username));
    nvs_get_string_or(handle, "dev_pass", config.device_password, sizeof(config.device_password));
    nvs_get_string_or(handle, "rcv_url", config.receiver_url, sizeof(config.receiver_url));
    nvs_get_string_or(handle, "rcv_token", config.receiver_token, sizeof(config.receiver_token));

    uint8_t ap_open = config.ap_open ? 1 : 0;
    uint8_t ap_channel = config.ap_channel;
    nvs_get_u8(handle, "ap_open", &ap_open);
    nvs_get_u8(handle, "ap_chan", &ap_channel);
    nvs_get_u32(handle, "poll_s", &config.poll_interval_seconds);
    nvs_get_u32(handle, "last_ser", &config.last_serial);
    nvs_close(handle);

    config.ap_open = ap_open != 0;
    config.ap_channel = ap_channel == 0 ? DEFAULT_AP_CHANNEL : ap_channel;
    if (config.poll_interval_seconds < 5) {
        config.poll_interval_seconds = DEFAULT_POLL_INTERVAL_SECONDS;
    }

    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    s_config = config;
    s_status.last_serial = config.last_serial;
    xSemaphoreGive(s_state_lock);

    return ESP_OK;
}

static esp_err_t config_save(const bridge_config_t *config)
{
    nvs_handle_t handle;
    ESP_RETURN_ON_ERROR(nvs_open(NVS_NAMESPACE, NVS_READWRITE, &handle), TAG, "open NVS rw");

    esp_err_t err = ESP_OK;
    err |= nvs_set_str(handle, "bridge_id", config->bridge_id);
    err |= nvs_set_str(handle, "sta_ssid", config->sta_ssid);
    err |= nvs_set_str(handle, "sta_pass", config->sta_password);
    err |= nvs_set_str(handle, "ap_ssid", config->ap_ssid);
    err |= nvs_set_str(handle, "ap_pass", config->ap_password);
    err |= nvs_set_u8(handle, "ap_open", config->ap_open ? 1 : 0);
    err |= nvs_set_u8(handle, "ap_chan", config->ap_channel);
    err |= nvs_set_str(handle, "dev_url", config->device_base_url);
    err |= nvs_set_str(handle, "dev_user", config->device_username);
    err |= nvs_set_str(handle, "dev_pass", config->device_password);
    err |= nvs_set_str(handle, "rcv_url", config->receiver_url);
    err |= nvs_set_str(handle, "rcv_token", config->receiver_token);
    err |= nvs_set_u32(handle, "poll_s", config->poll_interval_seconds);
    err |= nvs_set_u32(handle, "last_ser", config->last_serial);

    if (err == ESP_OK) {
        err = nvs_commit(handle);
    }
    nvs_close(handle);

    if (err == ESP_OK) {
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_config = *config;
        s_status.last_serial = config->last_serial;
        xSemaphoreGive(s_state_lock);
    }

    return err;
}

static esp_err_t persist_last_serial(uint32_t serial)
{
    nvs_handle_t handle;
    ESP_RETURN_ON_ERROR(nvs_open(NVS_NAMESPACE, NVS_READWRITE, &handle), TAG, "open NVS cursor");
    esp_err_t err = nvs_set_u32(handle, "last_ser", serial);
    if (err == ESP_OK) {
        err = nvs_commit(handle);
    }
    nvs_close(handle);

    if (err == ESP_OK) {
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_config.last_serial = serial;
        s_status.last_serial = serial;
        s_status.last_delivery_us = esp_timer_get_time();
        xSemaphoreGive(s_state_lock);
    }

    return err;
}

static void wifi_event_handler(void *arg, esp_event_base_t event_base, int32_t event_id, void *event_data)
{
    (void)arg;

    if (event_base == WIFI_EVENT && event_id == WIFI_EVENT_STA_START) {
        bridge_config_t config;
        config_snapshot(&config);
        if (config.sta_ssid[0] != '\0') {
            esp_wifi_connect();
        }
    } else if (event_base == WIFI_EVENT && event_id == WIFI_EVENT_STA_DISCONNECTED) {
        xEventGroupClearBits(s_events, WIFI_CONNECTED_BIT);
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_status.sta_connected = false;
        s_status.sta_ip[0] = '\0';
        xSemaphoreGive(s_state_lock);

        bridge_config_t config;
        config_snapshot(&config);
        if (config.sta_ssid[0] != '\0') {
            ESP_LOGW(TAG, "STA disconnected, reconnecting");
            esp_wifi_connect();
        }
    } else if (event_base == IP_EVENT && event_id == IP_EVENT_STA_GOT_IP) {
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)event_data;
        xEventGroupSetBits(s_events, WIFI_CONNECTED_BIT);
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_status.sta_connected = true;
        snprintf(s_status.sta_ip, sizeof(s_status.sta_ip), IPSTR, IP2STR(&event->ip_info.ip));
        xSemaphoreGive(s_state_lock);
        ESP_LOGI(TAG, "STA got IP %s", s_status.sta_ip);
    }
}

static esp_err_t wifi_start(void)
{
    bridge_config_t config;
    config_snapshot(&config);

    ESP_ERROR_CHECK(esp_netif_init());
    ESP_ERROR_CHECK(esp_event_loop_create_default());

    esp_netif_create_default_wifi_ap();
    s_sta_netif = esp_netif_create_default_wifi_sta();
    esp_netif_set_default_netif(s_sta_netif);

    wifi_init_config_t wifi_init = WIFI_INIT_CONFIG_DEFAULT();
    ESP_ERROR_CHECK(esp_wifi_init(&wifi_init));

    ESP_ERROR_CHECK(esp_event_handler_instance_register(WIFI_EVENT, ESP_EVENT_ANY_ID, wifi_event_handler, NULL, NULL));
    ESP_ERROR_CHECK(esp_event_handler_instance_register(IP_EVENT, IP_EVENT_STA_GOT_IP, wifi_event_handler, NULL, NULL));

    wifi_config_t ap_config = {0};
    copy_string((char *)ap_config.ap.ssid, sizeof(ap_config.ap.ssid), config.ap_ssid);
    ap_config.ap.ssid_len = strlen(config.ap_ssid);
    ap_config.ap.channel = config.ap_channel;
    ap_config.ap.max_connection = 4;
    ap_config.ap.pmf_cfg.required = false;
    if (config.ap_open || config.ap_password[0] == '\0') {
        ap_config.ap.authmode = WIFI_AUTH_OPEN;
        ap_config.ap.password[0] = '\0';
    } else {
        copy_string((char *)ap_config.ap.password, sizeof(ap_config.ap.password), config.ap_password);
        ap_config.ap.authmode = WIFI_AUTH_WPA2_PSK;
    }

    wifi_config_t sta_config = {0};
    if (config.sta_ssid[0] != '\0') {
        copy_string((char *)sta_config.sta.ssid, sizeof(sta_config.sta.ssid), config.sta_ssid);
        copy_string((char *)sta_config.sta.password, sizeof(sta_config.sta.password), config.sta_password);
        sta_config.sta.scan_method = WIFI_ALL_CHANNEL_SCAN;
        sta_config.sta.failure_retry_cnt = 8;
        sta_config.sta.threshold.authmode = WIFI_AUTH_OPEN;
        sta_config.sta.sae_pwe_h2e = WPA3_SAE_PWE_BOTH;
    }

    ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_APSTA));
    ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_AP, &ap_config));
    ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &sta_config));
    ESP_ERROR_CHECK(esp_wifi_start());

    ESP_LOGI(TAG, "AP started: ssid=%s auth=%s channel=%u",
             config.ap_ssid,
             (config.ap_open || config.ap_password[0] == '\0') ? "open" : "wpa2",
             config.ap_channel);

    if (config.sta_ssid[0] != '\0') {
        ESP_LOGI(TAG, "STA connecting to %s", config.sta_ssid);
    } else {
        ESP_LOGW(TAG, "STA SSID not configured; bridge will stay in setup/AP mode");
    }

    return ESP_OK;
}

static esp_err_t http_capture_handler(esp_http_client_event_t *event)
{
    http_capture_t *capture = (http_capture_t *)event->user_data;
    if (event->event_id == HTTP_EVENT_ON_HEADER && capture != NULL && event->header_key != NULL && event->header_value != NULL) {
        if (strcasecmp(event->header_key, "Content-Type") == 0) {
            copy_string(capture->content_type, sizeof(capture->content_type), event->header_value);
        }
        return ESP_OK;
    }

    if (event->event_id != HTTP_EVENT_ON_DATA || event->data == NULL || event->data_len <= 0) {
        return ESP_OK;
    }

    if (capture == NULL || capture->data == NULL || capture->capacity <= 0) {
        return ESP_OK;
    }

    if (event->client != NULL) {
        int current_status = esp_http_client_get_status_code(event->client);
        if (current_status < 200 || current_status >= 300) {
            return ESP_OK;
        }
    }

    int available = capture->capacity - capture->len - 1;
    if (available <= 0) {
        capture->overflow = true;
        return ESP_OK;
    }

    int to_copy = event->data_len < available ? event->data_len : available;
    memcpy(capture->data + capture->len, event->data, (size_t)to_copy);
    capture->len += to_copy;
    capture->data[capture->len] = '\0';
    if (to_copy < event->data_len) {
        capture->overflow = true;
    }

    return ESP_OK;
}

static esp_err_t perform_http_request_capture(const char *url,
                                              esp_http_client_method_t method,
                                              const char *username,
                                              const char *password,
                                              esp_http_client_auth_type_t auth_type,
                                              const char *bearer_token,
                                              const char *body,
                                              char *response,
                                              int response_capacity,
                                              int *status_code,
                                              int *response_len,
                                              char *content_type,
                                              size_t content_type_size,
                                              bool *response_overflowed)
{
    http_capture_t capture = {
        .data = response,
        .len = 0,
        .capacity = response_capacity,
        .overflow = false,
    };
    if (response_capacity > 0) {
        response[0] = '\0';
    }

    esp_http_client_config_t http_config = {
        .url = url,
        .username = username,
        .password = password,
        .auth_type = auth_type,
        .method = method,
        .timeout_ms = 9000,
        .event_handler = http_capture_handler,
        .user_data = &capture,
        .buffer_size = 2048,
        .buffer_size_tx = 2048,
        .user_agent = "esp32-attendance-bridge/" FIRMWARE_VERSION,
        .disable_auto_redirect = true,
    };

#if CONFIG_MBEDTLS_CERTIFICATE_BUNDLE
    if (starts_with(url, "https://")) {
        http_config.crt_bundle_attach = esp_crt_bundle_attach;
    }
#endif

    esp_http_client_handle_t client = esp_http_client_init(&http_config);
    if (client == NULL) {
        return ESP_ERR_NO_MEM;
    }

    esp_http_client_set_header(client, "Accept", body != NULL ? "application/json" : "*/*");
    if (body != NULL) {
        esp_http_client_set_header(client, "Content-Type", "application/json");
        esp_http_client_set_post_field(client, body, strlen(body));
    }
    if (bearer_token != NULL && bearer_token[0] != '\0') {
        char auth_header[128];
        snprintf(auth_header, sizeof(auth_header), "Bearer %s", bearer_token);
        esp_http_client_set_header(client, "Authorization", auth_header);
        esp_http_client_set_header(client, "X-Bridge-Token", bearer_token);
    }

    esp_err_t err = esp_http_client_perform(client);
    if (status_code != NULL) {
        *status_code = esp_http_client_get_status_code(client);
    }
    if (response_len != NULL) {
        *response_len = capture.len;
    }
    if (content_type != NULL && content_type_size > 0) {
        copy_string(content_type, content_type_size, capture.content_type);
    }
    if (response_overflowed != NULL) {
        *response_overflowed = capture.overflow;
    }
    if (capture.overflow) {
        ESP_LOGW(TAG, "HTTP response from %s exceeded %d bytes", url, response_capacity);
    }
    esp_http_client_cleanup(client);

    return err;
}

static esp_err_t perform_http_request(const char *url,
                                      esp_http_client_method_t method,
                                      const char *username,
                                      const char *password,
                                      esp_http_client_auth_type_t auth_type,
                                      const char *bearer_token,
                                      const char *body,
                                      char *response,
                                      int response_capacity,
                                      int *status_code)
{
    return perform_http_request_capture(url,
                                        method,
                                        username,
                                        password,
                                        auth_type,
                                        bearer_token,
                                        body,
                                        response,
                                        response_capacity,
                                        status_code,
                                        NULL,
                                        NULL,
                                        0,
                                        NULL);
}

static void build_device_url(const bridge_config_t *config, const char *path, char *url, size_t url_size)
{
    size_t base_len = strlen(config->device_base_url);
    bool base_has_slash = base_len > 0 && config->device_base_url[base_len - 1] == '/';
    bool path_has_slash = path[0] == '/';

    if (base_has_slash && path_has_slash) {
        snprintf(url, url_size, "%.*s%s", (int)(base_len - 1), config->device_base_url, path);
    } else if (!base_has_slash && !path_has_slash) {
        snprintf(url, url_size, "%s/%s", config->device_base_url, path);
    } else {
        snprintf(url, url_size, "%s%s", config->device_base_url, path);
    }
}

static esp_err_t hikvision_request(const bridge_config_t *config,
                                   const char *path,
                                   esp_http_client_method_t method,
                                   const char *body,
                                   char *response,
                                   int response_capacity,
                                   int *status_code)
{
    char url[256];
    build_device_url(config, path, url, sizeof(url));
    return perform_http_request(url,
                                method,
                                config->device_username,
                                config->device_password,
                                HTTP_AUTH_TYPE_DIGEST,
                                NULL,
                                body,
                                response,
                                response_capacity,
                                status_code);
}

static esp_err_t receiver_post(const bridge_config_t *config, const char *payload, int *status_code)
{
    char response[HTTP_CAPTURE_RECEIVER_BYTES];
    return perform_http_request(config->receiver_url,
                                HTTP_METHOD_POST,
                                NULL,
                                NULL,
                                HTTP_AUTH_TYPE_NONE,
                                config->receiver_token,
                                payload,
                                response,
                                sizeof(response),
                                status_code);
}

static bool build_picture_url(const bridge_config_t *config, const char *picture_url, char *url, size_t url_size)
{
    if (picture_url == NULL || picture_url[0] == '\0' || url_size == 0) {
        return false;
    }

    int written = 0;
    if (valid_http_url(picture_url, false)) {
        written = snprintf(url, url_size, "%s", picture_url);
    } else {
        size_t base_len = strlen(config->device_base_url);
        bool base_has_slash = base_len > 0 && config->device_base_url[base_len - 1] == '/';
        bool path_has_slash = picture_url[0] == '/';

        if (base_has_slash && path_has_slash) {
            written = snprintf(url, url_size, "%.*s%s", (int)(base_len - 1), config->device_base_url, picture_url);
        } else if (!base_has_slash && !path_has_slash) {
            written = snprintf(url, url_size, "%s/%s", config->device_base_url, picture_url);
        } else {
            written = snprintf(url, url_size, "%s%s", config->device_base_url, picture_url);
        }
    }

    return written > 0 && (size_t)written < url_size;
}

static void normalize_content_type(char *content_type)
{
    if (content_type == NULL || content_type[0] == '\0') {
        return;
    }

    char *semicolon = strchr(content_type, ';');
    if (semicolon != NULL) {
        *semicolon = '\0';
    }
    for (char *p = content_type; *p != '\0'; ++p) {
        *p = (char)tolower((unsigned char)*p);
    }
}

static bool supported_picture_content_type(const char *content_type)
{
    return strcmp(content_type, "image/jpeg") == 0 || strcmp(content_type, "image/png") == 0;
}

static bool picture_bytes_match_content_type(const char *content_type, const uint8_t *data, size_t len)
{
    if (strcmp(content_type, "image/jpeg") == 0) {
        return len >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF;
    }
    if (strcmp(content_type, "image/png") == 0) {
        static const uint8_t png_signature[8] = {0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'};
        return len >= sizeof(png_signature) && memcmp(data, png_signature, sizeof(png_signature)) == 0;
    }

    return false;
}

static void event_picture_free(event_picture_t *picture)
{
    if (picture == NULL) {
        return;
    }

    free(picture->data);
    memset(picture, 0, sizeof(*picture));
}

static bool download_event_picture(const bridge_config_t *config, cJSON *event, event_picture_t *picture)
{
    memset(picture, 0, sizeof(*picture));

    const char *picture_url = json_string(event, "pictureURL");
    if (picture_url[0] == '\0') {
        return true;
    }

    char url[768];
    if (!build_picture_url(config, picture_url, url, sizeof(url))) {
        status_set_error("Picture URL is too long for serial %" PRIu32, json_u32(event, "serialNo"));
        return false;
    }

    uint8_t *data = malloc(HTTP_CAPTURE_PICTURE_BYTES + 1);
    if (data == NULL) {
        status_set_error("No memory for picture buffer on serial %" PRIu32, json_u32(event, "serialNo"));
        return false;
    }

    int status = 0;
    int response_len = 0;
    bool response_overflowed = false;
    char content_type[sizeof(picture->content_type)] = "";
    esp_err_t err = perform_http_request_capture(url,
                                                 HTTP_METHOD_GET,
                                                 config->device_username,
                                                 config->device_password,
                                                 HTTP_AUTH_TYPE_DIGEST,
                                                 NULL,
                                                 NULL,
                                                 (char *)data,
                                                 HTTP_CAPTURE_PICTURE_BYTES + 1,
                                                 &status,
                                                 &response_len,
                                                 content_type,
                                                 sizeof(content_type),
                                                 &response_overflowed);
    uint32_t serial = json_u32(event, "serialNo");
    if (err != ESP_OK || status != 200) {
        status_set_error("Picture download failed for serial %" PRIu32 ": err=%s status=%d",
                         serial,
                         esp_err_to_name(err),
                         status);
        free(data);
        return false;
    }
    if (response_overflowed || response_len <= 0 || response_len > HTTP_CAPTURE_PICTURE_BYTES) {
        status_set_error("Picture size is invalid for serial %" PRIu32 ": bytes=%d", serial, response_len);
        free(data);
        return false;
    }

    normalize_content_type(content_type);
    if (!supported_picture_content_type(content_type)) {
        status_set_error("Unsupported picture content type for serial %" PRIu32 ": %s",
                         serial,
                         content_type[0] != '\0' ? content_type : "-");
        free(data);
        return false;
    }
    if (!picture_bytes_match_content_type(content_type, data, (size_t)response_len)) {
        status_set_error("Picture content type does not match data for serial %" PRIu32, serial);
        free(data);
        return false;
    }

    picture->data = data;
    picture->len = (size_t)response_len;
    copy_string(picture->content_type, sizeof(picture->content_type), content_type);
    return true;
}

static bool extract_xml_tag(const char *xml, const char *tag, char *output, size_t output_size)
{
    char open[48];
    char close[48];
    snprintf(open, sizeof(open), "<%s>", tag);
    snprintf(close, sizeof(close), "</%s>", tag);

    const char *start = strstr(xml, open);
    if (start == NULL) {
        output[0] = '\0';
        return false;
    }
    start += strlen(open);
    const char *end = strstr(start, close);
    if (end == NULL) {
        output[0] = '\0';
        return false;
    }

    size_t len = (size_t)(end - start);
    if (len >= output_size) {
        len = output_size - 1;
    }
    memcpy(output, start, len);
    output[len] = '\0';
    return true;
}

static void fetch_device_identity_if_needed(const bridge_config_t *config)
{
    device_identity_t current;
    identity_snapshot(&current);
    if (current.loaded) {
        return;
    }

    char *response = calloc(1, 8192);
    if (response == NULL) {
        status_set_error("No memory for deviceInfo response");
        return;
    }

    int status = 0;
    esp_err_t err = hikvision_request(config,
                                      "/ISAPI/System/deviceInfo",
                                      HTTP_METHOD_GET,
                                      NULL,
                                      response,
                                      8192,
                                      &status);
    if (err != ESP_OK || status != 200) {
        status_set_error("deviceInfo failed: err=%s status=%d", esp_err_to_name(err), status);
        free(response);
        return;
    }

    device_identity_t identity = {0};
    extract_xml_tag(response, "deviceName", identity.device_name, sizeof(identity.device_name));
    extract_xml_tag(response, "model", identity.model, sizeof(identity.model));
    extract_xml_tag(response, "serialNumber", identity.serial_number, sizeof(identity.serial_number));
    extract_xml_tag(response, "macAddress", identity.mac_address, sizeof(identity.mac_address));
    identity.loaded = identity.serial_number[0] != '\0' || identity.model[0] != '\0';

    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    s_identity = identity;
    xSemaphoreGive(s_state_lock);

    free(response);
}

static const char *json_string(cJSON *object, const char *key)
{
    cJSON *item = cJSON_GetObjectItemCaseSensitive(object, key);
    return cJSON_IsString(item) && item->valuestring != NULL ? item->valuestring : "";
}

static uint32_t json_u32(cJSON *object, const char *key)
{
    cJSON *item = cJSON_GetObjectItemCaseSensitive(object, key);
    if (cJSON_IsNumber(item) && item->valuedouble >= 0) {
        return (uint32_t)item->valuedouble;
    }
    return 0;
}

static char *build_receiver_payload(const bridge_config_t *config, cJSON *event, const event_picture_t *picture)
{
    device_identity_t identity;
    identity_snapshot(&identity);

    char *raw = cJSON_PrintUnformatted(event);
    if (raw == NULL) {
        return NULL;
    }

    bool has_picture = picture != NULL && picture->data != NULL && picture->len > 0;
    size_t picture_base64_len = has_picture ? ((picture->len + 2) / 3) * 4 : 0;
    size_t string_budget = strlen(config->bridge_id) +
                           strlen(config->device_base_url) +
                           strlen(config->device_username) +
                           strlen(identity.device_name) +
                           strlen(identity.model) +
                           strlen(identity.serial_number) +
                           strlen(identity.mac_address) +
                           strlen(json_string(event, "time")) +
                           strlen(json_string(event, "employeeNoString")) +
                           strlen(json_string(event, "name")) +
                           strlen(json_string(event, "currentVerifyMode")) +
                           strlen(json_string(event, "attendanceStatus")) +
                           strlen(json_string(event, "pictureURL"));
    size_t capacity = 8192 + strlen(raw) + picture_base64_len + (string_budget * 2);
    char *payload = calloc(1, capacity);
    if (payload == NULL) {
        cJSON_free(raw);
        return NULL;
    }

    json_buffer_t out = {
        .data = payload,
        .len = 0,
        .capacity = capacity,
        .overflow = false,
    };

    json_append_char(&out, '{');
    json_append_string_field(&out, "schema", "hikvision.acs_event.v1");
    json_append_char(&out, ',');
    json_append_string_field(&out, "firmware", FIRMWARE_VERSION);
    json_appendf(&out, ",\"bridge\":{");
    json_append_string_field(&out, "id", config->bridge_id);
    json_appendf(&out, "},\"device\":{");
    json_append_string_field(&out, "base_url", config->device_base_url);
    json_append_char(&out, ',');
    json_append_string_field(&out, "username", config->device_username);
    json_append_char(&out, ',');
    json_append_string_field(&out, "name", identity.device_name);
    json_append_char(&out, ',');
    json_append_string_field(&out, "model", identity.model);
    json_append_char(&out, ',');
    json_append_string_field(&out, "serial_number", identity.serial_number);
    json_append_char(&out, ',');
    json_append_string_field(&out, "mac_address", identity.mac_address);
    json_appendf(&out, "},\"event\":{");
    json_append_u32_field(&out, "serialNo", json_u32(event, "serialNo"));
    json_append_char(&out, ',');
    json_append_u32_field(&out, "major", json_u32(event, "major"));
    json_append_char(&out, ',');
    json_append_u32_field(&out, "minor", json_u32(event, "minor"));
    json_append_char(&out, ',');
    json_append_string_field(&out, "time", json_string(event, "time"));
    json_append_char(&out, ',');
    json_append_string_field(&out, "employeeNoString", json_string(event, "employeeNoString"));
    json_append_char(&out, ',');
    json_append_string_field(&out, "name", json_string(event, "name"));
    json_append_char(&out, ',');
    json_append_string_field(&out, "currentVerifyMode", json_string(event, "currentVerifyMode"));
    json_append_char(&out, ',');
    json_append_string_field(&out, "attendanceStatus", json_string(event, "attendanceStatus"));
    json_append_char(&out, ',');
    json_append_u32_field(&out, "statusValue", json_u32(event, "statusValue"));
    json_append_char(&out, ',');
    json_append_string_field(&out, "pictureURL", json_string(event, "pictureURL"));
    json_appendf(&out, ",\"raw\":%s", raw);

    if (has_picture) {
        json_appendf(&out, ",\"picture\":{");
        json_append_string_field(&out, "contentType", picture->content_type);
        json_append_char(&out, ',');
        json_append_string_field(&out, "encoding", "base64");
        json_append_char(&out, ',');
        json_append_u32_field(&out, "bytes", (uint32_t)picture->len);
        json_appendf(&out, ",\"data\":\"");
        json_append_base64(&out, picture->data, picture->len);
        json_append_char(&out, '"');
        json_append_char(&out, '}');
    }

    json_appendf(&out, "}}");

    cJSON_free(raw);
    if (out.overflow) {
        free(payload);
        return NULL;
    }

    return payload;
}

static int compare_event_ref(const void *a, const void *b)
{
    const event_ref_t *left = (const event_ref_t *)a;
    const event_ref_t *right = (const event_ref_t *)b;
    if (left->serial < right->serial) {
        return -1;
    }
    if (left->serial > right->serial) {
        return 1;
    }
    return 0;
}

static int poll_hikvision_once(const bridge_config_t *config)
{
    if (!valid_http_url(config->receiver_url, false)) {
        status_set_error("Receiver URL is not configured");
        return -1;
    }

    fetch_device_identity_if_needed(config);

    uint32_t begin_serial = config->last_serial + 1;
    char body[512];
    snprintf(body,
             sizeof(body),
             "{\"AcsEventCond\":{\"searchID\":\"%s\",\"searchResultPosition\":0,\"maxResults\":%d,"
             "\"major\":0,\"minor\":0,\"startTime\":\"2000-01-01T00:00:00+03:00\","
             "\"endTime\":\"2037-12-31T23:59:59+03:00\",\"beginSerialNo\":%" PRIu32 ","
             "\"endSerialNo\":3000000000,\"timeReverseOrder\":false}}",
             config->bridge_id,
             MAX_HIKVISION_RESULTS,
             begin_serial);

    static char response[HTTP_CAPTURE_DEVICE_BYTES + 1];
    memset(response, 0, sizeof(response));

    int status = 0;
    int response_len = 0;
    bool response_overflowed = false;
    char content_type[80] = "";
    char url[256];
    build_device_url(config, "/ISAPI/AccessControl/AcsEvent?format=json", url, sizeof(url));
    esp_err_t err = perform_http_request_capture(url,
                                                 HTTP_METHOD_POST,
                                                 config->device_username,
                                                 config->device_password,
                                                 HTTP_AUTH_TYPE_DIGEST,
                                                 NULL,
                                                 body,
                                                 response,
                                                 sizeof(response),
                                                 &status,
                                                 &response_len,
                                                 content_type,
                                                 sizeof(content_type),
                                                 &response_overflowed);
    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    s_status.last_poll_us = esp_timer_get_time();
    xSemaphoreGive(s_state_lock);

    if (err != ESP_OK || status != 200) {
        status_set_error("AcsEvent failed: err=%s status=%d", esp_err_to_name(err), status);
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_status.failed_polls++;
        xSemaphoreGive(s_state_lock);
        return -1;
    }
    if (response_overflowed) {
        status_set_error("AcsEvent response exceeded %d bytes", HTTP_CAPTURE_DEVICE_BYTES);
        xSemaphoreTake(s_state_lock, portMAX_DELAY);
        s_status.failed_polls++;
        xSemaphoreGive(s_state_lock);
        return -1;
    }

    const char *parse_end = NULL;
    cJSON *root = cJSON_ParseWithOpts(response, &parse_end, false);
    if (root == NULL) {
        size_t head_len = response_len < 120 ? (size_t)response_len : 120;
        size_t tail_len = response_len < 120 ? (size_t)response_len : 120;
        const char *tail = response_len > 120 ? response + response_len - tail_len : response;
        char head[121];
        char tail_buf[121];
        memcpy(head, response, head_len);
        head[head_len] = '\0';
        memcpy(tail_buf, tail, tail_len);
        tail_buf[tail_len] = '\0';
        status_set_error("AcsEvent response was not valid JSON: status=%d bytes=%d type=%s",
                         status,
                         response_len,
                         content_type[0] != '\0' ? content_type : "-");
        ESP_LOGW(TAG, "AcsEvent parse failed near: %s", parse_end != NULL ? parse_end : "(null)");
        ESP_LOGW(TAG, "AcsEvent head: %s", head);
        ESP_LOGW(TAG, "AcsEvent tail: %s", tail_buf);
        return -1;
    }

    cJSON *acs = cJSON_GetObjectItemCaseSensitive(root, "AcsEvent");
    cJSON *info_list = cJSON_IsObject(acs) ? cJSON_GetObjectItemCaseSensitive(acs, "InfoList") : NULL;
    if (!cJSON_IsArray(info_list)) {
        cJSON_Delete(root);
        status_clear_error();
        return 0;
    }

    event_ref_t refs[MAX_HIKVISION_RESULTS] = {0};
    int ref_count = 0;
    cJSON *item = NULL;
    cJSON_ArrayForEach(item, info_list) {
        uint32_t serial = json_u32(item, "serialNo");
        if (serial > config->last_serial && ref_count < MAX_HIKVISION_RESULTS) {
            refs[ref_count].item = item;
            refs[ref_count].serial = serial;
            ref_count++;
        }
    }

    if (ref_count == 0) {
        cJSON_Delete(root);
        status_clear_error();
        return 0;
    }

    qsort(refs, (size_t)ref_count, sizeof(refs[0]), compare_event_ref);

    int delivered = 0;
    for (int i = 0; i < ref_count; ++i) {
        event_picture_t picture;
        if (!download_event_picture(config, refs[i].item, &picture)) {
            xSemaphoreTake(s_state_lock, portMAX_DELAY);
            s_status.failed_deliveries++;
            xSemaphoreGive(s_state_lock);
            break;
        }

        char *payload = build_receiver_payload(config, refs[i].item, &picture);
        event_picture_free(&picture);
        if (payload == NULL) {
            status_set_error("No memory for receiver payload");
            break;
        }

        int receiver_status = 0;
        err = receiver_post(config, payload, &receiver_status);
        cJSON_free(payload);

        if (err != ESP_OK || receiver_status < 200 || receiver_status >= 300) {
            status_set_error("Receiver POST failed for serial %" PRIu32 ": err=%s status=%d",
                             refs[i].serial,
                             esp_err_to_name(err),
                             receiver_status);
            xSemaphoreTake(s_state_lock, portMAX_DELAY);
            s_status.failed_deliveries++;
            xSemaphoreGive(s_state_lock);
            break;
        }

        if (persist_last_serial(refs[i].serial) != ESP_OK) {
            status_set_error("Failed to persist serial %" PRIu32, refs[i].serial);
            break;
        }
        delivered++;
        status_clear_error();
        ESP_LOGI(TAG, "Delivered event serial=%" PRIu32, refs[i].serial);
    }

    cJSON_Delete(root);
    return delivered;
}

static void poll_task(void *arg)
{
    (void)arg;

    while (true) {
        bridge_config_t config;
        config_snapshot(&config);

        TickType_t wait_ticks = pdMS_TO_TICKS(config.poll_interval_seconds * 1000);
        xEventGroupWaitBits(s_events, POLL_NOW_BIT, pdTRUE, pdFALSE, wait_ticks);

        config_snapshot(&config);
        runtime_status_t status;
        status_snapshot(&status);

        if (!status.sta_connected) {
            if (config.sta_ssid[0] != '\0') {
                status_set_error("Waiting for WiFi station connection");
            }
            continue;
        }

        if (!valid_http_url(config.device_base_url, false)) {
            status_set_error("Device base URL must start with http:// or https://");
            continue;
        }

        int pages = 0;
        blue_led_set(true);
        while (pages < MAX_PAGES_PER_CYCLE) {
            config_snapshot(&config);
            int delivered = poll_hikvision_once(&config);
            if (delivered < MAX_HIKVISION_RESULTS) {
                break;
            }
            pages++;
            vTaskDelay(pdMS_TO_TICKS(250));
        }
        blue_led_set(false);
    }
}

static esp_err_t send_html(httpd_req_t *req, const char *message)
{
    bridge_config_t config;
    runtime_status_t status;
    device_identity_t identity;
    config_snapshot(&config);
    status_snapshot(&status);
    identity_snapshot(&identity);

    char sta_ssid[80], ap_ssid[80], device_url[180], device_user[80], receiver_url[240], bridge_id[64];
    char last_error[220], sta_ip[32], device_serial[160], device_model[90], message_html[180];
    html_escape(config.sta_ssid, sta_ssid, sizeof(sta_ssid));
    html_escape(config.ap_ssid, ap_ssid, sizeof(ap_ssid));
    html_escape(config.device_base_url, device_url, sizeof(device_url));
    html_escape(config.device_username, device_user, sizeof(device_user));
    html_escape(config.receiver_url, receiver_url, sizeof(receiver_url));
    html_escape(config.bridge_id, bridge_id, sizeof(bridge_id));
    html_escape(status.last_error, last_error, sizeof(last_error));
    html_escape(status.sta_ip, sta_ip, sizeof(sta_ip));
    html_escape(identity.serial_number, device_serial, sizeof(device_serial));
    html_escape(identity.model, device_model, sizeof(device_model));
    html_escape(message != NULL ? message : "", message_html, sizeof(message_html));

    char *html = calloc(1, HTML_MAX_BYTES);
    if (html == NULL) {
        httpd_resp_send_err(req, HTTPD_500_INTERNAL_SERVER_ERROR, "No memory");
        return ESP_FAIL;
    }

    int64_t uptime_seconds = (esp_timer_get_time() - status.boot_us) / 1000000;
    size_t used = 0;
    appendf(html, HTML_MAX_BYTES, &used,
            "<!doctype html><html><head><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"
            "<title>Attendance Bridge</title><style>"
            ":root{font-family:Arial,sans-serif;color:#17202a;background:#f6f7f9}"
            "body{margin:0}.wrap{max-width:980px;margin:0 auto;padding:22px}"
            "header{display:flex;justify-content:space-between;gap:16px;align-items:flex-start;margin-bottom:18px}"
            "h1{font-size:24px;margin:0 0 6px}h2{font-size:16px;margin:0 0 12px}"
            ".muted{color:#667085;font-size:13px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:14px}"
            "section{background:#fff;border:1px solid #d9dee7;border-radius:8px;padding:16px}"
            "label{display:block;font-size:13px;font-weight:700;margin:12px 0 6px}"
            "input{box-sizing:border-box;width:100%%;padding:10px;border:1px solid #b8c0cc;border-radius:6px;font-size:15px}"
            "input[type=checkbox]{width:auto;margin-right:8px}.row{display:flex;gap:12px;align-items:center}"
            ".row input{width:auto}.status{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px;margin-bottom:14px}"
            ".pill{background:#fff;border:1px solid #d9dee7;border-radius:8px;padding:10px}.pill b{display:block;font-size:12px;color:#667085;margin-bottom:4px}"
            "button{background:#155eef;color:white;border:0;border-radius:6px;padding:11px 14px;font-size:15px;font-weight:700;cursor:pointer}"
            ".secondary{background:#344054}.message{background:#ecfdf3;border-color:#abefc6}.error{background:#fff1f3;border-color:#fecdd6}"
            "</style></head><body><div class=\"wrap\"><header><div><h1>Attendance Bridge</h1>"
            "<div class=\"muted\">Firmware %s &middot; %s</div></div>"
            "<form method=\"post\" action=\"/poll-now\"><button class=\"secondary\" type=\"submit\">Poll now</button></form></header>",
            FIRMWARE_VERSION,
            bridge_id);

    if (message_html[0] != '\0') {
        appendf(html, HTML_MAX_BYTES, &used, "<section class=\"message\">%s</section><br>", message_html);
    }
    if (last_error[0] != '\0') {
        appendf(html, HTML_MAX_BYTES, &used, "<section class=\"error\">%s</section><br>", last_error);
    }

    appendf(html, HTML_MAX_BYTES, &used,
            "<div class=\"status\">"
            "<div class=\"pill\"><b>STA</b>%s</div>"
            "<div class=\"pill\"><b>STA IP</b>%s</div>"
            "<div class=\"pill\"><b>Last serial</b>%" PRIu32 "</div>"
            "<div class=\"pill\"><b>Uptime</b>%" PRId64 "s</div>"
            "<div class=\"pill\"><b>Device</b>%s %s</div>"
            "</div>",
            status.sta_connected ? "connected" : "not connected",
            sta_ip[0] != '\0' ? sta_ip : "-",
            status.last_serial,
            uptime_seconds,
            device_model[0] != '\0' ? device_model : "-",
            device_serial[0] != '\0' ? device_serial : "");

    appendf(html, HTML_MAX_BYTES, &used,
            "<form method=\"post\" action=\"/save\"><div class=\"grid\">"
            "<section><h2>WiFi station</h2>"
            "<label>SSID</label><input name=\"sta_ssid\" maxlength=\"32\" value=\"%s\">"
            "<label>Password</label><input name=\"sta_password\" type=\"password\" maxlength=\"64\" placeholder=\"leave blank to keep current password\">"
            "<label class=\"row\"><input type=\"checkbox\" name=\"sta_clear_password\" value=\"1\">Clear station password</label>"
            "</section>"
            "<section><h2>Setup access point</h2>"
            "<label>AP SSID</label><input name=\"ap_ssid\" maxlength=\"32\" value=\"%s\" required>"
            "<label>Password</label><input name=\"ap_password\" type=\"password\" maxlength=\"64\" placeholder=\"blank keeps current password\">"
            "<label class=\"row\"><input type=\"checkbox\" name=\"ap_open\" value=\"1\" %s>Open AP</label>"
            "<label>Channel</label><input name=\"ap_channel\" type=\"number\" min=\"1\" max=\"13\" value=\"%u\">"
            "</section>"
            "<section><h2>Hikvision terminal</h2>"
            "<label>Base URL</label><input name=\"device_base_url\" maxlength=\"127\" value=\"%s\" required>"
            "<label>Username</label><input name=\"device_username\" maxlength=\"39\" value=\"%s\" required>"
            "<label>Password</label><input name=\"device_password\" type=\"password\" maxlength=\"79\" placeholder=\"leave blank to keep current password\">"
            "</section>"
            "<section><h2>Laravel receiver</h2>"
            "<label>Endpoint URL</label><input name=\"receiver_url\" maxlength=\"191\" value=\"%s\">"
            "<label>API token</label><input name=\"receiver_token\" type=\"password\" maxlength=\"95\" placeholder=\"leave blank to keep current token\">"
            "<label class=\"row\"><input type=\"checkbox\" name=\"receiver_clear_token\" value=\"1\">Clear API token</label>"
            "</section>"
            "<section><h2>Delivery cursor</h2>"
            "<label>Poll interval seconds</label><input name=\"poll_interval_seconds\" type=\"number\" min=\"5\" max=\"3600\" value=\"%" PRIu32 "\">"
            "<label>Last delivered serial</label><input name=\"last_serial\" type=\"number\" min=\"0\" max=\"3000000000\" value=\"%" PRIu32 "\">"
            "</section>"
            "</div><br><button type=\"submit\">Save and reboot</button></form>"
            "<p class=\"muted\">JSON status: <a href=\"/api/status\">/api/status</a></p>"
            "</div></body></html>",
            sta_ssid,
            ap_ssid,
            config.ap_open ? "checked" : "",
            config.ap_channel,
            device_url,
            device_user,
            receiver_url,
            config.poll_interval_seconds,
            config.last_serial);

    httpd_resp_set_type(req, "text/html; charset=utf-8");
    esp_err_t err = httpd_resp_send(req, html, HTTPD_RESP_USE_STRLEN);
    free(html);
    return err;
}

static esp_err_t index_handler(httpd_req_t *req)
{
    return send_html(req, NULL);
}

static void restart_task(void *arg)
{
    (void)arg;
    vTaskDelay(pdMS_TO_TICKS(1200));
    esp_restart();
}

static esp_err_t save_handler(httpd_req_t *req)
{
    if (req->content_len > FORM_MAX_BYTES) {
        httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "Form body too large");
        return ESP_FAIL;
    }

    char *body = calloc(1, req->content_len + 1);
    if (body == NULL) {
        httpd_resp_send_err(req, HTTPD_500_INTERNAL_SERVER_ERROR, "No memory");
        return ESP_FAIL;
    }

    int received = 0;
    while (received < req->content_len) {
        int ret = httpd_req_recv(req, body + received, req->content_len - received);
        if (ret <= 0) {
            free(body);
            httpd_resp_send_err(req, HTTPD_500_INTERNAL_SERVER_ERROR, "Failed to read form body");
            return ESP_FAIL;
        }
        received += ret;
    }
    body[received] = '\0';

    bridge_config_t config;
    config_snapshot(&config);
    char value[256];

    if (form_get(body, "sta_ssid", value, sizeof(value))) {
        copy_string(config.sta_ssid, sizeof(config.sta_ssid), value);
    }
    if (form_has(body, "sta_clear_password")) {
        config.sta_password[0] = '\0';
    } else if (form_get(body, "sta_password", value, sizeof(value)) && value[0] != '\0') {
        copy_string(config.sta_password, sizeof(config.sta_password), value);
    }

    if (form_get(body, "ap_ssid", value, sizeof(value))) {
        copy_string(config.ap_ssid, sizeof(config.ap_ssid), value);
    }
    config.ap_open = form_has(body, "ap_open");
    if (form_get(body, "ap_password", value, sizeof(value)) && value[0] != '\0') {
        copy_string(config.ap_password, sizeof(config.ap_password), value);
    }
    if (config.ap_open) {
        config.ap_password[0] = '\0';
    } else if (strlen(config.ap_password) < 8) {
        free(body);
        httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "AP password must be blank for open mode or at least 8 characters for WPA2");
        return ESP_FAIL;
    }

    if (form_get(body, "ap_channel", value, sizeof(value))) {
        uint32_t channel = parse_u32_or(value, config.ap_channel);
        if (channel < 1 || channel > 13) {
            free(body);
            httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "AP channel must be between 1 and 13");
            return ESP_FAIL;
        }
        config.ap_channel = (uint8_t)channel;
    }

    if (form_get(body, "device_base_url", value, sizeof(value))) {
        copy_string(config.device_base_url, sizeof(config.device_base_url), value);
    }
    if (!valid_http_url(config.device_base_url, false)) {
        free(body);
        httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "Device base URL must start with http:// or https://");
        return ESP_FAIL;
    }
    if (form_get(body, "device_username", value, sizeof(value))) {
        copy_string(config.device_username, sizeof(config.device_username), value);
    }
    if (form_get(body, "device_password", value, sizeof(value)) && value[0] != '\0') {
        copy_string(config.device_password, sizeof(config.device_password), value);
    }

    if (form_get(body, "receiver_url", value, sizeof(value))) {
        copy_string(config.receiver_url, sizeof(config.receiver_url), value);
    }
    if (!valid_http_url(config.receiver_url, true)) {
        free(body);
        httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "Receiver URL must start with http:// or https://");
        return ESP_FAIL;
    }
    if (form_has(body, "receiver_clear_token")) {
        config.receiver_token[0] = '\0';
    } else if (form_get(body, "receiver_token", value, sizeof(value)) && value[0] != '\0') {
        copy_string(config.receiver_token, sizeof(config.receiver_token), value);
    }

    if (form_get(body, "poll_interval_seconds", value, sizeof(value))) {
        uint32_t poll_seconds = parse_u32_or(value, config.poll_interval_seconds);
        if (poll_seconds < 5 || poll_seconds > 3600) {
            free(body);
            httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "Poll interval must be between 5 and 3600 seconds");
            return ESP_FAIL;
        }
        config.poll_interval_seconds = poll_seconds;
    }
    if (form_get(body, "last_serial", value, sizeof(value))) {
        config.last_serial = parse_u32_or(value, config.last_serial);
    }

    free(body);

    if (config_save(&config) != ESP_OK) {
        httpd_resp_send_err(req, HTTPD_500_INTERNAL_SERVER_ERROR, "Failed to save configuration");
        return ESP_FAIL;
    }

    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    memset(&s_identity, 0, sizeof(s_identity));
    xSemaphoreGive(s_state_lock);

    xTaskCreate(restart_task, "restart", 2048, NULL, 5, NULL);
    return send_html(req, "Configuration saved. Rebooting to apply WiFi settings.");
}

static esp_err_t poll_now_handler(httpd_req_t *req)
{
    xEventGroupSetBits(s_events, POLL_NOW_BIT);
    httpd_resp_set_status(req, "303 See Other");
    httpd_resp_set_hdr(req, "Location", "/");
    return httpd_resp_send(req, NULL, 0);
}

static esp_err_t status_handler(httpd_req_t *req)
{
    bridge_config_t config;
    runtime_status_t status;
    device_identity_t identity;
    config_snapshot(&config);
    status_snapshot(&status);
    identity_snapshot(&identity);

    cJSON *root = cJSON_CreateObject();
    cJSON_AddStringToObject(root, "firmware", FIRMWARE_VERSION);
    cJSON_AddStringToObject(root, "bridge_id", config.bridge_id);
    cJSON_AddBoolToObject(root, "sta_connected", status.sta_connected);
    cJSON_AddStringToObject(root, "sta_ip", status.sta_ip);
    cJSON_AddStringToObject(root, "ap_ssid", config.ap_ssid);
    cJSON_AddBoolToObject(root, "ap_open", config.ap_open);
    cJSON_AddStringToObject(root, "device_base_url", config.device_base_url);
    cJSON_AddStringToObject(root, "receiver_url", config.receiver_url);
    cJSON_AddNumberToObject(root, "poll_interval_seconds", config.poll_interval_seconds);
    cJSON_AddNumberToObject(root, "last_serial", status.last_serial);
    cJSON_AddNumberToObject(root, "failed_polls", status.failed_polls);
    cJSON_AddNumberToObject(root, "failed_deliveries", status.failed_deliveries);
    cJSON_AddStringToObject(root, "last_error", status.last_error);

    cJSON *device = cJSON_AddObjectToObject(root, "device_identity");
    cJSON_AddBoolToObject(device, "loaded", identity.loaded);
    cJSON_AddStringToObject(device, "name", identity.device_name);
    cJSON_AddStringToObject(device, "model", identity.model);
    cJSON_AddStringToObject(device, "serial_number", identity.serial_number);
    cJSON_AddStringToObject(device, "mac_address", identity.mac_address);

    char *json = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (json == NULL) {
        httpd_resp_send_err(req, HTTPD_500_INTERNAL_SERVER_ERROR, "No memory");
        return ESP_FAIL;
    }

    httpd_resp_set_type(req, "application/json");
    esp_err_t err = httpd_resp_sendstr(req, json);
    cJSON_free(json);
    return err;
}

static esp_err_t start_web_server(void)
{
    httpd_config_t config = HTTPD_DEFAULT_CONFIG();
    config.max_uri_handlers = 8;
    config.stack_size = 8192;

    httpd_handle_t server = NULL;
    ESP_RETURN_ON_ERROR(httpd_start(&server, &config), TAG, "start HTTP server");

    const httpd_uri_t index_uri = {
        .uri = "/",
        .method = HTTP_GET,
        .handler = index_handler,
    };
    const httpd_uri_t save_uri = {
        .uri = "/save",
        .method = HTTP_POST,
        .handler = save_handler,
    };
    const httpd_uri_t poll_uri = {
        .uri = "/poll-now",
        .method = HTTP_POST,
        .handler = poll_now_handler,
    };
    const httpd_uri_t status_uri = {
        .uri = "/api/status",
        .method = HTTP_GET,
        .handler = status_handler,
    };

    ESP_ERROR_CHECK(httpd_register_uri_handler(server, &index_uri));
    ESP_ERROR_CHECK(httpd_register_uri_handler(server, &save_uri));
    ESP_ERROR_CHECK(httpd_register_uri_handler(server, &poll_uri));
    ESP_ERROR_CHECK(httpd_register_uri_handler(server, &status_uri));
    ESP_LOGI(TAG, "Configuration UI started");
    return ESP_OK;
}

void app_main(void)
{
    s_state_lock = xSemaphoreCreateMutex();
    s_events = xEventGroupCreate();
    if (s_state_lock == NULL || s_events == NULL) {
        ESP_LOGE(TAG, "Failed to create synchronization primitives");
        return;
    }

    xSemaphoreTake(s_state_lock, portMAX_DELAY);
    memset(&s_status, 0, sizeof(s_status));
    s_status.boot_us = esp_timer_get_time();
    xSemaphoreGive(s_state_lock);

    esp_err_t err = nvs_flash_init();
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        ESP_ERROR_CHECK(nvs_flash_erase());
        err = nvs_flash_init();
    }
    ESP_ERROR_CHECK(err);

    ESP_ERROR_CHECK(config_load());

    esp_chip_info_t chip_info;
    esp_chip_info(&chip_info);
    ESP_LOGI(TAG, "ESP32 attendance bridge firmware %s", FIRMWARE_VERSION);
    ESP_LOGI(TAG, "cores=%d revision=%d", chip_info.cores, chip_info.revision);

    blue_led_init();
    ESP_ERROR_CHECK(wifi_start());
    ESP_ERROR_CHECK(start_web_server());
    xTaskCreate(poll_task, "poll_task", 12288, NULL, 5, NULL);
}
