/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package os

import (
	"bytes"
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	os_exec "os/exec"
	"strings"
	"syscall"
	"time"
)

type Executor struct {
}

func NewExecutor() spec.Executor {
	return &Executor{}
}

func (*Executor) Name() string {
	return "os"
}

var c = channel.NewLocalChannel()

const (
	OS_BIN  = "chaos_os"
	CREATE  = "create"
	DESTROY = "destroy"
)

func (e *Executor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	if model.ActionFlags[exec.ChannelFlag.Name] == "ssh" {
		sshExecutor := &exec.SSHExecutor{}
		return sshExecutor.Exec(uid, ctx, model)
	}

	var args string
	var flags string
	for k, v := range model.ActionFlags {
		if v == "" {
			continue
		}
		flags = fmt.Sprintf("%s --%s=%s", flags, k, v)
	}

	_, isDestroy := spec.IsDestroy(ctx)

	if isDestroy {
		args = fmt.Sprintf("%s %s %s%s uid=%s", DESTROY, model.Target, model.ActionName, flags, uid)
	} else {
		args = fmt.Sprintf("%s %s %s%s uid=%s", CREATE, model.Target, model.ActionName, flags, uid)
	}

	// todo nohup
	if model.Target == "disk" && model.ActionName == "fill" && model.ActionFlags["retain-handle"] == "true" {
		cl := channel.NewLocalChannel()
		response := cl.Run(ctx, "nohup", fmt.Sprintf("%s %s >> /dev/null 2>&1 &", util.GetChaosOsBin(), args))
		if !response.Success {
			return response
		}
		// check pid todo
		time.Sleep(1 * time.Second)
		ctx = context.WithValue(ctx, channel.ProcessKey, uid)
		pids, err := c.GetPidsByProcessName(OS_BIN, ctx)
		if len(pids) == 0 || err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, "create experiment failed, can't found chaos_os bin")
		}
		return spec.ReturnSuccess(uid)
	}
	argsArray := strings.Split(args, " ")
	command := os_exec.CommandContext(ctx, util.GetChaosOsBin(), argsArray...)

	if model.ActionProcessHang && !isDestroy {
		if err := command.Start(); err != nil {
			sprintf := fmt.Sprintf("create experiment failed, %v", err)
			return spec.ReturnFail(spec.OsCmdExecFailed, sprintf)
		}
		command.SysProcAttr = &syscall.SysProcAttr{}
		// check pid todo
		time.Sleep(1 * time.Second)
		ctx = context.WithValue(ctx, channel.ProcessKey, uid)
		pids, err := c.GetPidsByProcessName(OS_BIN, ctx)
		if len(pids) == 0 || err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, "create experiment failed, can't found chaos_os bin")
		}
		return spec.ReturnSuccess(uid)
	} else {
		buf := new(bytes.Buffer)
		command.Stdout = buf
		command.Stderr = buf

		if err := command.Start(); err != nil {
			sprintf := fmt.Sprintf("create experiment failed, %v", err)
			return spec.ReturnFail(spec.OsCmdExecFailed, sprintf)
		}

		if err := command.Wait(); err != nil {
			sprintf := fmt.Sprintf("create experiment failed, %v", err)
			return spec.ReturnFail(spec.OsCmdExecFailed, sprintf)
		}
		return spec.Decode(buf.String(), nil)
	}
}

func (*Executor) SetChannel(channel spec.Channel) {
}
