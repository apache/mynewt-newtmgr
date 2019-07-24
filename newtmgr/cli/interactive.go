/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cli

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/runtimeco/go-coap"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"gopkg.in/abiosoft/ishell.v1"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

var ObserverId int

type observeElem struct {
	Id       int
	Path     string
	Listener *nmcoap.Listener
}

var ResourcePath string
var observerId int
var observers = map[int]*observeElem{}
var observerMtx sync.Mutex

func addObserver(path string, cl *nmcoap.Listener) *observeElem {
	observerMtx.Lock()
	defer observerMtx.Unlock()

	o := &observeElem{
		Id:       ObserverId,
		Path:     path,
		Listener: cl,
	}
	ObserverId++
	observers[o.Id] = o

	go func() {
		for {
			msg, err := sesn.RxCoap(o.Listener, 0)
			if err != nil {
				fmt.Printf("notification error: %s", err.Error())
				return
			}

			if msg == nil {
				// No longer listening.
				return
			}

			fmt.Println("Notification received:")
			fmt.Println("Code:", msg.Code())
			fmt.Println("Path:", o.Path)
			fmt.Println("Token: [", msg.Token(), "]")
			fmt.Printf("%s\n", resResponseStr(msg.PathString(), msg.Payload()))
			fmt.Println()
		}
	}()

	return o
}

func removeObserver(id int) *observeElem {
	observerMtx.Lock()
	defer observerMtx.Unlock()

	o := observers[id]
	if o != nil {
		delete(observers, id)
	}

	return o
}

func copyFromMap(m map[string]interface{}, key string) (value string) {

	if m[key] == nil {
		return ""
	}

	v, ok := m[key].(string)
	if ok {
		return v
	} else {
		return ""
	}
}

func hasStoredParams() bool {

	if strings.Compare(ResourcePath, "") == 0 {
		return false
	}

	return true
}

func getPath(m map[string]interface{}) {

	rpath := copyFromMap(m, "path")

	if strings.Compare(rpath, "") != 0 {
		ResourcePath = rpath
	}
}

func coapTxRx(c *ishell.Context, mp nmcoap.MsgParams) error {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	cmd := xact.NewResCmd()
	cmd.SetTxOptions(nmutil.TxOptions())
	cmd.MsgParams = mp

	res, err := cmd.Run(s)
	if err != nil {
		c.Println("Error:", err)
		return err
	}

	sres := res.(*xact.ResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n", sres.Rsp.Code(), sres.Status())
		return err
	}

	if len(sres.Rsp.Payload()) > 0 {
		fmt.Printf("%s\n",
			resResponseStr(mp.Uri, sres.Rsp.Payload()))
	}

	return nil
}

func getCmd(c *ishell.Context) {
	m, err := extractResKv(c.Args)
	if err != nil || len(c.Args) == 0 {
		c.Println("Incorrect or no parameters provided ... using cached ones")
	} else {
		getPath(m)
	}

	if hasStoredParams() == false {
		c.Println("Missing resource path")
		c.Println(c.HelpText())
		return
	}

	c.Println(m)

	c.Println("command: ", c.Cmd.Name)
	c.Println("path: ", ResourcePath)
	c.Println()

	mp := nmcoap.MsgParams{
		Code: coap.GET,
		Uri:  ResourcePath,
	}
	if err := coapTxRx(c, mp); err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
}

func registerCmd(c *ishell.Context) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	m, err := extractResKv(c.Args)
	if err != nil || len(c.Args) == 0 {
		c.Println("Incorrect or no parameters provided ... using cached ones")
	} else {
		getPath(m)
	}

	if hasStoredParams() == false {
		c.Println("Missing resource path")
		c.Println(c.HelpText())
		return
	}

	c.Println(m)

	c.Println("Register for notifications")
	c.Println("path: ", ResourcePath)
	c.Println()

	mc := nmcoap.MsgCriteria{Token: nmxutil.NextToken()}
	cl, err := s.ListenCoap(mc)
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		return
	}

	cmd := xact.NewResNoRxCmd()
	cmd.MsgParams = nmcoap.MsgParams{
		Code:    coap.GET,
		Uri:     ResourcePath,
		Observe: nmcoap.OBSERVE_START,
		Token:   mc.Token,
	}
	if _, err := cmd.Run(s); err != nil {
		fmt.Printf("error: %s\n", err.Error())
		return
	}

	if _, err := sesn.RxCoap(cl, nmutil.TxOptions().Timeout); err != nil {
		s.StopListenCoap(mc)
		fmt.Printf("error: %s\n", err.Error())
		return
	}

	o := addObserver(ResourcePath, cl)
	c.Println("Observer added:")
	c.Println("id:", o.Id, "path:", ResourcePath, "token:", mc.Token)
	c.Println()
}

func unregisterCmd(c *ishell.Context) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	m, err := extractResKv(c.Args)
	if err != nil || len(c.Args) == 0 {
		c.Println("Incorrect or no parameters provided ... using cached ones")
	} else {
		getPath(m)
	}

	if hasStoredParams() == false {
		c.Println("Missing resource path")
		c.Println(c.HelpText())
		return
	}

	if m["id"] == nil {
		c.Println(c.HelpText())
		return
	}

	id, err := cast.ToIntE(m["id"])
	if err != nil {
		c.Printf("Invalid ID: %v\n", m["id"])
		c.Println(c.HelpText())
		return
	}

	o := removeObserver(id)
	if o == nil {
		c.Println("Observer id:", id, "not found")
		return
	}
	s.StopListenCoap(o.Listener.Criteria)

	mp := nmcoap.MsgParams{
		Code:    coap.GET,
		Uri:     ResourcePath,
		Observe: nmcoap.OBSERVE_STOP,
	}
	if err := coapTxRx(c, mp); err != nil {
		fmt.Printf("error: %s\n", err.Error())
		return
	}

	c.Println("Unregister for notifications")
	c.Println("id: ", o.Id)
	c.Println("path: ", o.Path)
	c.Println("token: ", o.Listener.Criteria.Token)
	c.Println()
}

func getUriParams(c *ishell.Context) ([]byte, error) {

	c.ShowPrompt(false)
	defer c.ShowPrompt(true)

	c.Println("provide ", c.Cmd.Name, " parameters in format key=value [key=value]")
	pstr := c.ReadLine()
	params := strings.Split(pstr, " ")

	m, err := extractResKv(params)
	if err != nil {
		return nil, err
	}

	b, err := nmxutil.EncodeCborMap(m)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func putCmd(c *ishell.Context) {
	m, err := extractResKv(c.Args)
	if err != nil || len(c.Args) == 0 {
		c.Println("Incorrect or no parameters provided ... using cached ones")
	} else {
		getPath(m)
	}

	if hasStoredParams() == false {
		c.Println("Missing resource path")
		c.Println(c.HelpText())
		return
	}

	b, err := getUriParams(c)
	if err != nil {
		c.Println(c.HelpText())
		return
	}

	mp := nmcoap.MsgParams{
		Code:    coap.PUT,
		Uri:     ResourcePath,
		Payload: b,
	}
	if err := coapTxRx(c, mp); err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
}

func postCmd(c *ishell.Context) {
	m, err := extractResKv(c.Args)
	if err != nil || len(c.Args) == 0 {
		c.Println("Incorrect or no parameters provided ... using cached ones")
	} else {
		getPath(m)
	}

	if hasStoredParams() == false {
		c.Println("Missing resource path")
		c.Println(c.HelpText())
		return
	}

	b, err := getUriParams(c)
	if err != nil {
		c.Println(c.HelpText())
		return
	}

	mp := nmcoap.MsgParams{
		Code:    coap.POST,
		Uri:     ResourcePath,
		Payload: b,
	}
	if err := coapTxRx(c, mp); err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
}

func deleteCmd(c *ishell.Context) {
	m, err := extractResKv(c.Args)
	if err != nil || len(c.Args) == 0 {
		c.Println("Incorrect or no parameters provided ... using cached ones")
	} else {
		getPath(m)
	}

	if hasStoredParams() == false {
		c.Println("Missing resource path")
		c.Println(c.HelpText())
		return
	}

	mp := nmcoap.MsgParams{
		Code: coap.DELETE,
		Uri:  ResourcePath,
	}
	if err := coapTxRx(c, mp); err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
}

func printObservers(c *ishell.Context) {
	observerMtx.Lock()
	defer observerMtx.Unlock()

	var ids []int
	for id, _ := range observers {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		o := observers[id]
		c.Printf("id=%d path=%s token=%s\n",
			o.Id, o.Path, hex.EncodeToString(o.Listener.Criteria.Token))
	}
}

func startInteractive(cmd *cobra.Command, args []string) {

	// create new shell.
	// by default, new shell includes 'exit', 'help' and 'clear' commands.
	shell := ishell.New()
	shell.SetPrompt("> ")

	// display welcome info.
	shell.Println()
	shell.Println(" Newtmgr shell mode for COAP:")
	shell.Println("	Connection profile: ", nmutil.ConnProfile)
	shell.Println()

	shell.AddCmd(&ishell.Cmd{
		Name: "get",
		Help: "Send a CoAP GET request: get path=v",
		Func: getCmd,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "put",
		Help: "Send a CoAP PUT request: path=v <you will be asked for params>",
		Func: putCmd,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "post",
		Help: "Send a CoAP POST request: post path=v <you will be asked for params>",
		Func: postCmd,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "delete",
		Help: "Send a CoAP POST request: delete path=v",
		Func: deleteCmd,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "reg",
		Help: "Register for notifications: req path=v",
		Func: registerCmd,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "unreg",
		Help: "Unregister from notifications (id means observer id): unreq id=v",
		Func: unregisterCmd,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "observers",
		Help: "Print registered observers: observers",
		Func: printObservers,
	})

	shell.Run()
	shell.Close()
}

func interactiveCmd() *cobra.Command {

	shellCmd := &cobra.Command{
		Use: "interactive",
		Short: "Run " + nmutil.ToolInfo.ShortName +
			" interactive mode (used for COAP only)",
		Run: startInteractive,
	}

	return shellCmd
}
