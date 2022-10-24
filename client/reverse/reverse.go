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
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/wentaojin/microms/proto/common"
	"github.com/wentaojin/microms/proto/reverse"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	RunClient()
}

func RunClient() {
	conn, err := grpc.Dial("127.0.0.1:8900", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	c := reverse.NewReverseClient(conn)

	// 执行RPC调用并打印收到的响应数据
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.CreateTask(ctx, &reverse.ReqCreateTask{
		TaskUUID: ulid.Make().String(),
		TaskName: "Test",
		OracleDBEngine: &common.OracleDBEngine{
			OracleArch:    "noncdb",
			Username:      "marvin",
			Password:      "marvin",
			Host:          "10.2.103.31",
			Port:          1521,
			ServiceName:   "orclpdb1",
			LibDir:        "/Users/marvin/storehouse/oracle/instantclient_19_8",
			ConnectParams: "poolMinSessions=50&poolMaxSessions=1000&poolWaitTimeout=360s&poolSessionMaxLifetime=2h&poolSessionTimeout=2h&poolIncrement=30&timezone=Local&connect_timeout=15",
			SessionParams: []string{},
			SchemaName:    "marvin",
			IncludeTables: []string{"marvin"},
			ExcludeTables: []string{},
		},
		MySQLDBEngine: &common.MySQLDBEngine{
			DBType:        "tidb",
			Username:      "root",
			Password:      "",
			Host:          "10.2.103.30",
			Port:          4000,
			ConnectParams: "charset=utf8mb4&multiStatements=true&parseTime=True&loc=Local",
			SchemaName:    "marvin",
		},
		SQLiteDBEngine: &common.SQLiteDBEngine{
			DBPath: "/Users/marvin/gostore/microms/sqlite/microms.db",
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("success: %v\n", r.GetRespMsg())
}
