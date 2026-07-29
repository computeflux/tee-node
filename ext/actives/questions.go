package actives

// Day3QuestionsData is compiled-in Day 3 quiz questions.
var Day3QuestionsData = map[string]*QuizQuestionRecord{
    "D3-001": &QuizQuestionRecord{
        ID: "D3-001", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
        Question: "ComputeFlux 的主要目标是什么？", Options: []string{"提供去中心化 AI 算力访问", "销售手机硬件", "运营传统银行", "开发电子游戏主机"},
        CorrectAnswer: 0, Explanation: "ComputeFlux 的核心方向是连接 AI 开发者、AI Agent 与分布式 GPU 算力资源。",
    },
    "D3-002": &QuizQuestionRecord{
        ID: "D3-002", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
        Question: "ComputeFlux 计划提供哪种兼容接口？", Options: []string{"OpenAI-compatible API", "FTP-only API", "Email-only interface", "Bluetooth API"},
        CorrectAnswer: 0, Explanation: "OpenAI-compatible API 可以降低开发者迁移和集成不同模型的成本。",
    },
    "D3-003": &QuizQuestionRecord{
        ID: "D3-003", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
        Question: "AI 推理（inference）通常指什么？", Options: []string{"使用训练好的模型生成结果", "购买显卡", "删除训练数据", "设计网页颜色"},
        CorrectAnswer: 0, Explanation: "推理是模型训练完成后，根据输入生成预测或内容的过程。",
    },
    "D3-004": &QuizQuestionRecord{
        ID: "D3-004", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
        Question: "GPU 为什么常用于 AI 计算？", Options: []string{"擅长并行计算", "只能存储文本", "只用于播放音乐", "不能处理矩阵运算"},
        CorrectAnswer: 0, Explanation: "GPU 能高效执行大量并行矩阵运算，因此适合深度学习训练和推理。",
    },
    "D3-005": &QuizQuestionRecord{
        ID: "D3-005", Day: 3, Difficulty: "beginner", Category: "Large Language Models",
        Question: "LLM 是什么的缩写？", Options: []string{"Large Language Model", "Local Logic Machine", "Long Latency Memory", "Linear Learning Map"},
        CorrectAnswer: 0, Explanation: "LLM 指 Large Language Model，即大型语言模型。",
    },
    "D3-006": &QuizQuestionRecord{
        ID: "D3-006", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
        Question: "模型训练和模型推理的主要区别是什么？", Options: []string{"训练用于学习参数，推理用于生成结果", "两者完全相同", "推理只发生在数据库中", "训练不需要数据"},
        CorrectAnswer: 0, Explanation: "训练阶段优化模型参数；推理阶段使用已训练模型处理新输入。",
    },
    "D3-007": &QuizQuestionRecord{
        ID: "D3-007", Day: 3, Difficulty: "beginner", Category: "APIs",
        Question: "API 的主要作用是什么？", Options: []string{"让不同软件系统相互通信", "增加显示器亮度", "给电脑充电", "替代所有数据库"},
        CorrectAnswer: 0, Explanation: "API 定义了软件之间调用功能和交换数据的方式。",
    },
    "D3-008": &QuizQuestionRecord{
        ID: "D3-008", Day: 3, Difficulty: "beginner", Category: "AI Agents",
        Question: "AI Agent 通常是什么？", Options: []string{"能够感知、决策并执行任务的软件系统", "一种显卡品牌", "一种数据库格式", "一种加密货币钱包"},
        CorrectAnswer: 0, Explanation: "AI Agent 通常会接收目标、调用工具、做出决策并执行任务。",
    },
    "D3-009": &QuizQuestionRecord{
        ID: "D3-009", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
        Question: "ComputeFlux 想连接哪些参与者？", Options: []string{"AI 开发者、AI Agent 与 GPU 提供者", "只有游戏玩家", "只有银行", "只有手机用户"},
        CorrectAnswer: 0, Explanation: "ComputeFlux 的定位之一是连接算力需求方和算力提供方。",
    },
    "D3-010": &QuizQuestionRecord{
        ID: "D3-010", Day: 3, Difficulty: "beginner", Category: "Decentralized Compute",
        Question: "去中心化算力网络的一个潜在优势是什么？", Options: []string{"减少对单一云服务商的依赖", "保证所有任务零成本", "完全不需要网络", "永远不会出现故障"},
        CorrectAnswer: 0, Explanation: "分布式供给可以降低对单一中心化基础设施的依赖。",
    },
    "D3-011": &QuizQuestionRecord{
        ID: "D3-011", Day: 3, Difficulty: "beginner", Category: "AI Safety",
        Question: "AI 幻觉（hallucination）指什么？", Options: []string{"模型生成看似合理但不准确的信息", "GPU 温度过低", "网页加载太快", "密码被加密"},
        CorrectAnswer: 0, Explanation: "AI 幻觉是模型输出不真实或无法由证据支持的信息。",
    },
    "D3-012": &QuizQuestionRecord{
        ID: "D3-012", Day: 3, Difficulty: "beginner", Category: "Privacy",
        Question: "为什么 AI 应用需要重视隐私？", Options: []string{"用户输入可能包含敏感数据", "因为模型不能处理文本", "因为 GPU 不支持网络", "因为所有数据都公开"},
        CorrectAnswer: 0, Explanation: "提示词、文档和业务数据可能包含敏感或专有信息。",
    },
    "D3-013": &QuizQuestionRecord{
        ID: "D3-013", Day: 3, Difficulty: "beginner", Category: "TEE",
        Question: "TEE 的主要用途是什么？", Options: []string{"在受保护环境中执行代码和处理数据", "提升屏幕分辨率", "压缩视频文件", "替代互联网连接"},
        CorrectAnswer: 0, Explanation: "可信执行环境可帮助隔离敏感代码和数据，降低被外部篡改或窥探的风险。",
    },
    "D3-014": &QuizQuestionRecord{
        ID: "D3-014", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
        Question: "Token 在语言模型中通常指什么？", Options: []string{"模型处理文本时使用的基本单位", "一种显卡", "一种操作系统", "一种数据库表"},
        CorrectAnswer: 0, Explanation: "语言模型会把文本拆分成 token 后进行处理。",
    },
    "D3-015": &QuizQuestionRecord{
        ID: "D3-015", Day: 3, Difficulty: "beginner", Category: "APIs",
        Question: "OpenAI-compatible 接口的一个优势是什么？", Options: []string{"开发者可复用现有工具和代码", "所有模型输出完全相同", "无需身份验证", "自动消除所有费用"},
        CorrectAnswer: 0, Explanation: "兼容接口可让现有 SDK、工具和代码更容易迁移到新的模型服务。",
    },
    "D3-016": &QuizQuestionRecord{
        ID: "D3-016", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Product",
        Question: "ComputeFlux 的统一网关主要帮助解决什么问题？", Options: []string{"连接不同模型与算力来源", "替代所有操作系统", "生产实体显卡", "管理家庭电器"},
        CorrectAnswer: 0, Explanation: "统一网关让应用可以通过一致接口访问不同 AI 模型和算力。",
    },
    "D3-017": &QuizQuestionRecord{
        ID: "D3-017", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
        Question: "下列哪项最可能影响 AI 推理速度？", Options: []string{"模型大小和可用算力", "网页背景颜色", "键盘品牌", "文件名长度"},
        CorrectAnswer: 0, Explanation: "模型规模、硬件能力、批量大小和输入长度都会影响推理速度。",
    },
    "D3-018": &QuizQuestionRecord{
        ID: "D3-018", Day: 3, Difficulty: "beginner", Category: "Responsible AI",
        Question: "负责任地使用 AI 时，较好的做法是什么？", Options: []string{"核实重要输出并保护敏感数据", "无条件相信所有回答", "公开所有私人数据", "隐藏模型限制"},
        CorrectAnswer: 0, Explanation: "重要场景下应验证输出、保护隐私并向用户说明限制。",
    },
    "D3-019": &QuizQuestionRecord{
        ID: "D3-019", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
        Question: "GPU 提供者在 ComputeFlux 网络中的潜在角色是什么？", Options: []string{"贡献可用算力资源", "发布天气预报", "制造所有 AI 模型", "管理用户邮箱"},
        CorrectAnswer: 0, Explanation: "GPU 提供者可以向网络贡献计算资源，供开发者和 AI Agent 使用。",
    },
    "D3-020": &QuizQuestionRecord{
        ID: "D3-020", Day: 3, Difficulty: "beginner", Category: "Decentralized Compute",
        Question: "分布式算力市场需要解决的一个关键问题是什么？", Options: []string{"任务结果的可信性和资源质量", "让所有用户使用同一密码", "取消所有日志", "禁止模型更新"},
        CorrectAnswer: 0, Explanation: "分布式环境下需要验证提供者、执行过程、结果和服务质量。",
    },
}

// Day7QuestionsData is compiled-in Day 7 quiz questions.
var Day7QuestionsData = map[string]*QuizQuestionRecord{
    "D7-001": &QuizQuestionRecord{
        ID: "D7-001", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Architecture",
        Question: "为什么 ComputeFlux 采用统一 AI 网关设计？", Options: []string{"让应用通过一致接口访问不同模型和算力", "保证每个模型回答一致", "完全取消身份验证", "只允许单一模型"},
        CorrectAnswer: 0, Explanation: "统一网关降低了多模型、多提供者接入的复杂度。",
    },
    "D7-002": &QuizQuestionRecord{
        ID: "D7-002", Day: 7, Difficulty: "intermediate", Category: "TEE",
        Question: "TEE attestation 的主要价值是什么？", Options: []string{"提供执行环境身份和完整性的证据", "提高显示器刷新率", "压缩模型权重", "替代 API 密钥"},
        CorrectAnswer: 0, Explanation: "远程证明可帮助验证代码是否运行在预期的受保护环境中。",
    },
    "D7-003": &QuizQuestionRecord{
        ID: "D7-003", Day: 7, Difficulty: "intermediate", Category: "Decentralized Compute",
        Question: "分布式 GPU 网络为什么需要调度机制？", Options: []string{"把任务匹配给合适的硬件并管理容量", "让所有 GPU 同时关机", "删除用户请求", "固定所有模型输出"},
        CorrectAnswer: 0, Explanation: "调度需要综合模型需求、显存、位置、价格、延迟和可用性。",
    },
    "D7-004": &QuizQuestionRecord{
        ID: "D7-004", Day: 7, Difficulty: "intermediate", Category: "AI Agents",
        Question: "AI Agent 使用工具调用时，最重要的安全措施之一是什么？", Options: []string{"限制权限并验证高风险操作", "让 Agent 获得所有系统权限", "禁用所有日志", "永久保存所有密码"},
        CorrectAnswer: 0, Explanation: "最小权限、审批机制和操作审计能减少 Agent 误操作风险。",
    },
    "D7-005": &QuizQuestionRecord{
        ID: "D7-005", Day: 7, Difficulty: "intermediate", Category: "APIs",
        Question: "OpenAI-compatible API 最常见的价值是什么？", Options: []string{"减少现有应用迁移到新模型服务的工作量", "让所有 API 免费", "保证零延迟", "取消服务端计算"},
        CorrectAnswer: 0, Explanation: "兼容常见请求和响应格式可提升开发工具的可复用性。",
    },
    "D7-006": &QuizQuestionRecord{
        ID: "D7-006", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Product",
        Question: "ComputeFlux 为什么可能同时连接本地、云端和分布式模型资源？", Options: []string{"在成本、隐私、性能和可用性之间进行路由", "让所有模型变成同一个模型", "消除所有网络延迟", "避免使用任何 GPU"},
        CorrectAnswer: 0, Explanation: "多来源路由可以根据任务需求选择更适合的执行环境。",
    },
    "D7-007": &QuizQuestionRecord{
        ID: "D7-007", Day: 7, Difficulty: "intermediate", Category: "AI Fundamentals",
        Question: "量化模型的主要目的通常是什么？", Options: []string{"降低内存和计算需求", "增加训练数据数量", "自动纠正所有事实错误", "把模型变成数据库"},
        CorrectAnswer: 0, Explanation: "量化通过使用更低精度表示权重，减少显存占用并常常提升推理效率。",
    },
    "D7-008": &QuizQuestionRecord{
        ID: "D7-008", Day: 7, Difficulty: "intermediate", Category: "GPU Compute",
        Question: "为什么显存容量会限制可运行的模型？", Options: []string{"模型权重和运行时数据需要占用显存", "显存只存储网页图片", "模型只使用硬盘", "显存与 AI 无关"},
        CorrectAnswer: 0, Explanation: "权重、KV cache、中间激活等都会消耗 GPU 显存。",
    },
    "D7-009": &QuizQuestionRecord{
        ID: "D7-009", Day: 7, Difficulty: "intermediate", Category: "Privacy",
        Question: "在处理敏感提示词时，哪种设计更有利于隐私？", Options: []string{"尽量减少数据保留并采用受保护执行环境", "永久公开所有提示词", "把密钥放在前端代码中", "关闭访问控制"},
        CorrectAnswer: 0, Explanation: "数据最小化、加密、访问控制和受保护执行可降低泄露风险。",
    },
    "D7-010": &QuizQuestionRecord{
        ID: "D7-010", Day: 7, Difficulty: "intermediate", Category: "Verifiable Execution",
        Question: "可验证执行主要试图回答什么问题？", Options: []string{"任务是否按声明的方式执行并产生可信结果", "网页是否使用深色模式", "用户是否喜欢某个模型", "GPU 是否来自同一品牌"},
        CorrectAnswer: 0, Explanation: "验证机制关注执行环境、代码、输入输出关系和结果完整性。",
    },
    "D7-011": &QuizQuestionRecord{
        ID: "D7-011", Day: 7, Difficulty: "intermediate", Category: "AI Safety",
        Question: "为什么仅依赖 LLM 自己评价自己的答案可能不够？", Options: []string{"模型可能重复自身偏差或错误", "LLM 无法输出文本", "自评总是比人工更准确", "评估不需要标准"},
        CorrectAnswer: 0, Explanation: "可靠评估通常需要外部标准、测试集、规则或人工审核。",
    },
    "D7-012": &QuizQuestionRecord{
        ID: "D7-012", Day: 7, Difficulty: "advanced", Category: "Decentralized Compute",
        Question: "在开放算力网络中，什么最能减少提供者伪造任务完成结果的风险？", Options: []string{"证明、审计和可重复验证机制", "只依赖提供者自我声明", "删除所有日志", "不记录任务状态"},
        CorrectAnswer: 0, Explanation: "开放网络需要独立证据和可验证机制，而不能只信任单方声明。",
    },
    "D7-013": &QuizQuestionRecord{
        ID: "D7-013", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Architecture",
        Question: "ComputeFlux 路由器在多模型环境中可能依据什么选择提供者？", Options: []string{"成本、延迟、模型能力、隐私和可用性", "用户头像颜色", "键盘布局", "文件扩展名"},
        CorrectAnswer: 0, Explanation: "智能路由通常需要综合性能、价格、容量、合规和隐私约束。",
    },
    "D7-014": &QuizQuestionRecord{
        ID: "D7-014", Day: 7, Difficulty: "intermediate", Category: "APIs",
        Question: "API 请求中的 idempotency 主要解决什么问题？", Options: []string{"避免重试导致重复执行或重复扣费", "提高屏幕亮度", "让模型自动训练", "取消数据库事务"},
        CorrectAnswer: 0, Explanation: "幂等设计确保重复提交相同操作时不会产生多次副作用。",
    },
    "D7-015": &QuizQuestionRecord{
        ID: "D7-015", Day: 7, Difficulty: "intermediate", Category: "AI Fundamentals",
        Question: "KV cache 在大语言模型推理中的作用是什么？", Options: []string{"缓存注意力相关状态以加速后续 token 生成", "存储用户密码", "训练新模型", "压缩网页图片"},
        CorrectAnswer: 0, Explanation: "KV cache 避免在生成每个新 token 时重复计算全部历史注意力状态。",
    },
    "D7-016": &QuizQuestionRecord{
        ID: "D7-016", Day: 7, Difficulty: "intermediate", Category: "Web3 Payments",
        Question: "加密支付用于机器到机器服务时的潜在优势是什么？", Options: []string{"可以支持程序化、小额和跨地域结算", "保证代币价格上涨", "消除所有监管要求", "不需要任何安全措施"},
        CorrectAnswer: 0, Explanation: "链上或数字资产支付可支持自动化结算，但仍需要考虑费用、安全和合规。",
    },
    "D7-017": &QuizQuestionRecord{
        ID: "D7-017", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Product",
        Question: "AI Agent 为什么适合使用统一模型网关？", Options: []string{"可以按任务调用不同模型而不必分别集成每个提供者", "Agent 只能调用一个模型", "网关会自动消除所有错误", "不再需要权限控制"},
        CorrectAnswer: 0, Explanation: "统一接口能让 Agent 更方便地选择文本、代码、图像或专用模型。",
    },
    "D7-018": &QuizQuestionRecord{
        ID: "D7-018", Day: 7, Difficulty: "advanced", Category: "Privacy",
        Question: "TEE 能提供保护，但为什么仍不能被视为绝对安全？", Options: []string{"仍可能存在实现漏洞、侧信道或配置问题", "TEE 不运行代码", "TEE 只能用于手机", "TEE 自动保证模型正确"},
        CorrectAnswer: 0, Explanation: "TEE 是安全架构的一部分，仍需补丁、审计、密钥管理和系统级防护。",
    },
    "D7-019": &QuizQuestionRecord{
        ID: "D7-019", Day: 7, Difficulty: "intermediate", Category: "AI Evaluation",
        Question: "评估 AI 网关质量时，哪组指标最合理？", Options: []string{"延迟、成功率、输出质量、成本和可用性", "图标颜色和网站字体", "团队人数和办公室面积", "文件名长度"},
        CorrectAnswer: 0, Explanation: "服务质量应结合性能、可靠性、质量和成本等多维指标。",
    },
    "D7-020": &QuizQuestionRecord{
        ID: "D7-020", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Architecture",
        Question: "为什么 ComputeFlux 需要对算力提供者进行质量管理？", Options: []string{"不同提供者的性能、稳定性和可信度可能不同", "所有 GPU 永远完全相同", "模型不会失败", "网络不需要监控"},
        CorrectAnswer: 0, Explanation: "开放算力网络中资源差异较大，需要监控、评分、验证和故障处理。",
    },
}
