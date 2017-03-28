package config

import (
	"fmt"
	"strconv"
	"strings"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newt/util"
)

func einvalSerialConnString(f string, args ...interface{}) error {
	suffix := fmt.Sprintf(f, args)
	return util.FmtNewtError("Invalid serial connstring; %s", suffix)
}

func ParseSerialConnString(cs string) (*nmserial.XportCfg, error) {
	sc := nmserial.NewXportCfg()
	sc.Baud = 115200
	sc.ReadTimeout = nmutil.TxOptions().Timeout

	parts := strings.Split(cs, ",")
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		// Handle old-style conn string (single token indicating dev file).
		if len(kv) == 1 {
			kv = []string{"dev", kv[0]}
		}

		k := kv[0]
		v := kv[1]

		switch k {
		case "dev":
			sc.DevPath = v

		case "baud":
			var err error
			sc.Baud, err = strconv.Atoi(v)
			if err != nil {
				return sc, einvalSerialConnString("Invalid baud: %s", v)
			}

		default:
			return sc, einvalSerialConnString("Unrecognized key: %s", k)
		}
	}

	return sc, nil
}

func BuildSerialXport(sc *nmserial.XportCfg) (*nmserial.SerialXport, error) {
	sx := nmserial.NewSerialXport(sc)
	if err := sx.Start(); err != nil {
		return nil, util.ChildNewtError(err)
	}

	return sx, nil
}
