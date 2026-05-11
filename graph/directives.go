package graph

// 指令
// 1. AuthCheck
func NewDirectiveRoot() DirectiveRoot {
	return DirectiveRoot{
		AuthCheck: AuthCheck,
	}
}
