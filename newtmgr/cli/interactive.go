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
	"container/list"
	"fmt"
	"github.com/runtimeco/go-coap"
	"github.com/spf13/cobra"
	"gopkg.in/abiosoft/ishell.v1"
	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
	"strconv"
	"strings"
	"time"
)

var ObserverId int

type ObserveElem struct {
	Id         int
	Token      []byte
	Stopsignal chan int
	Path       string
}

var ResourcePath string
var ObserversList *list.List

func addObserver(path string, token []byte, stopsignal chan int) ObserveElem {
	o := ObserveElem{
		Id:         ObserverId,
		Token:      token,
		Stopsignal: stopsignal,
		Path:       path,
	}

	ObserverId++
	ObserversList.PushBack(o)
	return o
}

func notificationCb(path string, Code coap.COAPCode, value []byte, token []byte) {
	fmt.Println("Notification received:")
	fmt.Println("Code:", Code)
	fmt.Println("Token: [", token, "]")
	fmt.Printf("%s\n", resResponseStr(path, value))
	fmt.Println()
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

func getCmdCommon(c *ishell.Context, observe int, token []byte) error {

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	cmd := xact.NewGetResCmd()
	cmd.SetTxOptions(nmutil.TxOptions())
	cmd.Path = ResourcePath
	cmd.Observe = observe
	cmd.NotifyFunc = notificationCb
	cmd.StopSignal = make(chan int)
	cmd.Token = token

	res, err := cmd.Run(s)
	if err != nil {
		c.Println("Error:", err)
		return err
	}

	sres := res.(*xact.GetResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return err
	}

	if observe == 0 {
		o := addObserver(cmd.Path, sres.Token, cmd.StopSignal)
		c.Println("Observer added:")
		c.Println("id:", o.Id, "path:", o.Path, "token:", o.Token)
		c.Println()
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(cmd.Path, sres.Value))
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

	getCmdCommon(c, -1, nil)
}

func registerCmd(c *ishell.Context) {
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

	getCmdCommon(c, 0, nil)
}

func unregisterCmd(c *ishell.Context) {
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

	idstr, ok := m["id"].(string)
	if ok == false {
		c.Println(c.HelpText())
		return
	}

	id, err := strconv.Atoi(idstr)
	if err != nil {
		c.Println(c.HelpText())
		return
	}

	var e ObserveElem
	found := false
	elem := ObserversList.Front()
	for elem != nil && found == false {
		e, ok = (elem.Value).(ObserveElem)
		if ok && e.Id == id {
			found = true
		} else {
			elem = elem.Next()
		}
	}

	if found == false {
		c.Println("Observer id:", id, "not found")
		return
	}

	/* Stop listen. Sleep to allow to close existing listener before we send next GET request */
	e.Stopsignal <- 1
	time.Sleep(1)

	c.Println("Unegister for notifications")
	c.Println("id: ", e.Id)
	c.Println("path: ", e.Path)
	c.Println("token: ", e.Token)
	c.Println()

	err = getCmdCommon(c, 1, e.Token)
	if err == nil {
		ObserversList.Remove(elem)
	}
}

func getUriParams(c *ishell.Context) (map[string]interface{}, error) {

	c.ShowPrompt(false)
	defer c.ShowPrompt(true)

	c.Println("provide ", c.Cmd.Name, " parameters in format key=value [key=value]")
	pstr := c.ReadLine()
	params := strings.Split(pstr, " ")

	return extractResKv(params)
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

	m, err = getUriParams(c)
	if err != nil {
		c.Println(c.HelpText())
		return
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	b, err := nmxutil.EncodeCborMap(m)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	cmd := xact.NewPutResCmd()
	cmd.SetTxOptions(nmutil.TxOptions())
	cmd.Path = ResourcePath
	cmd.Value = b

	res, err := cmd.Run(s)
	if err != nil {
		c.Println("Error: ", err)
		return
	}

	sres := res.(*xact.PutResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(cmd.Path, sres.Value))
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

	m, err = getUriParams(c)
	if err != nil {
		c.Println(c.HelpText())
		return
	}

	b, err := nmxutil.EncodeCborMap(m)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	cmd := xact.NewPostResCmd()
	cmd.SetTxOptions(nmutil.TxOptions())
	cmd.Path = ResourcePath
	cmd.Value = b

	res, err := cmd.Run(s)
	if err != nil {
		c.Println("Error: ", err)
		return
	}

	sres := res.(*xact.PostResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(cmd.Path, sres.Value))
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

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	cmd := xact.NewDeleteResCmd()
	cmd.SetTxOptions(nmutil.TxOptions())
	cmd.Path = ResourcePath

	res, err := cmd.Run(s)
	if err != nil {
		c.Println("Error: ", err)
		return
	}

	sres := res.(*xact.DeleteResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(cmd.Path, sres.Value))
	}
}

func printObservers(c *ishell.Context) {

	elem := ObserversList.Front()
	for elem != nil {
		e, ok := (elem.Value).(ObserveElem)
		if ok {
			c.Println("id:", e.Id, ", path:", e.Path, ", token:", e.Token)
		}
		elem = elem.Next()
	}
}

func startInteractive(cmd *cobra.Command, args []string) {

	// create new shell.
	// by default, new shell includes 'exit', 'help' and 'clear' commands.
	shell := ishell.New()
	shell.SetPrompt("> ")

	ObserversList = list.New()

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
