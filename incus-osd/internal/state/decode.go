package state

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
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
}

// Decode reconstitutes a given state. Optionally, if provided, a list of upgrade functions will be
// applied before decoding the state.
func Decode(b []byte, upgradeFuncs UpgradeFuncs, s *State) error {
	lines := strings.Split(string(b), "\n")

	// Check if we need to run any update logic.
	if strings.HasPrefix(lines[0], "#Version: ") {
		version, err := strconv.Atoi(strings.TrimPrefix(lines[0], "#Version: "))
		if err != nil {
			return err
		}

		// Record our starting version.
		s.StateVersion = version

		// If no custom upgrade functions are supplied, use the default list.
		if upgradeFuncs == nil {
			upgradeFuncs = upgrades
		}

		// Apply any needed upgrade functions to the input.
		for i := version; i < len(upgradeFuncs); i++ {
			if upgradeFuncs[i] != nil {
				lines, err = upgradeFuncs[i](lines)
				if err != nil {
					return err
				}

				// An upgrade may generate more than one new line of content, so we join
				// then resplit the lines after each upgrade function runs.
				lines = strings.Split(strings.Join(lines, "\n"), "\n")

				// Increment the state's version number.
				s.StateVersion = i + 1
			}
		}
	}

	// Parse each line.
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("malformed line '%s'", line)
		}

		err := decodeHelper(reflect.ValueOf(s), strings.Split(parts[0], "."), parts[1])
		if err != nil {
			return err
		}
	}

	return nil
}

// decodeHelper walks the state struct as defined by the provided keys and attempts to set the
// value once it reaches the end.
//
// Because maps are unaddressable, when decoding a map we recursively call ourselves with the new
// map as the root value, and once it is fully decoded then set the map as the new value and return.
func decodeHelper(v reflect.Value, keys []string, value string) error {
	// Walk the state struct to the appropriate location.
	for keyIndex, key := range keys {
		if reflect.Indirect(v).Kind() != reflect.Struct {
			return fmt.Errorf("unsupported kind '%s'", reflect.Indirect(v).Kind())
		}

		// Potentially split the current key if it is indexing an array or a map.
		parts := strings.Split(key, "[")
		if len(parts) == 2 {
			parts[1] = strings.TrimSuffix(parts[1], "]")
		}

		// Get the field from the current struct.
		field := reflect.Indirect(v).FieldByName(parts[0])

		if !field.IsValid() {
			return fmt.Errorf("invalid field '%s' for struct '%s'", key, v.Type())
		}

		// Do additional processing, if needed.
		switch field.Kind() { //nolint:exhaustive
		case reflect.Map:
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}

			mapField := field.MapIndex(reflect.ValueOf(parts[1]))
			if !mapField.IsValid() {
				mapField = reflect.New(field.Type().Elem()).Elem()
			} else {
				newMapField := reflect.New(field.Type().Elem()).Elem()
				newMapField.Set(mapField)
				mapField = newMapField
			}

			err := decodeHelper(mapField, keys[keyIndex+1:], value)
			if err != nil {
				return err
			}

			field.SetMapIndex(reflect.ValueOf(parts[1]), mapField)

			return nil
		case reflect.Pointer:
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
		case reflect.Slice:
			index, err := strconv.Atoi(parts[1])
			if err != nil {
				return err
			}

			if field.IsNil() {
				field.Set(reflect.MakeSlice(field.Type(), 0, 0))
			}

			for field.Len() <= index {
				t := field.Type().Elem()
				field.Set(reflect.Append(field, reflect.Zero(t)))
			}

			field = field.Index(index)
		default:
		}

		// Advance down one level into the state struct.
		v = field
	}

	// We've reached the end, and are ready to set the actual value.
	return setValue(v, value)
}

// setValue is a helper function to convert and set a string representation of a value.
func setValue(v reflect.Value, value string) error {
	// Set the value.
	switch v.Kind() { //nolint:exhaustive
	case reflect.Bool:
		bVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}

		v.SetBool(bVal)
	case reflect.Float32:
		fVal, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return err
		}

		v.SetFloat(fVal)
	case reflect.Float64:
		fVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}

		v.SetFloat(fVal)
	case reflect.Int:
		iVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		v.SetInt(iVal)
	case reflect.Int8:
		iVal, err := strconv.ParseInt(value, 10, 8)
		if err != nil {
			return err
		}

		v.SetInt(iVal)
	case reflect.Int16:
		iVal, err := strconv.ParseInt(value, 10, 16)
		if err != nil {
			return err
		}

		v.SetInt(iVal)
	case reflect.Int32:
		iVal, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return err
		}

		v.SetInt(iVal)
	case reflect.Int64:
		iVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		v.SetInt(iVal)
	case reflect.String:
		v.SetString(strings.ReplaceAll(value, "\\n", "\n"))
	case reflect.Uint:
		uVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}

		v.SetUint(uVal)
	case reflect.Uint8:
		uVal, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return err
		}

		v.SetUint(uVal)
	case reflect.Uint16:
		uVal, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return err
		}

		v.SetUint(uVal)
	case reflect.Uint32:
		uVal, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return err
		}

		v.SetUint(uVal)
	case reflect.Uint64:
		uVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}

		v.SetUint(uVal)
	default:
		return fmt.Errorf("unhandled kind '%s'", v.Kind())
	}

	return nil
}
