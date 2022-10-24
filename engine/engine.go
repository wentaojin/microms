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
package engine

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/godror/godror"
	"github.com/godror/godror/dsn"
	"github.com/wentaojin/microms/logger"
	"github.com/wentaojin/microms/proto/common"
	"github.com/wentaojin/microms/utils"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const (
	// gorm 慢日志阈值
	slowQueryThreshold = 300
)

type Engine struct {
	OracleDB *sql.DB
	MySQLDB  *sql.DB
	GormDB   *gorm.DB
}

func NewDBEngine(ctx context.Context, oraCfg *common.OracleDBEngine, mysqlCfg *common.MySQLDBEngine, sqliteCfg *common.SQLiteDBEngine) (*Engine, error) {
	oraDB, err := newOracleDBEngine(ctx, oraCfg)
	if err != nil {
		return nil, err
	}
	mysqlDB, err := newMySQLDBEngine(ctx, mysqlCfg)
	if err != nil {
		return nil, err
	}
	gormDB, err := newSQLiteDBEngine(ctx, sqliteCfg)
	if err != nil {
		return nil, err
	}
	return &Engine{
		OracleDB: oraDB,
		MySQLDB:  mysqlDB,
		GormDB:   gormDB,
	}, nil
}

func newOracleDBEngine(ctx context.Context, oraCfg *common.OracleDBEngine) (*sql.DB, error) {
	// https://pkg.go.dev/github.com/godror/godror
	// https://github.com/godror/godror/blob/db9cd12d89cdc1c60758aa3f36ece36cf5a61814/doc/connection.md
	// https://godror.github.io/godror/doc/connection.html
	// You can specify connection timeout seconds with "?connect_timeout=15" - Ping uses this timeout, NOT the Deadline in Context!
	// For more connection options, see [Godor Connection Handling](https://godror.github.io/godror/doc/connection.html).
	var (
		connString string
		oraDSN     dsn.ConnectionParams
		err        error
	)

	switch {
	// CDB 架构，程序用户 c## 开头
	case strings.EqualFold(oraCfg.OracleArch, "CDB") && !strings.EqualFold(oraCfg.SchemaName, oraCfg.Username) &&
		strings.HasPrefix(strings.ToUpper(oraCfg.Username), "C##"):
		// 启用异构池 heterogeneousPool 即程序连接用户与访问 oracle schema 用户名不一致
		connString = fmt.Sprintf("oracle://@%s/%s?connectionClass=POOL_CONNECTION_CLASS&heterogeneousPool=1&%s",
			fmt.Sprintf("%s:%d", oraCfg.Host, oraCfg.Port), oraCfg.ServiceName, oraCfg.ConnectParams)
		oraDSN, err = godror.ParseDSN(connString)
		if err != nil {
			return nil, err
		}

		// https://blogs.oracle.com/opal/post/external-and-proxy-connection-syntax-examples-for-node-oracledb
		// Using 12.2 or later client libraries
		// 异构连接池
		oraDSN.Username, oraDSN.Password = utils.StringsBuilder(oraCfg.Username, "[", oraCfg.SchemaName, "]"), godror.NewPassword(oraCfg.Password)
		oraDSN.OnInitStmts = oraCfg.SessionParams

	default:
		connString = fmt.Sprintf("oracle://%s:%s@%s/%s?connectionClass=POOL_CONNECTION_CLASS&heterogeneousPool=1&%s",
			oraCfg.Username, oraCfg.Password, fmt.Sprintf("%s:%d", oraCfg.Host, oraCfg.Port), oraCfg.ServiceName, oraCfg.ConnectParams)
		oraDSN, err = godror.ParseDSN(connString)
		if err != nil {
			return nil, err
		}

		oraDSN.OnInitStmts = oraCfg.SessionParams
	}

	// libDir won't have any effect on Linux for linking reasons to do with Oracle's libnnz library that are proving to be intractable.
	// You must set LD_LIBRARY_PATH or run ldconfig before your process starts.
	// This is documented in various places for other drivers that use ODPI-C. The parameter works on macOS and Windows.
	switch runtime.GOOS {
	case "linux":
		if err = os.Setenv("LD_LIBRARY_PATH", oraCfg.LibDir); err != nil {
			return nil, fmt.Errorf("set LD_LIBRARY_PATH env failed: %v", err)
		}
	case "windows", "darwin":
		oraDSN.LibDir = oraCfg.LibDir
	}

	// godror logger 日志输出
	// godror.SetLogger(zapr.NewLogger(zap.L()))

	sqlDB := sql.OpenDB(godror.NewConnector(oraDSN))
	sqlDB.SetMaxIdleConns(0)
	sqlDB.SetMaxOpenConns(0)
	sqlDB.SetConnMaxLifetime(0)

	err = sqlDB.Ping()
	if err != nil {
		return sqlDB, fmt.Errorf("error on ping oracle database connection:%v", err)
	}
	return sqlDB, nil
}

func newMySQLDBEngine(ctx context.Context, mysqlCfg *common.MySQLDBEngine) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		mysqlCfg.Username, mysqlCfg.Password, mysqlCfg.Host, mysqlCfg.Port, mysqlCfg.SchemaName, mysqlCfg.ConnectParams)

	mysqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error on open mysql database connection [target-schema]: %v", err)
	}

	mysqlDB.SetMaxIdleConns(512)
	mysqlDB.SetMaxOpenConns(1024)
	mysqlDB.SetConnMaxLifetime(300 * time.Second)
	mysqlDB.SetConnMaxIdleTime(200 * time.Second)

	if err = mysqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("error on ping mysql database connection [target-schema]: %v", err)
	}

	return mysqlDB, nil
}

func newSQLiteDBEngine(ctx context.Context, sqliteCfg *common.SQLiteDBEngine) (*gorm.DB, error) {
	// 初始化 gormDB
	// 初始化 gorm 日志记录器
	logger := logger.NewGormLogger(zap.L(), slowQueryThreshold)
	logger.SetAsDefault()
	sqliteDB, err := gorm.Open(sqlite.Open(sqliteCfg.DBPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		PrepareStmt:                              true,
		Logger:                                   logger,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 使用单数表名
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error on open sqlite database connection [meta-schema]: %v", err)
	}

	return sqliteDB, nil
}

// 查询返回表字段列和对应的字段行数据
func Query(db *sql.DB, querySQL string) ([]string, []map[string]string, error) {
	var (
		cols []string
		res  []map[string]string
	)
	rows, err := db.Query(querySQL)
	if err != nil {
		return cols, res, fmt.Errorf("general sql [%v] query failed: [%v]", querySQL, err.Error())
	}
	defer rows.Close()

	//不确定字段通用查询，自动获取字段名称
	cols, err = rows.Columns()
	if err != nil {
		return cols, res, fmt.Errorf("general sql [%v] query rows.Columns failed: [%v]", querySQL, err.Error())
	}

	values := make([][]byte, len(cols))
	scans := make([]interface{}, len(cols))
	for i := range values {
		scans[i] = &values[i]
	}

	for rows.Next() {
		err = rows.Scan(scans...)
		if err != nil {
			return cols, res, fmt.Errorf("general sql [%v] query rows.Scan failed: [%v]", querySQL, err.Error())
		}

		row := make(map[string]string)
		for k, v := range values {
			// Oracle/Mysql 对于 'NULL' 统一字符 NULL 处理，查询出来转成 NULL,所以需要判断处理
			// 查询字段值 NULL
			// 如果字段值 = NULLABLE 则表示值是 NULL
			// 如果字段值 = "" 则表示值是空字符串
			// 如果字段值 = 'NULL' 则表示值是 NULL 字符串
			// 如果字段值 = 'null' 则表示值是 null 字符串
			if v == nil {
				row[cols[k]] = "NULLABLE"
			} else {
				// 处理空字符串以及其他值情况
				// 数据统一 string 格式显示
				row[cols[k]] = string(v)
			}
		}
		res = append(res, row)
	}

	if err = rows.Err(); err != nil {
		return cols, res, fmt.Errorf("general sql [%v] query rows.Next failed: [%v]", querySQL, err.Error())
	}
	return cols, res, nil
}
