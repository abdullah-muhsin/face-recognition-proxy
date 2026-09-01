FROM mcr.microsoft.com/dotnet/sdk:10.0@sha256:e1ffd2a92ae84c1291bc1b6887501f8af98e6331e7af6d4c8d37168c5e87a64c AS build

WORKDIR /src

COPY PushSdkGateway.csproj ./
COPY tests/PushSdkGateway.Tests.csproj ./tests/
RUN dotnet restore tests/PushSdkGateway.Tests.csproj

COPY . ./
RUN dotnet publish PushSdkGateway.csproj \
    --configuration Release \
    --no-restore \
    --output /app/publish

FROM build AS test

RUN dotnet test tests/PushSdkGateway.Tests.csproj \
    --configuration Release \
    --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:10.0@sha256:a4556ed033fa96f984bb7a8d348851cb2d36b1281dd2420070045f664fbb5f94 AS runtime

RUN apt-get update \
    && apt-get install --yes --no-install-recommends wget \
    && rm -r -f -- /var/lib/apt/lists/* \
    && groupadd --system pushsdk \
    && useradd --system --gid pushsdk --home-dir /nonexistent --shell /usr/sbin/nologin pushsdk \
    && mkdir --parents /var/lib/pushsdk-gateway \
    && chown pushsdk:pushsdk /var/lib/pushsdk-gateway

WORKDIR /app

ENV ASPNETCORE_URLS=http://+:8080 \
    ASPNETCORE_ENVIRONMENT=Production \
    DOTNET_EnableDiagnostics=0

COPY --from=build /app/publish ./
COPY docker/entrypoint.sh /usr/local/bin/pushsdk-gateway-entrypoint

RUN chmod 755 /usr/local/bin/pushsdk-gateway-entrypoint

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/pushsdk-gateway-entrypoint"]
