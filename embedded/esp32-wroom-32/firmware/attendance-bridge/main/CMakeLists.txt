idf_component_register(
    SRCS "main.c"
    INCLUDE_DIRS "."
    REQUIRES
        esp_event
        esp_http_client
        esp_http_server
        esp_netif
        esp_timer
        esp_wifi
        mbedtls
        nvs_flash
)
