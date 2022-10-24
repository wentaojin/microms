/*
Copyright © 2020 Marvin

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"fmt"
	"net"

	"github.com/wentaojin/microms/engine"
	"github.com/wentaojin/microms/proto/common"
	"github.com/wentaojin/microms/proto/reverse"
	"google.golang.org/grpc"
)

type Reverse struct {
	reverse.UnimplementedReverseServer
}

func (r *Reverse) CreateTask(ctx context.Context, in *reverse.ReqCreateTask) (*reverse.RespCreateTask, error) {
	e, err := engine.NewDBEngine(ctx, in.OracleDBEngine, in.MySQLDBEngine, in.SQLiteDBEngine)
	if err != nil {
		return &reverse.RespCreateTask{
			TaskInfo: &common.TaskInfo{
				TaskUUID: in.GetTaskUUID(),
				TaskName: in.GetTaskName()},
			RespMsg: &common.ResponseMsg{
				RespCode: 1,
				RespMsg:  err.Error(),
			},
		}, err
	}

	_, res, err := engine.Query(e.OracleDB, `SELECT ID FROM MARVIN.MARVIN WHERE ROWNUM = 1`)
	if err != nil {
		return &reverse.RespCreateTask{
			TaskInfo: &common.TaskInfo{
				TaskUUID: in.GetTaskUUID(),
				TaskName: in.GetTaskName()},
			RespMsg: &common.ResponseMsg{
				RespCode: 1,
				RespMsg:  err.Error(),
			},
		}, err
	}

	return &reverse.RespCreateTask{
		TaskInfo: &common.TaskInfo{
			TaskUUID: in.GetTaskUUID(),
			TaskName: in.GetTaskName()},
		RespMsg: &common.ResponseMsg{
			RespCode: 0,
			RespMsg:  res[0]["ID"],
		},
	}, nil
}

func main() {
	RunServer()
}

func RunServer() {
	// 监听本地的8972端口
	lis, err := net.Listen("tcp", "127.0.01:8900")
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
		return
	}
	s := grpc.NewServer()                        // 创建gRPC服务器
	reverse.RegisterReverseServer(s, &Reverse{}) // 在gRPC服务端注册服务
	// 启动服务
	err = s.Serve(lis)
	if err != nil {
		fmt.Printf("failed to serve: %v", err)
		return
	}
}
