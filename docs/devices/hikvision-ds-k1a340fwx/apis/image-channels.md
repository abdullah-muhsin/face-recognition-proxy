# Image Channels

## Covered Endpoints

- `GET /ISAPI/Image/channels`
- `GET /ISAPI/Image/channels/1`
- `GET /ISAPI/Image/channels/2`

## Purpose

Reads image-channel settings. This terminal exposes image configuration but did not expose full `/ISAPI/Streaming/...` video channels during testing.

## Request: Channel List

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Image/channels"
```

## Observed Response: Channel List

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ImageChannellist version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <ImageChannel>
    <id>1</id>
    <enabled>true</enabled>
    <videoInputID>1</videoInputID>
    <WDR>
      <mode>close</mode>
    </WDR>
    <Sharpness>
      <SharpnessLevel>50</SharpnessLevel>
    </Sharpness>
    <powerLineFrequency>
      <powerLineFrequencyMode>50hz</powerLineFrequencyMode>
    </powerLineFrequency>
    <Color>
      <brightnessLevel>50</brightnessLevel>
      <contrastLevel>50</contrastLevel>
      <saturationLevel>50</saturationLevel>
    </Color>
    <SupplementLight>
      <mode>off</mode>
    </SupplementLight>
  </ImageChannel>
  <ImageChannel>
    <id>2</id>
    <enabled>true</enabled>
    <videoInputID>2</videoInputID>
    <SupplementLight>
      <mode>on</mode>
      <brightnessLimit>100</brightnessLimit>
    </SupplementLight>
  </ImageChannel>
</ImageChannellist>
```

## Request: Channel 1

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Image/channels/1"
```

## Observed Response: Channel 1

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ImageChannel version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>1</id>
  <enabled>true</enabled>
  <videoInputID>1</videoInputID>
  <WDR>
    <mode>close</mode>
  </WDR>
  <Sharpness>
    <SharpnessLevel>50</SharpnessLevel>
  </Sharpness>
  <powerLineFrequency>
    <powerLineFrequencyMode>50hz</powerLineFrequencyMode>
  </powerLineFrequency>
  <Color>
    <brightnessLevel>50</brightnessLevel>
    <contrastLevel>50</contrastLevel>
    <saturationLevel>50</saturationLevel>
  </Color>
  <SupplementLight>
    <mode>off</mode>
  </SupplementLight>
</ImageChannel>
```

## Request: Channel 2

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/Image/channels/2"
```

## Observed Response: Channel 2

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ImageChannel version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>2</id>
  <enabled>true</enabled>
  <videoInputID>2</videoInputID>
  <SupplementLight>
    <mode>on</mode>
    <brightnessLimit>100</brightnessLimit>
  </SupplementLight>
</ImageChannel>
```

## Integration Notes

- `GET /ISAPI/Image/channels/{id}/capabilities` returned `404` for channels 1 and 2.
- `/ISAPI/Streaming/channels`, `/ISAPI/Streaming/channels/101`, and snapshot-style streaming picture endpoints returned `404`.
- Use event `pictureURL` values for captured verification images instead of expecting camera streaming.

