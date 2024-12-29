package kernel

func (rt *RequestRuntime) BindJSON(obj any) {
	if err := rt.RequestContext.ShouldBindJSON(obj); err != nil {
		rt.S(rt.MakeErrorf("failed to bind json: %v", err))
	}
}

// Suppress function error output
func (rt *RequestRuntime) S(err error) {
	if err != nil {
	}
}
