// IncusOS external REST API
//
// This is the REST API used to manage and monitor IncusOS systems.
//
// The API is available over a local unix socket as well as over HTTPS
// through the primary application's API proxy (under the "/os" prefix)
// or through the fallback listener. Remote users authenticate with TLS
// client certificates.
//
//	Version: 1.0
//	License: Apache-2.0 https://www.apache.org/licenses/LICENSE-2.0
//	Contact: IncusOS upstream <lxc-devel@lists.linuxcontainers.org> https://github.com/lxc/incus-os
//
// swagger:meta
package main
