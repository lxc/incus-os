package state

import (
	"fmt"
	"net/url"
	"strings"
)

// UpgradeFuncs is a list of functions to apply in order to upgrade the version of a given state.
// Each function consumes a list of strings, each representing one line of input, and returns a
// list of strings representing the upgraded state.
type UpgradeFuncs []func([]string) ([]string, error)

// upgrades is a list of upgrade functions to process old states.
var upgrades = UpgradeFuncs{
	// V1: struct System.Encryption renamed to System.Security, along with renaming of a couple of fields.
	func(lines []string) ([]string, error) {
		for i, line := range lines {
			if strings.HasPrefix(line, "System.Encryption.") {
				lines[i] = strings.Replace(lines[i], "System.Encryption.", "System.Security.", 1)
				lines[i] = strings.Replace(lines[i], "System.Security.Config.RecoveryKeys", "System.Security.Config.EncryptionRecoveryKeys", 1)
				lines[i] = strings.Replace(lines[i], "System.Security.State.RecoveryKeysRetrieved", "System.Security.State.EncryptionRecoveryKeysRetrieved", 1)
			}
		}

		return lines, nil
	},
	// V2: struct Network.Proxy expended to support switch to using kpx for proxying.
	func(lines []string) ([]string, error) {
		currentRuleIndex := 0

		for i, line := range lines {
			if strings.HasPrefix(line, "System.Network.Config.Proxy.HTTPProxy") || strings.HasPrefix(line, "System.Network.Config.Proxy.HTTPSProxy") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("malformed line '%s'", line)
				}

				// Bit of a hack: if parts[1] doesn't begin with http, add it for url.Parse() to work correctly.
				proxyHost := parts[1]
				if !strings.HasPrefix(proxyHost, "http") {
					proxyHost = "http://" + proxyHost
				}

				parsedProxy, err := url.Parse(proxyHost)
				if err != nil {
					return nil, err
				}

				mapKey := strings.ReplaceAll(parsedProxy.Hostname(), ".", "_")
				newHost := parsedProxy.Hostname()
				newAuth := "anonymous"

				if parsedProxy.Port() != "" {
					mapKey += "_" + parsedProxy.Port()
					newHost += ":" + parsedProxy.Port()
				}

				if parsedProxy.User != nil {
					newAuth = "basic"
				}

				lines[i] = fmt.Sprintf(`System.Network.Config.Proxy.Servers[%s].Host: %s
System.Network.Config.Proxy.Servers[%s].Auth: %s
`, mapKey, newHost, mapKey, newAuth)

				if parsedProxy.User != nil {
					userPassword, _ := parsedProxy.User.Password()
					lines[i] += fmt.Sprintf(`System.Network.Config.Proxy.Servers[%s].Username: %s
System.Network.Config.Proxy.Servers[%s].Password: %s
`, mapKey, parsedProxy.User.Username(), mapKey, userPassword)
				}

				proxyRulePrefix := "http://"
				if strings.HasPrefix(line, "System.Network.Config.Proxy.HTTPSProxy") {
					proxyRulePrefix = "https://"
				}

				lines[i] += fmt.Sprintf(`System.Network.Config.Proxy.Rules[%d].Destination: %s*
System.Network.Config.Proxy.Rules[%d].Target: %s
`, currentRuleIndex, proxyRulePrefix, currentRuleIndex, mapKey)

				currentRuleIndex++
			}

			if strings.HasPrefix(line, "System.Network.Config.Proxy.NoProxy") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("malformed line '%s'", line)
				}

				newValue := strings.ReplaceAll(parts[1], ",", "|")

				lines[i] = fmt.Sprintf(`System.Network.Config.Proxy.Rules[%d].Destination: %s
System.Network.Config.Proxy.Rules[%d].Target: direct
`, currentRuleIndex, newValue, currentRuleIndex)

				currentRuleIndex++
			}
		}

		return lines, nil
	},
	// V3: Applications have fields moved under State struct.
	func(lines []string) ([]string, error) {
		for i, line := range lines {
			if strings.HasPrefix(line, "Applications[") {
				lines[i] = strings.Replace(lines[i], ".Initialized: ", ".State.Initialized: ", 1)
				lines[i] = strings.Replace(lines[i], ".Version: ", ".State.Version: ", 1)
			}
		}

		return lines, nil
	},
	// V4: Set default value for channel list.
	func(lines []string) ([]string, error) {
		for i := range lines {
			lines[i] = strings.Replace(lines[i], "System.Update.Config.UpdateFrequency: ", "System.Update.Config.CheckFrequency: ", 1)
			lines[i] = strings.Replace(lines[i], "System.Update.State.UpdateStatus: ", "System.Update.State.Status: ", 1)
		}

		lines = append(lines, "System.Update.Config.Channel: stable")

		return lines, nil
	},
}
