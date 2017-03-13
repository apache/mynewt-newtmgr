package xact

import (
	"fmt"
	"sort"

	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type CrashType int

const (
	CRASH_TYPE_DIV0 CrashType = iota
	CRASH_TYPE_JUMP0
	CRASH_TYPE_REF0
	CRASH_TYPE_ASSERT
	CRASH_TYPE_WDOG
)

var CrashTypeNameMap = map[CrashType]string{
	CRASH_TYPE_DIV0:   "div0",
	CRASH_TYPE_JUMP0:  "jump0",
	CRASH_TYPE_REF0:   "ref0",
	CRASH_TYPE_ASSERT: "assert",
	CRASH_TYPE_WDOG:   "wdog",
}

func CrashTypeToString(ct CrashType) string {
	return CrashTypeNameMap[ct]
}

func CrashTypeFromString(s string) (CrashType, error) {
	for k, v := range CrashTypeNameMap {
		if s == v {
			return k, nil
		}
	}

	return CrashType(0), fmt.Errorf("invalid crash type: %s", s)
}

func CrashTypeNames() []string {
	names := make([]string, 0, len(CrashTypeNameMap))
	for _, v := range CrashTypeNameMap {
		names = append(names, v)
	}

	sort.Strings(names)
	return names
}

type CrashCmd struct {
	CmdBase
	CrashType CrashType
}

func NewCrashCmd() *CrashCmd {
	return &CrashCmd{}
}

type CrashResult struct {
	Rsp *nmp.CrashRsp
}

func newCrashResult() *CrashResult {
	return &CrashResult{}
}

func (r *CrashResult) Status() int {
	return r.Rsp.Rc
}

func (c *CrashCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCrashReq()
	r.CrashType = CrashTypeToString(c.CrashType)

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CrashRsp)

	res := newCrashResult()
	res.Rsp = srsp
	return res, nil
}
