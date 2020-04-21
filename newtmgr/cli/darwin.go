// +build darwin

package cli

import (
	"github.com/JuulLabs-OSS/cbgo"
)

func OSSpecificInit() {
	cbgo.SetLogLevel(NewtmgrLogLevel)
}
