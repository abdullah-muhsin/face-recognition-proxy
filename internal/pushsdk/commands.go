package pushsdk

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"

	"github.com/itplus/pushsdk-gateway/internal/store"
)

var commandUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type commandRequestResponse struct {
	successResponse
	CommandNum  int                  `json:"commandNum"`
	CommandList []commandRequestItem `json:"commandList"`
}

type commandRequestItem struct {
	UUID       string `json:"UUID"`
	URL        string `json:"URL"`
	DataFormat string `json:"dataFormat"`
	Data       string `json:"data"`
}

type commandResultResponse struct {
	successResponse
	IsPendingCommand bool `json:"isPendingCommand"`
}

type commandResultEnvelope struct {
	CommandNum  *int                 `json:"commandNum"`
	CommandList *[]commandResultItem `json:"commandList"`
}

type commandResultItem struct {
	UUID       string          `json:"UUID"`
	DataFormat json.RawMessage `json:"dataFormat"`
	Data       *string         `json:"data"`
}

// ParseCommandResultBatch accepts exactly the documented PushSDK result
// envelope. The vendor documents one exception: a terminal can omit
// dataFormat in the result of a noData command. That omission is retained as
// such and is never inferred or rewritten as a declared format.
func ParseCommandResultBatch(body []byte) ([]store.ISAPICommandResult, error) {
	var envelope commandResultEnvelope
	if err := decodeExactJSON(body, &envelope); err != nil {
		return nil, badRequest("command result envelope is not valid JSON: %v", err)
	}
	if envelope.CommandNum == nil || envelope.CommandList == nil {
		return nil, badRequest("command result must contain commandNum and commandList")
	}
	if *envelope.CommandNum < 0 || *envelope.CommandNum > 20 {
		return nil, badRequest("commandNum must be between 0 and 20")
	}
	if len(*envelope.CommandList) != *envelope.CommandNum {
		return nil, badRequest("commandNum must equal commandList length")
	}
	seen := make(map[string]struct{}, len(*envelope.CommandList))
	results := make([]store.ISAPICommandResult, 0, len(*envelope.CommandList))
	for _, item := range *envelope.CommandList {
		if !commandUUID.MatchString(item.UUID) {
			return nil, badRequest("command result UUID is invalid")
		}
		if _, found := seen[item.UUID]; found {
			return nil, badRequest("command result contains a duplicate UUID")
		}
		seen[item.UUID] = struct{}{}
		if item.Data == nil {
			return nil, badRequest("command result %s has no data", item.UUID)
		}
		if _, err := base64.StdEncoding.DecodeString(*item.Data); err != nil {
			return nil, badRequest("command result %s data is not base64: %v", item.UUID, err)
		}
		var dataFormat *string
		if item.DataFormat == nil {
			// The vendor's CommandResult model documents that a device may omit
			// dataFormat for noData. Store correlates the omission with the sent
			// noData command; its Base64 response value is retained verbatim.
		} else {
			if bytes.Equal(bytes.TrimSpace(item.DataFormat), []byte("null")) {
				return nil, badRequest("command result %s dataFormat must be a string when present", item.UUID)
			}
			var declaredFormat string
			if err := json.Unmarshal(item.DataFormat, &declaredFormat); err != nil {
				return nil, badRequest("command result %s dataFormat must be a string when present", item.UUID)
			}
			dataFormat = &declaredFormat
			if !supportedDataFormat(*dataFormat) {
				return nil, badRequest("command result %s has an unsupported dataFormat", item.UUID)
			}
			if *dataFormat == "noData" && *item.Data != "" {
				return nil, badRequest("command result %s noData payload must be empty", item.UUID)
			}
			if *dataFormat != "noData" && *item.Data == "" {
				return nil, badRequest("command result %s payload must not be empty", item.UUID)
			}
		}
		results = append(results, store.ISAPICommandResult{
			UUID:       item.UUID,
			DataFormat: dataFormat,
			DataBase64: *item.Data,
		})
	}
	return results, nil
}

func commandRequestItemFor(delivery store.ISAPICommandDelivery) commandRequestItem {
	return commandRequestItem{
		UUID:       delivery.UUID,
		URL:        delivery.Method + " " + delivery.URL,
		DataFormat: delivery.DataFormat,
		Data:       base64.StdEncoding.EncodeToString(delivery.Data),
	}
}
