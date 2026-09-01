#!/bin/sh
set -eu

source_config_path="${PUSHSDK_GATEWAY_CONFIG_PATH:?PUSHSDK_GATEWAY_CONFIG_PATH must be set}"
runtime_config_path="/tmp/gateway.json"

# The host-side configuration is mode 600 and owned by the rootless-Docker
# deploy user. Copy it while container root is mapped to that host user, then
# execute the gateway as the dedicated unprivileged account.
cp -- "$source_config_path" "$runtime_config_path"
chown pushsdk:pushsdk "$runtime_config_path"
chmod 600 "$runtime_config_path"

export PUSHSDK_GATEWAY_CONFIG_PATH="$runtime_config_path"
exec setpriv --reuid=pushsdk --regid=pushsdk --init-groups dotnet PushSdkGateway.dll
