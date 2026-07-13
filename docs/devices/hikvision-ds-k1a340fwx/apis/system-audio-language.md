# Audio And Language

## Covered Endpoints

- `GET /ISAPI/System/DeviceLanguage`
- `GET /ISAPI/System/DeviceLanguage/capabilities`
- `GET /ISAPI/System/Audio/capabilities`
- `GET /ISAPI/System/Audio/channels`
- `GET /ISAPI/System/Audio/AudioOut/channels/1`
- `GET /ISAPI/System/Audio/AudioOut/channels/1/capabilities`

## Device Language

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/DeviceLanguage"
```

### Observed Response

Status: `200 OK`

```xml
<DeviceLanguage version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <language>English</language>
</DeviceLanguage>
```

### Capability Response

```xml
<DeviceLanguage version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <language opt="English,Russian,BrazilianPortuguese,Indonesian,Spanish,Arabic,Vietnamese,Thai,Japanese,Korean"/>
</DeviceLanguage>
```

## Audio Capability

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Audio/capabilities"
```

### Observed Response

```xml
<AudioCap version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <audioInputNums>0</audioInputNums>
  <audioOutputNums>1</audioOutputNums>
</AudioCap>
```

`GET /ISAPI/System/Audio/channels` returned an empty `AudioChannelList`, so audio input channel enumeration is empty even though one output is present.

## Audio Output Channel 1

### Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/Audio/AudioOut/channels/1"
```

### Observed Response

```xml
<AudioOut version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>1</id>
  <AudioOutVolumelist>
    <AudioOutVlome>
      <type>audioOutput</type>
      <volume>5</volume>
    </AudioOutVlome>
  </AudioOutVolumelist>
</AudioOut>
```

### Capability Response

```xml
<AudioOutCap version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <id>1</id>
  <AudioOutVolumelist>
    <AudioOutVlome>
      <type>audioOutput</type>
      <volume min="0" max="10" def="5">7</volume>
    </AudioOutVlome>
  </AudioOutVolumelist>
</AudioOutCap>
```

## Integration Notes

- Volume is readable and constrained to `0..10`.
- The XML key is misspelled as `AudioOutVlome`; integration code should use the exact returned spelling.
- This is system audio output configuration, separate from access-control TTS text configuration.
