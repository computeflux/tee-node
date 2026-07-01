package indexer

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

const GatewayModelIndexName = "gateway_models"

// GatewayModelSearchRequest 定义 gateway 模型索引的查询参数。
type GatewayModelSearchRequest struct {
	Keyword    string
	MinContext *uint64
	MaxContext *uint64
	MinPrice   *uint64
	MaxPrice   *uint64
	Modalities uint8 // inputModalities 位掩码筛选，0 表示不过滤
	Offset     uint64
	Limit      uint32
}

// GatewayModelIndex 为 gateway 合约中的模型建立内存索引，支持名称模糊搜索和数值范围筛选。
type GatewayModelIndex struct {
	mu     sync.RWMutex
	index  bleve.Index
	models map[string]cgateway.ModelInfoWithProvider
}

// NewGatewayModelIndex 创建 gateway 模型索引实例。
func NewGatewayModelIndex() *GatewayModelIndex {
	return &GatewayModelIndex{}
}

// Name 返回索引名称。
func (g *GatewayModelIndex) Name() string {
	return GatewayModelIndexName
}

// Bleve 返回当前底层 Bleve 索引。
func (g *GatewayModelIndex) Bleve() bleve.Index {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.index
}

// Refresh 从链上拉取全部活跃模型并重建索引。
func (g *GatewayModelIndex) Refresh(ctx context.Context, chain *teechain.TeeChain) error {
	models, err := gateway.GatewayListActiveModelsWithinProvider(chain, 0, 0, 0, 0, 0, 100000)
	if err != nil {
		return fmt.Errorf("list active models: %w", err)
	}
	return g.BuildFromModels(models)
}

// BuildFromModels 直接从模型数据重建索引，便于测试和外部批量更新。
func (g *GatewayModelIndex) BuildFromModels(models []cgateway.ModelInfoWithProvider) error {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = g.buildDocumentMapping()

	newIndex, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return fmt.Errorf("create bleve index: %w", err)
	}

	newModels := make(map[string]cgateway.ModelInfoWithProvider, len(models))
	for _, m := range models {
		docID := strconv.FormatUint(m.ID, 10)
		newModels[docID] = m

		doc := map[string]any{
			"name":              m.Model.Name,
			"modelId":           m.Model.ModelID,
			"providerType":      m.Model.ProviderType,
			"contextLength":     m.Model.ContextLength,
			"inPrice":           m.Model.InPrice,
			"cachedPrice":       m.Model.CachedPrice,
			"outPrice":          m.Model.OutPrice,
			"providerID":        m.Model.ProviderID,
			"inputModalityTags": inputModalityTags(m.Model.InputModalities),
		}
		if err := newIndex.Index(docID, doc); err != nil {
			_ = newIndex.Close()
			return fmt.Errorf("index model %s: %w", docID, err)
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.index != nil {
		_ = g.index.Close()
	}
	g.index = newIndex
	g.models = newModels
	return nil
}

// Get 按 ID 查询单个模型。
func (g *GatewayModelIndex) Get(id uint64) (cgateway.ModelInfoWithProvider, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	m, ok := g.models[strconv.FormatUint(id, 10)]
	return m, ok
}

// Search 在索引中查询模型。
func (g *GatewayModelIndex) Search(ctx context.Context, req GatewayModelSearchRequest) ([]cgateway.ModelInfoWithProvider, error) {
	g.mu.RLock()
	index := g.index
	models := g.models
	g.mu.RUnlock()

	if index == nil {
		return nil, fmt.Errorf("gateway model index not ready")
	}

	queries := make([]query.Query, 0, 4)

	if req.Keyword != "" {
		mq := bleve.NewMatchQuery(req.Keyword)
		mq.SetField("name")
		mq.SetFuzziness(1)
		queries = append(queries, mq)
	}

	if req.Modalities != 0 {
		tagQueries := make([]query.Query, 0, 5)
		for _, tag := range inputModalityTags(req.Modalities) {
			mq := bleve.NewMatchQuery(tag)
			mq.SetField("inputModalityTags")
			tagQueries = append(tagQueries, mq)
		}
		if len(tagQueries) > 0 {
			queries = append(queries, bleve.NewDisjunctionQuery(tagQueries...))
		}
	}

	if req.MinContext != nil || req.MaxContext != nil {
		queries = append(queries, numericRangeQuery(req.MinContext, req.MaxContext, "contextLength"))
	}

	if req.MinPrice != nil || req.MaxPrice != nil {
		queries = append(queries, numericRangeQuery(req.MinPrice, req.MaxPrice, "inPrice"))
	}

	var searchQuery query.Query
	if len(queries) == 0 {
		searchQuery = bleve.NewMatchAllQuery()
	} else {
		searchQuery = bleve.NewConjunctionQuery(queries...)
	}

	searchReq := bleve.NewSearchRequest(searchQuery)
	searchReq.From = int(req.Offset)
	searchReq.Size = int(req.Limit)
	searchReq.Fields = []string{"*"}

	res, err := index.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("search index: %w", err)
	}

	out := make([]cgateway.ModelInfoWithProvider, 0, len(res.Hits))
	for _, hit := range res.Hits {
		if m, ok := models[hit.ID]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func (g *GatewayModelIndex) buildDocumentMapping() *mapping.DocumentMapping {
	dm := bleve.NewDocumentMapping()

	nameFM := bleve.NewTextFieldMapping()
	nameFM.Analyzer = "standard"
	dm.AddFieldMappingsAt("name", nameFM)

	modelIDFM := bleve.NewTextFieldMapping()
	modelIDFM.Analyzer = "standard"
	dm.AddFieldMappingsAt("modelId", modelIDFM)

	dm.AddFieldMappingsAt("providerType", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("contextLength", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("inPrice", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("cachedPrice", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("outPrice", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("providerID", bleve.NewNumericFieldMapping())

	tagsFM := bleve.NewTextFieldMapping()
	tagsFM.Analyzer = "standard"
	dm.AddFieldMappingsAt("inputModalityTags", tagsFM)

	return dm
}

func numericRangeQuery(min, max *uint64, field string) query.Query {
	var minF, maxF float64
	if min != nil {
		minF = float64(*min)
	} else {
		minF = 0
	}
	if max != nil {
		maxF = float64(*max)
	} else {
		maxF = float64(math.MaxUint64)
	}
	q := bleve.NewNumericRangeQuery(&minF, &maxF)
	q.SetField(field)
	return q
}

// inputModalityTags 将 inputModalities 位掩码转换为索引/查询用的标签列表。
// 位定义与链上 Gateway 合约一致：0x01 Text, 0x02 Image, 0x04 File, 0x08 Audio, 0x10 Video。
func inputModalityTags(modalities uint8) []string {
	tags := make([]string, 0, 5)
	if modalities&cgateway.ModalityVideo != 0 {
		tags = append(tags, "Video")
	}
	if modalities&cgateway.ModalityAudio != 0 {
		tags = append(tags, "Audio")
	}
	if modalities&cgateway.ModalityFile != 0 {
		tags = append(tags, "File")
	}
	if modalities&cgateway.ModalityImage != 0 {
		tags = append(tags, "Image")
	}
	if modalities&cgateway.ModalityText != 0 {
		tags = append(tags, "Text")
	}
	return tags
}
