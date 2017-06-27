package bll

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/darwin"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

func exchangeMtu(cln ble.Client, preferredMtu int) (int, error) {
	log.Debugf("Exchanging MTU")

	// We loop three times here to workaround an library issue with macOS.  In
	// macOS, the request to exchange MTU doesn't actually do anything.  The
	// BLE library relies on the assumption that the OS already exchanged MTUs
	// on its own.  If this assumption is incorrect, the number that was
	// returned is the default out of date value (23).  In this case, sleep and
	// retry.
	var mtu int
	for i := 0; i < 3; i++ {
		var err error
		mtu, err = cln.ExchangeMTU(preferredMtu)
		if err != nil {
			return 0, err
		}

		// If this isn't macOS, or
		if _, ok := cln.(*darwin.Client); !ok {
			break
		}

		// If macOS returned a value other than 23, then MTU exchange has
		// completed.
		if mtu != bledefs.BLE_ATT_MTU_DFLT {
			break
		}

		// Otherwise, give the OS some time to perform the exchange.
		log.Debugf("macOS reports an MTU of 23.  " +
			"Assume exchange hasn't completed; wait and requery.")
		time.Sleep(time.Second)
	}

	log.Debugf("Exchanged MTU; ATT MTU = %d", mtu)
	return mtu, nil
}
