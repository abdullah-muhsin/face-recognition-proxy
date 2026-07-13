#include <inttypes.h>

#include "esp_chip_info.h"
#include "esp_flash.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

static const char *TAG = "esp32_smoke";

void app_main(void)
{
    esp_chip_info_t chip_info;
    uint32_t flash_size = 0;

    esp_chip_info(&chip_info);
    esp_flash_get_size(NULL, &flash_size);

    ESP_LOGI(TAG, "ESP32 smoke firmware is running");
    ESP_LOGI(TAG, "cores=%d revision=%d flash=%" PRIu32 "MB",
             chip_info.cores,
             chip_info.revision,
             flash_size / (1024 * 1024));

    while (true) {
        ESP_LOGI(TAG, "heartbeat");
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}
