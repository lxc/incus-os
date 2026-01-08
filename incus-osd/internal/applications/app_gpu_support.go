package applications

type gpuSupport struct {
	common
}

func (*gpuSupport) Name() string {
	return "gpu-support"
}
