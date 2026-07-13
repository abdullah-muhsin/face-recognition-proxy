# Access-Control Capabilities

## Endpoint

`GET /ISAPI/AccessControl/capabilities`

## Purpose

Returns the access-control/time-attendance feature map for this terminal.

## Request

```bash
curl --digest -u "$ISAPI_USER:$ISAPI_PASS" \
  "$ISAPI_BASE/ISAPI/AccessControl/capabilities"
```

## Observed Response

Status: `200 OK`

Content-Type: `application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<AccessControl version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <isSupportSNAPConfig>true</isSupportSNAPConfig>
  <isSupportIdentityTerminal>true</isSupportIdentityTerminal>
  <isSupportDeployInfo>true</isSupportDeployInfo>
  <isSupportSubmarineBackReader>false</isSupportSubmarineBackReader>
  <isSupportStartReaderInfo>false</isSupportStartReaderInfo>
  <isSupportFaceCompareCond>false</isSupportFaceCompareCond>
  <isSupportRemoteControlDoor>false</isSupportRemoteControlDoor>
  <isSupportUserInfo>true</isSupportUserInfo>
  <EmployeeNoInfo>
    <employeeNo min="1" max="32">10</employeeNo>
    <characterType opt="any"/>
  </EmployeeNoInfo>
  <isSupportCardInfo>false</isSupportCardInfo>
  <isSupportFDLib>true</isSupportFDLib>
  <isSupportUserInfoDetailDelete>true</isSupportUserInfoDetailDelete>
  <isSupportFingerPrintCfg>true</isSupportFingerPrintCfg>
  <isSupportFingerPrintDelete>true</isSupportFingerPrintDelete>
  <isSupportCaptureFingerPrint>true</isSupportCaptureFingerPrint>
  <isSupportDoorStatusWeekPlanCfg>false</isSupportDoorStatusWeekPlanCfg>
  <isSupportVerifyWeekPlanCfg>true</isSupportVerifyWeekPlanCfg>
  <isSupportCardRightWeekPlanCfg>true</isSupportCardRightWeekPlanCfg>
  <isSupportDoorStatusHolidayPlanCfg>false</isSupportDoorStatusHolidayPlanCfg>
  <isSupportVerifyHolidayPlanCfg>true</isSupportVerifyHolidayPlanCfg>
  <isSupportCardRightHolidayPlanCfg>true</isSupportCardRightHolidayPlanCfg>
  <isSupportDoorStatusHolidayGroupCfg>false</isSupportDoorStatusHolidayGroupCfg>
  <isSupportVerifyHolidayGroupCfg>true</isSupportVerifyHolidayGroupCfg>
  <isSupportUserRightHolidayGroupCfg>true</isSupportUserRightHolidayGroupCfg>
  <isSupportDoorStatusPlanTemplate>false</isSupportDoorStatusPlanTemplate>
  <isSupportVerifyPlanTemplate>true</isSupportVerifyPlanTemplate>
  <isSupportUserRightPlanTemplate>true</isSupportUserRightPlanTemplate>
  <isSupportDoorStatusPlan>false</isSupportDoorStatusPlan>
  <isSupportCardReaderPlan>true</isSupportCardReaderPlan>
  <isSupportClearPlansCfg>true</isSupportClearPlansCfg>
  <isSupportEventCardLinkageCfg>true</isSupportEventCardLinkageCfg>
  <isSupportClearEventCardLinkageCfg>true</isSupportClearEventCardLinkageCfg>
  <isSupportAcsEvent>true</isSupportAcsEvent>
  <isSupportAcsEventTotalNum>true</isSupportAcsEventTotalNum>
  <isSupportEventOptimizationCfg>true</isSupportEventOptimizationCfg>
  <isSupportAcsWorkStatus>true</isSupportAcsWorkStatus>
  <isSupportDoorCfg>true</isSupportDoorCfg>
  <isSupportCardReaderCfg>true</isSupportCardReaderCfg>
  <isSupportAcsCfg>true</isSupportAcsCfg>
  <isSupportGroupCfg>false</isSupportGroupCfg>
  <isSupportClearGroupCfg>false</isSupportClearGroupCfg>
  <isSupportMultiCardCfg>false</isSupportMultiCardCfg>
  <isSupportAntiSneakCfg>false</isSupportAntiSneakCfg>
  <isSupportCardReaderAntiSneakCfg>false</isSupportCardReaderAntiSneakCfg>
  <isSupportClearAntiSneakCfg>false</isSupportClearAntiSneakCfg>
  <isSupportAttendanceStatusModeCfg>true</isSupportAttendanceStatusModeCfg>
  <isSupportAttendanceStatusRuleCfg>true</isSupportAttendanceStatusRuleCfg>
  <FactoryReset>
    <isSupportFactoryReset>true</isSupportFactoryReset>
    <mode opt="full,basic,part"/>
  </FactoryReset>
  <isSupportCaptureFace>true</isSupportCaptureFace>
  <isSupportCaptureInfraredFace>true</isSupportCaptureInfraredFace>
  <isSupportFaceRecognizeMode>true</isSupportFaceRecognizeMode>
  <isSupportTTSText>true</isSupportTTSText>
  <isSupportMaintenanceDataExport>false</isSupportMaintenanceDataExport>
  <isSupportKeyCfgAttendance>true</isSupportKeyCfgAttendance>
  <isSupportAsyncImportDatas>true</isSupportAsyncImportDatas>
  <isSupportAsyncImportPic>true</isSupportAsyncImportPic>
  <isSupportFingerDataEncryption>true</isSupportFingerDataEncryption>
  <isSupportAttendanceWeekPlan>true</isSupportAttendanceWeekPlan>
  <isSupportClearAttendancePlan>true</isSupportClearAttendancePlan>
  <isSupportAttendanceMode>true</isSupportAttendanceMode>
  <isSupportAttendancePlanTemplate>true</isSupportAttendancePlanTemplate>
  <isSupportAttendancePlanTemplateList>true</isSupportAttendancePlanTemplateList>
  <isSupportClearPictureCfg>true</isSupportClearPictureCfg>
  <isSupportEventStorageCfg>true</isSupportEventStorageCfg>
  <isSupportTTSTextHolidayPlan>true</isSupportTTSTextHolidayPlan>
  <isSupportTTSTextSearchHolidayPlan>true</isSupportTTSTextSearchHolidayPlan>
  <LocalAttendanceCap>
    <isSupportGroupSearch>true</isSupportGroupSearch>
    <isSupportGroup>true</isSupportGroup>
    <isSupportShiftSearch>true</isSupportShiftSearch>
    <isSupportShift>true</isSupportShift>
    <isSupportHolidayPlanSearch>true</isSupportHolidayPlanSearch>
    <isSupportHolidayPlan>true</isSupportHolidayPlan>
    <isSupportHolidayGroupSearch>true</isSupportHolidayGroupSearch>
    <isSupportHolidayGroup>true</isSupportHolidayGroup>
    <isSupportWeekPlanSearch>true</isSupportWeekPlanSearch>
    <isSupportWeekPlan>true</isSupportWeekPlan>
    <isSupportPlanTemplateSearch>true</isSupportPlanTemplateSearch>
    <isSupportPlanTemplate>true</isSupportPlanTemplate>
    <isSupportGroupShiftSearch>true</isSupportGroupShiftSearch>
    <isSupportGroupShift>true</isSupportGroupShift>
    <isSupportPersonalShiftNum>true</isSupportPersonalShiftNum>
    <isSupportLocalAttendanceRule>true</isSupportLocalAttendanceRule>
  </LocalAttendanceCap>
</AccessControl>
```

## Confirmed By Follow-Up Calls

- User search and user count work.
- Card count and card search work, but there are zero cards.
- Access-control event search and total count work.
- Work status, ACS config, card-reader config, face-recognition mode, event optimization, event storage, attendance key config, attendance mode, verification plans, attendance week plan, attendance templates, local attendance rule, and local attendance searches work.
- Raw face/fingerprint search endpoints tested returned `notSupport`.
- Some capability flags are true, but common endpoint names still returned `404` or empty payloads. See [tested unsupported endpoints](tested-unsupported-endpoints.md).
