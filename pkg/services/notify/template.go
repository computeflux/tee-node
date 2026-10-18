package notify

import "fmt"

// 模板化通知文案（LLM 不可用时的降级方案）。
// 按事件类型预置，变量用 fmt 填充；全链路无 LLM 依赖。

// TemplateFor 根据事件类型返回预置的 (title, content)。
// 未知类型回退到通用系统通知模板。
func TemplateFor(ev Event) (title, content string) {
	switch ev.Type {
	case EventModelDeactivated:
		return templateModelDeactivated(ev)
	case EventModelActivated:
		return templateModelActivated(ev)
	case EventModelRegistered:
		return templateModelRegistered(ev)
	case EventSystem:
		return templateSystem(ev)
	default:
		return templateSystem(ev)
	}
}

func modelRef(ev Event) string {
	if ev.ModelName != "" {
		if ev.ProviderName != "" {
			return fmt.Sprintf("%s（%s）", ev.ModelName, ev.ProviderName)
		}
		return ev.ModelName
	}
	if ev.ProviderName != "" {
		return ev.ProviderName
	}
	return "该模型"
}

func templateModelDeactivated(ev Event) (string, string) {
	title := "模型已下架"
	content := fmt.Sprintf("您关注的模型 %s 已下架，暂时无法调用。我们已为您保留配置，模型恢复后将自动通知。", modelRef(ev))
	if r := ev.Extra["reason"]; r != "" {
		content += " 下架原因：" + r + "。"
	}
	return title, content
}

func templateModelActivated(ev Event) (string, string) {
	title := "模型已恢复"
	content := fmt.Sprintf("好消息！模型 %s 已恢复上架，现在可以正常调用。", modelRef(ev))
	return title, content
}

func templateModelRegistered(ev Event) (string, string) {
	title := "新模型上线"
	content := fmt.Sprintf("新模型 %s 已上线，欢迎体验。", modelRef(ev))
	return title, content
}

func templateSystem(ev Event) (string, string) {
	title := "系统通知"
	if m := ev.Extra["message"]; m != "" {
		return title, m
	}
	return title, "ComputeFlux 系统通知。"
}

// defaultTitle 为 LLM 输出无标题时提供兜底标题。
func defaultTitle(ev Event) string {
	switch ev.Type {
	case EventModelDeactivated:
		return "模型已下架"
	case EventModelActivated:
		return "模型已恢复"
	case EventModelRegistered:
		return "新模型上线"
	default:
		return "系统通知"
	}
}
