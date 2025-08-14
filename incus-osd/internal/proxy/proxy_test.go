package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/proxy"
)

func TestDefaultConfigGeneration(t *testing.T) {
	t.Parallel()

	networkConfig := api.SystemNetworkProxy{}

	yamlConfig := `bind: localhost
port: 3128
check: false
rules:
    - host: '*'
      proxy: direct
`

	content, err := proxy.GenerateKPXConfig(&networkConfig)

	require.NoError(t, err)
	require.YAMLEq(t, string(content), yamlConfig)
}

func TestDefaultServerConfigGeneration(t *testing.T) {
	t.Parallel()

	networkConfig := api.SystemNetworkProxy{
		Servers: map[string]api.SystemNetworkProxyServer{
			"example": {
				Host: "proxy.example.org",
				Auth: "anonymous",
			},
		},
	}

	yamlConfig := `bind: localhost
port: 3128
check: false
proxies:
    example:
        host: proxy.example.org
        ssl: false
        type: anonymous
rules:
    - host: '*'
      proxy: example
`

	content, err := proxy.GenerateKPXConfig(&networkConfig)

	require.NoError(t, err)
	require.YAMLEq(t, string(content), yamlConfig)
}

func TestConfigGeneration(t *testing.T) {
	t.Parallel()

	networkConfig := api.SystemNetworkProxy{
		Servers: map[string]api.SystemNetworkProxyServer{
			"corp-kerberos": {
				Host:     "my-krb-proxy.corp.example.net:3128",
				Auth:     "kerberos",
				Username: "biz",
				Password: "baz",
				Realm:    "corp.example.net",
			},
			"corp-basic": {
				Host:     "my-basic-proxy.corp.example.net:8080",
				UseTLS:   true,
				Auth:     "basic",
				Username: "foo",
				Password: "bar",
			},
			"corp-anonymous": {
				Host: "my-anonymous-proxy.corp.example.net:8000",
				Auth: "anonymous",
			},
			"ipv6-proxy": {
				Host: "[fd42:3cfb:8972:3990:1266:6aff:fe35:ead9]:8000",
				Auth: "anonymous",
			},
			"http-proxy": {
				Host: "http://proxy.example.org",
				Auth: "anonymous",
			},
			"https-proxy": {
				Host: "https://proxy.example.org",
				Auth: "anonymous",
			},
		},
		Rules: []api.SystemNetworkProxyRule{
			{
				Destination: "*.hated-domain.example.net|hated-domain.example.net",
				Target:      "none",
			},
			{
				Destination: "*.my-cluster.example.net|operations-center.example.net",
				Target:      "direct",
			},
			{
				Destination: "*.example.net",
				Target:      "corp-basic",
			},
			{
				Destination: "*.example.org",
				Target:      "corp-anonymous",
			},
			{
				Destination: "*",
				Target:      "corp-kerberos",
			},
		},
	}

	yamlConfig := `bind: localhost
port: 3128
check: false
proxies:
    corp-anonymous:
        host: my-anonymous-proxy.corp.example.net
        port: 8000
        ssl: false
        type: anonymous
    corp-basic:
        host: my-basic-proxy.corp.example.net
        port: 8080
        ssl: true
        type: basic
        credential: corp-basic
    corp-kerberos:
        host: my-krb-proxy.corp.example.net
        port: 3128
        ssl: false
        type: kerberos
        realm: corp.example.net
        credential: corp-kerberos
    ipv6-proxy:
        host: fd42:3cfb:8972:3990:1266:6aff:fe35:ead9
        port: 8000
        ssl: false
        type: anonymous
    http-proxy:
        host: proxy.example.org
        port: 80
        ssl: false
        type: anonymous
    https-proxy:
        host: proxy.example.org
        port: 443
        ssl: true
        type: anonymous
credentials:
    corp-basic:
        login: foo
        password: bar
    corp-kerberos:
        login: biz
        password: baz
rules:
    - host: '*.hated-domain.example.net|hated-domain.example.net'
      proxy: none
    - host: '*.my-cluster.example.net|operations-center.example.net'
      proxy: direct
    - host: '*.example.net'
      proxy: corp-basic
    - host: '*.example.org'
      proxy: corp-anonymous
    - host: '*'
      proxy: corp-kerberos
`

	content, err := proxy.GenerateKPXConfig(&networkConfig)

	require.NoError(t, err)
	require.YAMLEq(t, string(content), yamlConfig)
}

func TestConfigGenerationErrors(t *testing.T) {
	t.Parallel()

	networkConfig := api.SystemNetworkProxy{
		Servers: map[string]api.SystemNetworkProxyServer{
			"direct": {
				Host: "proxy.example.org:3128",
				Auth: "anonymous",
			},
		},
	}

	_, err := proxy.GenerateKPXConfig(&networkConfig)
	require.EqualError(t, err, "cannot use reserved keyword 'direct' for proxy definition")

	networkConfig = api.SystemNetworkProxy{
		Servers: map[string]api.SystemNetworkProxyServer{
			"myproxy": {
				Host:     "proxy.example.org:3128",
				Auth:     "foobar",
				Username: "user",
				Password: "pass",
			},
		},
	}

	_, err = proxy.GenerateKPXConfig(&networkConfig)
	require.EqualError(t, err, "unsupported proxy authentication type foobar")

	networkConfig = api.SystemNetworkProxy{
		Rules: []api.SystemNetworkProxyRule{
			{
				Destination: "*.example.org",
				Target:      "myproxy",
			},
		},
	}

	_, err = proxy.GenerateKPXConfig(&networkConfig)
	require.EqualError(t, err, "no proxy defined for target myproxy")
}
