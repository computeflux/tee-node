package gateway

import "github.com/computeflux/tee-provider/pkg/model"

func InitSeedData() {
	if m, _ := model.GetGatewayModel(0); m != nil {
		InitOptimizations()
	InitAutoScaler()
		return
	}

	// Models
	model.SetGatewayModel(0, &model.GatewayModelInfo{Name: "gpt-3.5-turbo", Owner: "admin", PointsPer1K: 3, IsActive: true})
	model.SetGatewayModel(1, &model.GatewayModelInfo{Name: "gpt-4", Owner: "admin", PointsPer1K: 30, IsActive: true})
	model.SetGatewayModel(2, &model.GatewayModelInfo{Name: "claude-3-sonnet", Owner: "admin", PointsPer1K: 10, IsActive: true})
	model.SetGatewayModel(3, &model.GatewayModelInfo{Name: "gemini-1.5-flash", Owner: "admin", PointsPer1K: 1, IsActive: true})

	// Channel: openai
	model.SetGatewayProvider(0, &model.GatewayProviderInfo{
		ID: 0, Name: "openai", ModelIds: []uint64{0, 1}, Priority: 10, IsActive: true, Owner: "admin",
	})
	// Endpoints for openai channel (multiple keys for load balancing)
	model.SetGatewayEndpoint(0, &model.GatewayEndpoint{ID: 0, ProviderID: 0, URL: "https://api.openai.com", APIKey: "sk-key1", Weight: 1, IsActive: true})
	model.SetGatewayEndpoint(1, &model.GatewayEndpoint{ID: 1, ProviderID: 0, URL: "https://api.openai.com", APIKey: "sk-key2", Weight: 1, IsActive: true})

	// Channel: anthropic
	model.SetGatewayProvider(1, &model.GatewayProviderInfo{
		ID: 1, Name: "anthropic", ModelIds: []uint64{2}, Priority: 10, IsActive: true, Owner: "admin",
	})
	model.SetGatewayEndpoint(2, &model.GatewayEndpoint{ID: 2, ProviderID: 1, URL: "https://api.anthropic.com", APIKey: "sk-ant-key1", Weight: 1, IsActive: true})

	// Channel: gemini
	model.SetGatewayProvider(2, &model.GatewayProviderInfo{
		ID: 2, Name: "gemini", ModelIds: []uint64{3}, Priority: 10, IsActive: true, Owner: "admin",
	})
	model.SetGatewayEndpoint(3, &model.GatewayEndpoint{ID: 3, ProviderID: 2, URL: "https://generativelanguage.googleapis.com", APIKey: "google-key1", Weight: 1, IsActive: true})

	// Token
	model.SetGatewayToken(0, &model.GatewayTokenInfo{ID: 0, Owner: "admin", Key: "demo-token", RateLimit: 60, IsActive: true})

	model.SetNextID("gateway_model", 4)
	model.SetNextID("gateway_provider", 3)
	model.SetNextID("gateway_token", 1)

	InitOptimizations()
	InitAutoScaler()
}
