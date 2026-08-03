package actives

// Day3QuestionsData is compiled-in Day 3 quiz questions.
var Day3QuestionsData = map[string]*QuizQuestionRecord{
	"D3-001": &QuizQuestionRecord{
		ID: "D3-001", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
		Question: "What is the main goal of ComputeFlux?", Options: []string{"Provide decentralized AI compute access", "Sell phone hardware", "Operate traditional banks", "Develop video game consoles"},
		CorrectAnswer: 0, Explanation: "ComputeFlux's core direction is to connect AI developers, AI Agents, and distributed GPU compute resources.",
	},
	"D3-002": &QuizQuestionRecord{
		ID: "D3-002", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
		Question: "What compatible interface does ComputeFlux plan to provide?", Options: []string{"OpenAI-compatible API", "FTP-only API", "Email-only interface", "Bluetooth API"},
		CorrectAnswer: 0, Explanation: "The OpenAI-compatible API reduces the cost for developers to migrate and integrate different models.",
	},
	"D3-003": &QuizQuestionRecord{
		ID: "D3-003", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
		Question: "What does AI inference usually refer to?", Options: []string{"Using a trained model to generate results", "Buying graphics cards", "Deleting training data", "Designing web page colors"},
		CorrectAnswer: 0, Explanation: "Inference is the process of generating predictions or content based on input after model training is complete.",
	},
	"D3-004": &QuizQuestionRecord{
		ID: "D3-004", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
		Question: "Why are GPUs commonly used for AI computing?", Options: []string{"They excel at parallel computing", "They can only store text", "They are only for playing music", "They cannot process matrix operations"},
		CorrectAnswer: 0, Explanation: "GPUs can efficiently perform large-scale parallel matrix operations, making them suitable for deep learning training and inference.",
	},
	"D3-005": &QuizQuestionRecord{
		ID: "D3-005", Day: 3, Difficulty: "beginner", Category: "Large Language Models",
		Question: "What does LLM stand for?", Options: []string{"Large Language Model", "Local Logic Machine", "Long Latency Memory", "Linear Learning Map"},
		CorrectAnswer: 0, Explanation: "LLM stands for Large Language Model.",
	},
	"D3-006": &QuizQuestionRecord{
		ID: "D3-006", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
		Question: "What is the main difference between model training and model inference?", Options: []string{"Training learns parameters, inference generates results", "They are exactly the same", "Inference only happens in databases", "Training does not require data"},
		CorrectAnswer: 0, Explanation: "The training phase optimizes model parameters; the inference phase uses the trained model to process new inputs.",
	},
	"D3-007": &QuizQuestionRecord{
		ID: "D3-007", Day: 3, Difficulty: "beginner", Category: "APIs",
		Question: "What is the main purpose of an API?", Options: []string{"Enable different software systems to communicate with each other", "Increase monitor brightness", "Charge the computer", "Replace all databases"},
		CorrectAnswer: 0, Explanation: "API defines how software systems invoke functions and exchange data with each other.",
	},
	"D3-008": &QuizQuestionRecord{
		ID: "D3-008", Day: 3, Difficulty: "beginner", Category: "AI Agents",
		Question: "What is an AI Agent usually?", Options: []string{"A software system that can perceive, make decisions, and execute tasks", "A graphics card brand", "A database format", "A cryptocurrency wallet"},
		CorrectAnswer: 0, Explanation: "An AI Agent typically receives goals, invokes tools, makes decisions, and executes tasks.",
	},
	"D3-009": &QuizQuestionRecord{
		ID: "D3-009", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
		Question: "Which participants does ComputeFlux aim to connect?", Options: []string{"AI developers, AI Agents, and GPU providers", "Only gamers", "Only banks", "Only mobile users"},
		CorrectAnswer: 0, Explanation: "One of ComputeFlux's goals is to connect compute demanders with compute providers.",
	},
	"D3-010": &QuizQuestionRecord{
		ID: "D3-010", Day: 3, Difficulty: "beginner", Category: "Decentralized Compute",
		Question: "What is a potential advantage of a decentralized compute network?", Options: []string{"Reduce reliance on a single cloud provider", "Guarantee zero cost for all tasks", "Completely eliminate the need for network", "Never have any failures"},
		CorrectAnswer: 0, Explanation: "Distributed supply can reduce reliance on single centralized infrastructure.",
	},
	"D3-011": &QuizQuestionRecord{
		ID: "D3-011", Day: 3, Difficulty: "beginner", Category: "AI Safety",
		Question: "What does AI hallucination refer to?", Options: []string{"The model generates plausible but inaccurate information", "GPU temperature is too low", "Web pages load too fast", "Passwords are encrypted"},
		CorrectAnswer: 0, Explanation: "AI hallucination is when the model outputs information that is false or not supported by evidence.",
	},
	"D3-012": &QuizQuestionRecord{
		ID: "D3-012", Day: 3, Difficulty: "beginner", Category: "Privacy",
		Question: "Why is privacy important for AI applications?", Options: []string{"User input may contain sensitive data", "Because models cannot process text", "Because GPUs don't support networking", "Because all data is public"},
		CorrectAnswer: 0, Explanation: "Prompts, documents, and business data may contain sensitive or proprietary information.",
	},
	"D3-013": &QuizQuestionRecord{
		ID: "D3-013", Day: 3, Difficulty: "beginner", Category: "TEE",
		Question: "What is the main purpose of TEE?", Options: []string{"Execute code and process data in a protected environment", "Improve screen resolution", "Compress video files", "Replace internet connection"},
		CorrectAnswer: 0, Explanation: "Trusted Execution Environment helps isolate sensitive code and data, reducing the risk of external tampering or eavesdropping.",
	},
	"D3-014": &QuizQuestionRecord{
		ID: "D3-014", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
		Question: "What does a token usually refer to in language models?", Options: []string{"The basic unit used by the model when processing text", "A graphics card", "An operating system", "A database table"},
		CorrectAnswer: 0, Explanation: "Language models split text into tokens before processing.",
	},
	"D3-015": &QuizQuestionRecord{
		ID: "D3-015", Day: 3, Difficulty: "beginner", Category: "APIs",
		Question: "What is one advantage of an OpenAI-compatible interface?", Options: []string{"Developers can reuse existing tools and code", "All model outputs are identical", "No authentication required", "Automatically eliminate all costs"},
		CorrectAnswer: 0, Explanation: "Compatible interfaces make it easier to migrate existing SDKs, tools, and code to new model services.",
	},
	"D3-016": &QuizQuestionRecord{
		ID: "D3-016", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Product",
		Question: "What problem does ComputeFlux's unified gateway primarily help solve?", Options: []string{"Connect different models and compute sources", "Replace all operating systems", "Manufacture physical graphics cards", "Manage home appliances"},
		CorrectAnswer: 0, Explanation: "The unified gateway allows applications to access different AI models and compute resources through a consistent interface.",
	},
	"D3-017": &QuizQuestionRecord{
		ID: "D3-017", Day: 3, Difficulty: "beginner", Category: "AI Fundamentals",
		Question: "Which of the following is most likely to affect AI inference speed?", Options: []string{"Model size and available compute", "Web page background color", "Keyboard brand", "File name length"},
		CorrectAnswer: 0, Explanation: "Model size, hardware capability, batch size, and input length all affect inference speed.",
	},
	"D3-018": &QuizQuestionRecord{
		ID: "D3-018", Day: 3, Difficulty: "beginner", Category: "Responsible AI",
		Question: "What is a good practice for using AI responsibly?", Options: []string{"Verify important outputs and protect sensitive data", "Unconditionally trust all answers", "Make all private data public", "Hide model limitations"},
		CorrectAnswer: 0, Explanation: "In important scenarios, outputs should be verified, privacy protected, and limitations disclosed to users.",
	},
	"D3-019": &QuizQuestionRecord{
		ID: "D3-019", Day: 3, Difficulty: "beginner", Category: "ComputeFlux Basics",
		Question: "What is the potential role of GPU providers in the ComputeFlux network?", Options: []string{"Contribute available compute resources", "Publish weather forecasts", "Manufacture all AI models", "Manage user mailboxes"},
		CorrectAnswer: 0, Explanation: "GPU providers can contribute compute resources to the network for developers and AI Agents.",
	},
	"D3-020": &QuizQuestionRecord{
		ID: "D3-020", Day: 3, Difficulty: "beginner", Category: "Decentralized Compute",
		Question: "What is a key problem that a distributed compute market needs to solve?", Options: []string{"Trustworthiness of task results and resource quality", "Make all users use the same password", "Cancel all logs", "Ban model updates"},
		CorrectAnswer: 0, Explanation: "In a distributed environment, providers, execution processes, results, and service quality need to be verified.",
	},
}

// Day7QuestionsData is compiled-in Day 7 quiz questions.
var Day7QuestionsData = map[string]*QuizQuestionRecord{
	"D7-001": &QuizQuestionRecord{
		ID: "D7-001", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Architecture",
		Question: "Why does ComputeFlux adopt a unified AI gateway design?", Options: []string{"Allow applications to access different models and compute through a consistent interface", "Ensure every model gives the same answer", "Completely eliminate authentication", "Only allow a single model"},
		CorrectAnswer: 0, Explanation: "The unified gateway reduces the complexity of integrating multiple models and providers.",
	},
	"D7-002": &QuizQuestionRecord{
		ID: "D7-002", Day: 7, Difficulty: "intermediate", Category: "TEE",
		Question: "What is the main value of TEE attestation?", Options: []string{"Provide evidence of execution environment identity and integrity", "Increase monitor refresh rate", "Compress model weights", "Replace API keys"},
		CorrectAnswer: 0, Explanation: "Remote attestation helps verify that code is running in the expected protected environment.",
	},
	"D7-003": &QuizQuestionRecord{
		ID: "D7-003", Day: 7, Difficulty: "intermediate", Category: "Decentralized Compute",
		Question: "Why does a distributed GPU network need a scheduling mechanism?", Options: []string{"Match tasks to suitable hardware and manage capacity", "Shut down all GPUs simultaneously", "Delete user requests", "Fix all model outputs"},
		CorrectAnswer: 0, Explanation: "Scheduling needs to consider model requirements, VRAM, location, price, latency, and availability.",
	},
	"D7-004": &QuizQuestionRecord{
		ID: "D7-004", Day: 7, Difficulty: "intermediate", Category: "AI Agents",
		Question: "When an AI Agent uses tool calls, what is one of the most important security measures?", Options: []string{"Restrict permissions and verify high-risk operations", "Give the Agent all system privileges", "Disable all logs", "Permanently save all passwords"},
		CorrectAnswer: 0, Explanation: "Least privilege, approval mechanisms, and operation auditing can reduce the risk of Agent misoperation.",
	},
	"D7-005": &QuizQuestionRecord{
		ID: "D7-005", Day: 7, Difficulty: "intermediate", Category: "APIs",
		Question: "What is the most common value of an OpenAI-compatible API?", Options: []string{"Reduce the effort of migrating existing applications to new model services", "Make all APIs free", "Guarantee zero latency", "Eliminate server-side computation"},
		CorrectAnswer: 0, Explanation: "Compatibility with common request and response formats improves the reusability of development tools.",
	},
	"D7-006": &QuizQuestionRecord{
		ID: "D7-006", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Product",
		Question: "Why might ComputeFlux connect local, cloud, and distributed model resources simultaneously?", Options: []string{"Route based on cost, privacy, performance, and availability", "Make all models become the same model", "Eliminate all network latency", "Avoid using any GPU"},
		CorrectAnswer: 0, Explanation: "Multi-source routing can select a more suitable execution environment based on task requirements.",
	},
	"D7-007": &QuizQuestionRecord{
		ID: "D7-007", Day: 7, Difficulty: "intermediate", Category: "AI Fundamentals",
		Question: "What is usually the main purpose of quantizing a model?", Options: []string{"Reduce memory and computation requirements", "Increase the amount of training data", "Automatically correct all factual errors", "Turn the model into a database"},
		CorrectAnswer: 0, Explanation: "Quantization reduces VRAM usage by representing weights with lower precision and often improves inference efficiency.",
	},
	"D7-008": &QuizQuestionRecord{
		ID: "D7-008", Day: 7, Difficulty: "intermediate", Category: "GPU Compute",
		Question: "Why does VRAM capacity limit which models can be run?", Options: []string{"Model weights and runtime data need to occupy VRAM", "VRAM only stores web page images", "Models only use hard drives", "VRAM is irrelevant to AI"},
		CorrectAnswer: 0, Explanation: "Weights, KV cache, intermediate activations, etc. all consume GPU VRAM.",
	},
	"D7-009": &QuizQuestionRecord{
		ID: "D7-009", Day: 7, Difficulty: "intermediate", Category: "Privacy",
		Question: "When processing sensitive prompts, which design is more beneficial for privacy?", Options: []string{"Minimize data retention and use a protected execution environment", "Permanently make all prompts public", "Place keys in frontend code", "Disable access control"},
		CorrectAnswer: 0, Explanation: "Data minimization, encryption, access control, and protected execution can reduce the risk of leakage.",
	},
	"D7-010": &QuizQuestionRecord{
		ID: "D7-010", Day: 7, Difficulty: "intermediate", Category: "Verifiable Execution",
		Question: "What question does verifiable execution primarily try to answer?", Options: []string{"Whether a task was executed as declared and produced trustworthy results", "Whether a web page uses dark mode", "Whether users like a certain model", "Whether GPUs are from the same brand"},
		CorrectAnswer: 0, Explanation: "Verification mechanisms focus on execution environment, code, input-output relationships, and result integrity.",
	},
	"D7-011": &QuizQuestionRecord{
		ID: "D7-011", Day: 7, Difficulty: "intermediate", Category: "AI Safety",
		Question: "Why may relying solely on an LLM to evaluate its own answers be insufficient?", Options: []string{"The model may repeat its own biases or errors", "LLMs cannot output text", "Self-evaluation is always more accurate than human judgment", "Evaluation does not need standards"},
		CorrectAnswer: 0, Explanation: "Reliable evaluation usually requires external standards, test sets, rules, or human review.",
	},
	"D7-012": &QuizQuestionRecord{
		ID: "D7-012", Day: 7, Difficulty: "advanced", Category: "Decentralized Compute",
		Question: "In an open compute network, what best reduces the risk of providers forging task completion results?", Options: []string{"Attestation, auditing, and reproducible verification mechanisms", "Only rely on provider self-declaration", "Delete all logs", "Do not record task status"},
		CorrectAnswer: 0, Explanation: "Open networks require independent evidence and verifiable mechanisms, not just trusting a single party's declaration.",
	},
	"D7-013": &QuizQuestionRecord{
		ID: "D7-013", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Architecture",
		Question: "What may the ComputeFlux router base its provider selection on in a multi-model environment?", Options: []string{"Cost, latency, model capability, privacy, and availability", "User avatar color", "Keyboard layout", "File extension"},
		CorrectAnswer: 0, Explanation: "Intelligent routing usually needs to consider performance, price, capacity, compliance, and privacy constraints.",
	},
	"D7-014": &QuizQuestionRecord{
		ID: "D7-014", Day: 7, Difficulty: "intermediate", Category: "APIs",
		Question: "What problem does idempotency in API requests primarily solve?", Options: []string{"Avoid duplicate execution or duplicate charges caused by retries", "Increase screen brightness", "Make models auto-train", "Cancel database transactions"},
		CorrectAnswer: 0, Explanation: "Idempotent design ensures that submitting the same operation multiple times does not produce multiple side effects.",
	},
	"D7-015": &QuizQuestionRecord{
		ID: "D7-015", Day: 7, Difficulty: "intermediate", Category: "AI Fundamentals",
		Question: "What is the role of KV cache in large language model inference?", Options: []string{"Cache attention-related states to accelerate subsequent token generation", "Store user passwords", "Train new models", "Compress web page images"},
		CorrectAnswer: 0, Explanation: "KV cache avoids recomputing all historical attention states when generating each new token.",
	},
	"D7-016": &QuizQuestionRecord{
		ID: "D7-016", Day: 7, Difficulty: "intermediate", Category: "Web3 Payments",
		Question: "What is the potential advantage of using crypto payments for machine-to-machine services?", Options: []string{"Can support programmatic, micro, and cross-region settlements", "Guarantee token price appreciation", "Eliminate all regulatory requirements", "No security measures needed"},
		CorrectAnswer: 0, Explanation: "On-chain or digital asset payments can support automated settlement, but fees, security, and compliance still need to be considered.",
	},
	"D7-017": &QuizQuestionRecord{
		ID: "D7-017", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Product",
		Question: "Why are AI Agents well-suited for using a unified model gateway?", Options: []string{"Can invoke different models per task without integrating each provider individually", "Agents can only invoke one model", "The gateway automatically eliminates all errors", "No more permission control needed"},
		CorrectAnswer: 0, Explanation: "A unified interface makes it easier for Agents to choose text, code, image, or specialized models.",
	},
	"D7-018": &QuizQuestionRecord{
		ID: "D7-018", Day: 7, Difficulty: "advanced", Category: "Privacy",
		Question: "TEE provides protection, but why can it still not be considered absolutely secure?", Options: []string{"Implementation vulnerabilities, side channels, or configuration issues may still exist", "TEE does not run code", "TEE can only be used on phones", "TEE automatically guarantees model correctness"},
		CorrectAnswer: 0, Explanation: "TEE is part of a security architecture and still requires patches, audits, key management, and system-level protection.",
	},
	"D7-019": &QuizQuestionRecord{
		ID: "D7-019", Day: 7, Difficulty: "intermediate", Category: "AI Evaluation",
		Question: "When evaluating AI gateway quality, which set of metrics is most reasonable?", Options: []string{"Latency, success rate, output quality, cost, and availability", "Icon color and website font", "Team size and office area", "File name length"},
		CorrectAnswer: 0, Explanation: "Service quality should combine multi-dimensional metrics such as performance, reliability, quality, and cost.",
	},
	"D7-020": &QuizQuestionRecord{
		ID: "D7-020", Day: 7, Difficulty: "intermediate", Category: "ComputeFlux Architecture",
		Question: "Why does ComputeFlux need quality management for compute providers?", Options: []string{"Different providers may have different performance, stability, and trustworthiness", "All GPUs are always exactly the same", "Models never fail", "The network does not need monitoring"},
		CorrectAnswer: 0, Explanation: "In an open compute network, resources vary greatly, requiring monitoring, scoring, verification, and fault handling.",
	},
}
