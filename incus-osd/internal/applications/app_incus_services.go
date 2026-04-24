package applications

type incusCeph struct {
	common
}

// GetDependencies returns a list of other applications this application depends on.
func (*incusCeph) GetDependencies() []string {
	return []string{"incus"}
}

func (*incusCeph) Name() string {
	return "incus-ceph"
}

type incusLinstor struct {
	common
}

// GetDependencies returns a list of other applications this application depends on.
func (*incusLinstor) GetDependencies() []string {
	return []string{"incus"}
}

func (*incusLinstor) Name() string {
	return "incus-linstor"
}
