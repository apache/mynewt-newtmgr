BIN=$(DESTDIR)/usr/bin
TARGET=newtmgr
NEWTMGR_INSTALLDIR=$(PWD)
GODIR:=$(shell mktemp -d /tmp/mynewt.XXXXXXXXXXX)
MYNEWTDIR=$(GODIR)/src/mynewt.apache.org
REPODIR=$(MYNEWTDIR)/$(TARGET)
NEWTMGRDIR=$(REPODIR)/$(TARGET)
DSTFILE=$(NEWTMGR_INSTALLDIR)/$(TARGET)/$(TARGET)
build:
	mkdir -p $(MYNEWTDIR)
	ln -s $(NEWTMGR_INSTALLDIR) $(REPODIR)
	GOPATH=$(GODIR) go get github.com/currantlabs/ble github.com/mgutz/logxi/v1 golang.org/x/sys/unix
	cd $(NEWTMGRDIR) && GOPATH=$(GODIR) GO15VENDOREXPERIMENT=1 go install
	mv $(GODIR)/bin/$(TARGET) $(DSTFILE)
	rm -rf $(GODIR)

install:
	install -d $(BIN)
	install $(TARGET)/$(TARGET) $(BIN)
	rm -f  $(TARGET)/$(TARGET)
