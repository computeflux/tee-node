package scheduler

import "sync"

// routeIDToName 维护 on-chain route ID → route name 的缓存。
// 由 Reloader 在每次重载时刷新。
// DistributeMiddleware 用它把 "model.RouteID > 0" 的 model 映射到正确的
// scheduler 键(路由名)。
var routeIDToName sync.Map // uint64 → string

// SetRouteNameMapping 在重载时写入一条映射。
func SetRouteNameMapping(routeID uint64, name string) {
	routeIDToName.Store(routeID, name)
}

// LookupRouteNameByID 查 routeID 对应的名字。找不到时返回 "",false。
func LookupRouteNameByID(routeID uint64) (string, bool) {
	v, ok := routeIDToName.Load(routeID)
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, s != ""
}

// ResetRouteMappings 清空所有映射(测试用)。
func ResetRouteMappings() {
	routeIDToName.Range(func(k, _ any) bool {
		routeIDToName.Delete(k)
		return true
	})
}
