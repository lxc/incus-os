package applications

type debug struct {
	common
}

func (*debug) Name() string {
	return "debug"
}
