# System Capabilities

## Endpoint

`GET /ISAPI/System/capabilities`

Also verified:

`GET /ISAPI/System/capabilities?type=all`

## Purpose

Returns high-level feature flags for system, security, event, access-control, platform, audio, serial, and video-intercom features.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/capabilities"
```

Equivalent full capability request:

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/System/capabilities?type=all"
```

## Observed Response

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<DeviceCap version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <isSupportPreview>false</isSupportPreview>
  <isSupportFactoryReset>true</isSupportFactoryReset>
  <isSupportUpdatefirmware>true</isSupportUpdatefirmware>
  <isSupportDeviceInfo>true</isSupportDeviceInfo>
  <SysCap>
    <isSupportDst>true</isSupportDst>
    <NetworkCap>
      <isSupportWireless>true</isSupportWireless>
      <isSupportPPPoE>false</isSupportPPPoE>
      <isSupportBond>true</isSupportBond>
      <isSupport802_1x>false</isSupport802_1x>
      <isSupportNtp>true</isSupportNtp>
      <isSupportFtp>false</isSupportFtp>
      <isSupportHttps>true</isSupportHttps>
      <SnmpCap>
        <isSupport>false</isSupport>
      </SnmpCap>
      <isSupportEZVIZ>true</isSupportEZVIZ>
      <isSupportEhome>true</isSupportEhome>
      <isSupportEZVIZUnbind>true</isSupportEZVIZUnbind>
    </NetworkCap>
    <SerialCap>
      <rs485PortNums>0</rs485PortNums>
      <supportRS232Config>false</supportRS232Config>
      <rs422PortNums>0</rs422PortNums>
      <rs232PortNums>0</rs232PortNums>
    </SerialCap>
    <AudioCap>
      <audioInputNums>0</audioInputNums>
      <audioOutputNums>1</audioOutputNums>
    </AudioCap>
    <isSupportSubscribeEvent>true</isSupportSubscribeEvent>
    <isSupportTimeCap>true</isSupportTimeCap>
    <isSupportPostUpdateFirmware>true</isSupportPostUpdateFirmware>
  </SysCap>
  <SecurityCap>
    <supportUserNums>32</supportUserNums>
    <userBondIpNums>0</userBondIpNums>
    <userBondMacNums>0</userBondMacNums>
    <isSupCertificate>true</isSupCertificate>
    <issupIllegalLoginLock>true</issupIllegalLoginLock>
    <isSupportOnlineUser>true</isSupportOnlineUser>
    <isSupportAnonymous>false</isSupportAnonymous>
    <isSupportStreamEncryption>false</isSupportStreamEncryption>
    <securityVersion opt="1"/>
    <keyIterateNum>100</keyIterateNum>
    <isSupportUserCheck>true</isSupportUserCheck>
    <isSupportSecurityQuestionConfig>true</isSupportSecurityQuestionConfig>
    <SecurityLimits>
      <LoginPasswordLenLimit min="8" max="16">8</LoginPasswordLenLimit>
      <SecurityAnswerLenLimit min="1" max="128"/>
    </SecurityLimits>
    <isSupportConfigFileImport>true</isSupportConfigFileImport>
    <isSupportConfigFileExport>true</isSupportConfigFileExport>
    <cfgFileSecretKeyLenLimit min="0" max="32">32</cfgFileSecretKeyLenLimit>
    <isIrreversible>true</isIrreversible>
    <salt>0</salt>
    <isSupportSecurityEmail>true</isSupportSecurityEmail>
  </SecurityCap>
  <EventCap>
    <isSupportHDFull>false</isSupportHDFull>
    <isSupportHDError>false</isSupportHDError>
    <isSupportNicBroken>false</isSupportNicBroken>
    <isSupportIpConflict>false</isSupportIpConflict>
    <isSupportIllAccess>false</isSupportIllAccess>
    <isSupportViException>false</isSupportViException>
    <isSupportViMismatch>false</isSupportViMismatch>
    <isSupportRecordException>false</isSupportRecordException>
    <isSupportMotionDetection>false</isSupportMotionDetection>
    <isSupportVideoLoss>true</isSupportVideoLoss>
    <isSupportTamperDetection>false</isSupportTamperDetection>
    <isSupportHumanRecognition>true</isSupportHumanRecognition>
    <isSupportFaceContrast>true</isSupportFaceContrast>
  </EventCap>
  <isSupportFaceCaptureStatistics>true</isSupportFaceCaptureStatistics>
  <VideoIntercomCap>
    <isSupportDeviceId>false</isSupportDeviceId>
    <isSupportOperationTime>false</isSupportOperationTime>
    <isSupportCallElevator>false</isSupportCallElevator>
    <isSupportRelatedDeviceAdress>true</isSupportRelatedDeviceAdress>
    <isSupportCardSectorCheck>true</isSupportCardSectorCheck>
    <isSupportKeyCfg>false</isSupportKeyCfg>
    <isSupportPrivilegePasswordStatus>false</isSupportPrivilegePasswordStatus>
    <isSupportElevatorControlCfg>false</isSupportElevatorControlCfg>
    <isSupportPrivilegePassword>false</isSupportPrivilegePassword>
  </VideoIntercomCap>
  <RacmCap>
    <SecurityLog>
      <isSupportLogServer>true</isSupportLogServer>
    </SecurityLog>
  </RacmCap>
  <isSupportAcsUpdate>true</isSupportAcsUpdate>
  <isSupportDiscoveryMode>true</isSupportDiscoveryMode>
  <isSupportPictureServer>true</isSupportPictureServer>
  <isSupportAlgorithmsInfo>true</isSupportAlgorithmsInfo>
  <isSupportAccessControlCap>true</isSupportAccessControlCap>
  <isSupportClientProxyWEB>true</isSupportClientProxyWEB>
  <OpenPlatformCap>
    <isSupportOpenPlatform>false</isSupportOpenPlatform>
    <maxAPPNum>4</maxAPPNum>
  </OpenPlatformCap>
  <isSupportSNAPConfig>true</isSupportSNAPConfig>
</DeviceCap>
```

## Integration Notes

- Treat this endpoint as the first capability discovery call.
- The `?type=all` variant returned the same top-level `DeviceCap` shape during testing.
- `SysCap/NetworkCap/isSupportBond` differed between the top-level system capability response and `/ISAPI/System/Network/capabilities`; prefer the more specific network endpoint when configuring network behavior.
- Several advertised features use firmware-specific paths or are not exposed through the common endpoint names tested in this documentation.
- Firmware update and factory reset are supported by capability flag but intentionally not tested.
