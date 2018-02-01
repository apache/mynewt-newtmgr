package darwin

import (
	"github.com/raff/goble/xpc"
)

// xpc command IDs are OS X version specific, so we will use a map
// to be able to handle arbitrary versions
var (
	cmdInit,
	cmdAdvertiseStart,
	cmdAdvertiseStop,
	cmdScanningStart,
	cmdScanningStop,
	cmdServicesAdd,
	cmdServicesRemove,
	cmdSendData,
	cmdSubscribed,
	cmdConnect,
	cmdDisconnect,
	cmdReadRSSI,
	cmdDiscoverServices,
	cmdDiscoverIncludedServices,
	cmdDiscoverCharacteristics,
	cmdReadCharacteristic,
	cmdWriteCharacteristic,
	cmdSubscribeCharacteristic,
	cmdDiscoverDescriptors,
	cmdReadDescriptor,
	cmdWriteDescriptor,
	evtStateChanged,
	evtAdvertisingStarted,
	evtAdvertisingStopped,
	evtServiceAdded,
	evtReadRequest,
	evtWriteRequest,
	evtSubscribe,
	evtUnsubscribe,
	evtConfirmation,
	evtPeripheralDiscovered,
	evtPeripheralConnected,
	evtPeripheralDisconnected,
	evtATTMTU,
	evtRSSIRead,
	evtServiceDiscovered,
	evtIncludedServicesDiscovered,
	evtCharacteristicsDiscovered,
	evtCharacteristicRead,
	evtCharacteristicWritten,
	evtNotificationValueSet,
	evtDescriptorsDiscovered,
	evtDescriptorRead,
	evtDescriptorWritten,
	evtSlaveConnectionComplete,
	evtMasterConnectionComplete int
)

var serviceID string

func initXpcIDs() error {
	var utsname xpc.Utsname
	err := xpc.Uname(&utsname)
	if err != nil {
		return err
	}

	cmdInit = 1

	if utsname.Release < "17." {
		// yosemite
		cmdAdvertiseStart = 8
		cmdAdvertiseStop = 9
		cmdServicesAdd = 10
		cmdServicesRemove = 12

		cmdSendData = 13
		cmdSubscribed = 15
		cmdScanningStart = 29
		cmdScanningStop = 30
		cmdConnect = 31
		cmdDisconnect = 32
		cmdReadRSSI = 44
		cmdDiscoverServices = 45
		cmdDiscoverIncludedServices = 60
		cmdDiscoverCharacteristics = 62
		cmdReadCharacteristic = 65
		cmdWriteCharacteristic = 66
		cmdSubscribeCharacteristic = 68
		cmdDiscoverDescriptors = 70
		cmdReadDescriptor = 77
		cmdWriteDescriptor = 78

		evtStateChanged = 6
		evtAdvertisingStarted = 16
		evtAdvertisingStopped = 17
		evtServiceAdded = 18
		evtReadRequest = 19
		evtWriteRequest = 20
		evtSubscribe = 21
		evtUnsubscribe = 22
		evtConfirmation = 23
		evtPeripheralDiscovered = 37
		evtPeripheralConnected = 38
		evtPeripheralDisconnected = 40
		evtATTMTU = 53
		evtRSSIRead = 55
		evtServiceDiscovered = 56
		evtIncludedServicesDiscovered = 63
		evtCharacteristicsDiscovered = 64
		evtCharacteristicRead = 71
		evtCharacteristicWritten = 72
		evtNotificationValueSet = 74
		evtDescriptorsDiscovered = 76
		evtDescriptorRead = 79
		evtDescriptorWritten = 80
		evtSlaveConnectionComplete = 81
		evtMasterConnectionComplete = 82

		serviceID = "com.apple.blued"
	} else {
		// high sierra
		cmdSendData = 21
		cmdSubscribed = 22
		cmdAdvertiseStart = 16
		cmdAdvertiseStop = 17
		cmdServicesAdd = 18
		cmdServicesRemove = 19
		cmdScanningStart = 44
		cmdScanningStop = 45
		cmdConnect = 46
		cmdDisconnect = 47
		cmdReadRSSI = 61
		cmdDiscoverServices = 62
		cmdDiscoverIncludedServices = 74
		cmdDiscoverCharacteristics = 75
		cmdReadCharacteristic = 78
		cmdWriteCharacteristic = 79
		cmdSubscribeCharacteristic = 81
		cmdDiscoverDescriptors = 82
		cmdReadDescriptor = 88
		cmdWriteDescriptor = 89

		evtStateChanged = 4
		evtPeripheralDiscovered = 48
		evtPeripheralConnected = 49
		evtPeripheralDisconnected = 50
		evtRSSIRead = 71
		evtServiceDiscovered = 72
		evtCharacteristicsDiscovered = 77
		evtCharacteristicRead = 83
		evtCharacteristicWritten = 84
		evtNotificationValueSet = 86
		evtDescriptorsDiscovered = 87
		evtDescriptorRead = 90
		evtDescriptorWritten = 91
		evtAdvertisingStarted = 27
		evtAdvertisingStopped = 28
		evtServiceAdded = 29
		evtReadRequest = 30
		evtWriteRequest = 31
		evtSubscribe = 32
		evtUnsubscribe = 33
		evtConfirmation = 34
		evtATTMTU = 57
		evtSlaveConnectionComplete = 60 // should be called params update
		evtMasterConnectionComplete = 59 //not confident
		evtIncludedServicesDiscovered = 76

		serviceID = "com.apple.bluetoothd"
	}

	return nil
}
