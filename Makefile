# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

# Use for building deb package. Needed by dpkg-buildpackage.
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
