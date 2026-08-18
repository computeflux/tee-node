package scheduler

import (
	"encoding/json"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/util"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// ClientTypeName 把合约的 ProviderType 转成调度器用的协议字符串名。
func ClientTypeName(pt uint16) string {
	switch pt {
	case cgateway.OpenAI:
		return "openai"
	case cgateway.Anthropic:
		return "anthropic"
	case cgateway.Gemini:
		return "gemini"
	case cgateway.OpenAIResponsesApi:
		return "openai-responses-api"
	case EdgeProviderType:
		return "personal"
	default:
		return fmt.Sprintf("provider-%d", pt)
	}
}

// BuildEndpointsFromModelInfo 把单个 ModelInfoWithProvider 转成一组 Endpoint（每个 APIKey 一个）。
// modelID 是合约里的 uint64 model id，用于计费。
func BuildEndpointsFromModelInfo(modelID uint64, model *cgateway.ModelInfoV2, provider *cgateway.ProviderInfo) []*Endpoint {
	if model == nil || provider == nil {
		return nil
	}

	providerName := provider.Name
	if providerName == "" {
		providerName = fmt.Sprintf("provider-%x", provider.Owner)
	}

	urls := map[uint16]string{}
	json.Unmarshal([]byte(model.URIS), &urls)

	// 本地模型(个人算力)走反向隧道,不需要 APIKey,单端点即可。
	if model.ModelType == cgateway.EdgeRoute {
		return []*Endpoint{{
			Provider:           ProviderID(providerName),
			KeyID:              fmt.Sprintf("local-model-%d", modelID),
			KeyValue:           "",
			Weight:             1.0,
			Enabled:            model.IsActive,
			Models:             []ModelKey{ModelKey(model.Name)},
			Concurrency:        int(model.MaxConcurrency),
			ModelID:            modelID,
			ContractProviderID: model.ProviderID,
			ModelName:          model.Name,
			ModelUrls:          urls,
		}}
	}

	var endpoints []*Endpoint
	for i, key := range model.APIKeys {
		if key == "" {
			continue
		}
		endpoints = append(endpoints, &Endpoint{
			Provider:           ProviderID(providerName),
			KeyID:              fmt.Sprintf("%d-%d", modelID, i),
			KeyValue:           key,
			Weight:             1.0, // 合约暂无权重，均分
			Enabled:            model.IsActive,
			Models:             []ModelKey{ModelKey(model.Name)},
			Concurrency:        int(model.MaxConcurrency),
			ModelID:            modelID,
			ContractProviderID: model.ProviderID,
			ModelName:          model.Name,
			ModelUrls:          urls,
		})
	}

	if len(endpoints) == 0 && model.ModelType != cgateway.EdgeRoute {
		util.LogWarnf("[scheduler] model %s (provider=%s type=%d) has no usable API keys, endpoint_count=0", model.Name, providerName, model.ModelType)
	}
	return endpoints
}

// BuildModelRoute 从单个模型构建直接调用的 ModelRoute。
func BuildModelRoute(m cgateway.ModelInfoWithProvider) *ModelRoute {
	return &ModelRoute{
		Model:          ModelKey(m.Model.Name),
		MaxConcurrency: int(m.Model.MaxConcurrency),
		Endpoints:      BuildEndpointsFromModelInfo(m.ID, &m.Model, &m.Provider),
	}
}

// rewriteBodyModel 把 JSON body 里的 "model" 字符串字段替换为 modelName。
// 解析失败、不是 JSON 或没有 model 字段时返回原 body，不报错。
func rewriteBodyModel(body []byte, modelName string) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return body, nil
	}

	raw, ok := doc["model"]
	if !ok {
		return body, nil
	}

	var current string
	if err := json.Unmarshal(raw, &current); err != nil {
		return body, nil
	}
	if current == modelName {
		return body, nil
	}

	doc["model"], _ = json.Marshal(modelName)
	return json.Marshal(doc)
}
