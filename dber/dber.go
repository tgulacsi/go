// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// Package dber is an interfaceization of database/sql.DB.
// It even implements a wrapper for *sql.DB: SqlDBer.
package dber

import (
	"database/sql"
	"io"
)

type DBer interface {
	Begin() (Txer, error)
	Queryer
	Execer
	io.Closer
}

type Execer interface {
	Exec(string, ...interface{}) (sql.Result, error)
}

type Txer interface {
	Commit() error
	Rollback() error
	Execer
	Queryer
}

type Queryer interface {
	Query(string, ...interface{}) (Rowser, error)
	QueryRow(string, ...interface{}) Scanner
}

type Scanner interface {
	Scan(dest ...interface{}) error
}

type Rowser interface {
	Close() error
	Err() error
	Next() bool
	Scanner
}

var _ = DBer(SqlDBer{})

// SqlDBer is a wrapper for *sql.DB to implement DBer
type SqlDBer struct {
	*sql.DB
}

func (db SqlDBer) Begin() (Txer, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return SqlTxer{tx}, nil
}

func (db SqlDBer) Query(qry string, args ...interface{}) (Rowser, error) {
	rows, err := db.DB.Query(qry, args...)
	return SqlRowser{rows}, err
}

func (db SqlDBer) QueryRow(qry string, args ...interface{}) Scanner {
	return SqlScanner{db.DB.QueryRow(qry, args...)}
}

// SqlTxer is a wrapper for *sql.Tx to implement Txer
type SqlTxer struct {
	*sql.Tx
}

func (tx SqlTxer) Query(qry string, args ...interface{}) (Rowser, error) {
	rows, err := tx.Tx.Query(qry, args...)
	return SqlRowser{rows}, err
}

func (tx SqlTxer) QueryRow(qry string, args ...interface{}) Scanner {
	return SqlScanner{tx.Tx.QueryRow(qry, args...)}
}

// SqlRowser is a wrapper for *sql.Rows to implement Rowser.
type SqlRowser struct {
	*sql.Rows
}

// SqlScanner is a wrapper for *sql.Row to implement Scanner.
type SqlScanner struct {
	*sql.Row
}
