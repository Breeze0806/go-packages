# QueryContext

## 流程说明
1. 从db连接中获取连接一个Conn：db.conn
2. 尝试该链接是否实现QueryerContext或者Queryer，转第3步，否则转第7步
3. 如果实现QueryerContext或者Queryer
4. driverArgsConnLocked进行args参数转换，将sql语句参数和其他参数分离
5. ctxDriverQuery进行查询返回结果rows
6. initContextClose开启携程awaitDone监听ctx是否结束，之后结束
7. driverArgsConnLocked进行args参数转换，将sql语句参数和其他参数分离
8. ctxDriverPrepare进行Prepare(query string) (Stmt, error)
9. rowsiFromStatement进行查询返回结果rows
10. 开启携程awaitDone监听ctx是否结束，之后结束


## 查询相关
```golang
// QueryerContext is an optional interface that may be implemented by a Conn.
//
// If a Conn does not implement QueryerContext, the sql package's DB.Query
// will fall back to Queryer; if the Conn does not implement Queryer either,
// DB.Query will first prepare a query, execute the statement, and then
// close the statement.
//
// QueryerContext may return ErrSkip.
//
// QueryerContext must honor the context timeout and return when the context is canceled.
type QueryerContext interface {
	QueryContext(ctx context.Context, query string, args []NamedValue) (Rows, error)
}

// Queryer is an optional interface that may be implemented by a Conn.
//
// If a Conn implements neither QueryerContext nor Queryer,
// the sql package's DB.Query will first prepare a query, execute the statement,
// and then close the statement.
//
// Query may return ErrSkip.
//
// Deprecated: Drivers should implement QueryerContext instead.
type Queryer interface {
	Query(query string, args []Value) (Rows, error)
}

// Conn is a connection to a database. It is not used concurrently
// by multiple goroutines.
//
// Conn is assumed to be stateful.
type Conn interface {
	// Prepare returns a prepared statement, bound to this connection.
	Prepare(query string) (Stmt, error)

	// Close invalidates and potentially stops any current
	// prepared statements and transactions, marking this
	// connection as no longer in use.
	//
	// Because the sql package maintains a free pool of
	// connections and only calls Close when there's a surplus of
	// idle connections, it shouldn't be necessary for drivers to
	// do their own connection caching.
	Close() error

	// Begin starts and returns a new transaction.
	//
	// Deprecated: Drivers should implement ConnBeginTx instead (or additionally).
	Begin() (Tx, error)
}

// Stmt is a prepared statement. It is bound to a Conn and not
// used by multiple goroutines concurrently.
type Stmt interface {
	// Close closes the statement.
	//
	// As of Go 1.1, a Stmt will not be closed if it's in use
	// by any queries.
	Close() error

	// NumInput returns the number of placeholder parameters.
	//
	// If NumInput returns >= 0, the sql package will sanity check
	// argument counts from callers and return errors to the caller
	// before the statement's Exec or Query methods are called.
	//
	// NumInput may also return -1, if the driver doesn't know
	// its number of placeholders. In that case, the sql package
	// will not sanity check Exec or Query argument counts.
	NumInput() int

	// Exec executes a query that doesn't return rows, such
	// as an INSERT or UPDATE.
	//
	// Deprecated: Drivers should implement StmtExecContext instead (or additionally).
	Exec(args []Value) (Result, error)

	// Query executes a query that may return rows, such as a
	// SELECT.
	//
	// Deprecated: Drivers should implement StmtQueryContext instead (or additionally).
	Query(args []Value) (Rows, error)
}

// StmtQueryContext enhances the Stmt interface by providing Query with context.
type StmtQueryContext interface {
	// QueryContext executes a query that may return rows, such as a
	// SELECT.
	//
	// QueryContext must honor the context timeout and return when it is canceled.
	QueryContext(ctx context.Context, args []NamedValue) (Rows, error)
}
```
数据库驱动库都会实现Conn/Stmt/StmtQueryContext，大多数驱动库同时Queryer/QueryerContext
### 驱动实现举例

#### 优先使用QueryerContext/Queryer
+ github.com/go-sql-driver/mysql
+ github.com/lib/pq
#### 只使用Conn/Stmt/StmtQueryContext
+ github.com/godror/godror，需要注意的是其使用了Prepare/QueryContext的方式实现了对应的库

### args 参数相关
1. 可以通过NamedValueChecker，给本次查询加入参数，单独对本次查询进行配置，使得单次查询能够单独配置参数。
2. 通过ColumnConverter对占位符进行合理的类型转化？这里所属的观点仅仅是一个猜测，不是非常确切，是通过github.com/go-sql-driver/mysql的使用做出的部分推断。

```golang
// NamedValueChecker may be optionally implemented by Conn or Stmt. It provides
// the driver more control to handle Go and database types beyond the default
// Values types allowed.
//
// The sql package checks for value checkers in the following order,
// stopping at the first found match: Stmt.NamedValueChecker, Conn.NamedValueChecker,
// Stmt.ColumnConverter, DefaultParameterConverter.
//
// If CheckNamedValue returns ErrRemoveArgument, the NamedValue will not be included in
// the final query arguments. This may be used to pass special options to
// the query itself.
//
// If ErrSkip is returned the column converter error checking
// path is used for the argument. Drivers may wish to return ErrSkip after
// they have exhausted their own special cases.
type NamedValueChecker interface {
	// CheckNamedValue is called before passing arguments to the driver
	// and is called in place of any ColumnConverter. CheckNamedValue must do type
	// validation and conversion as appropriate for the driver.
	CheckNamedValue(*NamedValue) error
}

// ColumnConverter may be optionally implemented by Stmt if the
// statement is aware of its own columns' types and can convert from
// any type to a driver Value.
//
// Deprecated: Drivers should implement NamedValueChecker.
type ColumnConverter interface {
	// ColumnConverter returns a ValueConverter for the provided
	// column index. If the type of a specific column isn't known
	// or shouldn't be handled specially, DefaultValueConverter
	// can be returned.
	ColumnConverter(idx int) ValueConverter
}

// ValueConverter is the interface providing the ConvertValue method.
//
// Various implementations of ValueConverter are provided by the
// driver package to provide consistent implementations of conversions
// between drivers. The ValueConverters have several uses:
//
//  * converting from the Value types as provided by the sql package
//    into a database table's specific column type and making sure it
//    fits, such as making sure a particular int64 fits in a
//    table's uint16 column.
//
//  * converting a value as given from the database into one of the
//    driver Value types.
//
//  * by the sql package, for converting from a driver's Value type
//    to a user's type in a scan.
type ValueConverter interface {
	// ConvertValue converts a value to a driver Value.
	ConvertValue(v interface{}) (Value, error)
}
```

### 驱动实现举例
github.com/godror/godror通过NamedValueChecker对本次查询单独加入参数
```golang
// CheckNamedValue is called before passing arguments to the driver
// and is called in place of any ColumnConverter. CheckNamedValue must do type
// validation and conversion as appropriate for the driver.
//
// If CheckNamedValue returns ErrRemoveArgument, the NamedValue will not be included
// in the final query arguments.
// This may be used to pass special options to the query itself.
//
// If ErrSkip is returned the column converter error checking path is used
// for the argument.
// Drivers may wish to return ErrSkip after they have exhausted their own special cases.
func (st *statement) CheckNamedValue(nv *driver.NamedValue) error {
	if nv == nil {
		return nil
	}
	if apply, ok := nv.Value.(Option); ok {
		if apply != nil {
			apply(&st.stmtOptions)
		}
		return driver.ErrRemoveArgument
	}
	return nil
}
```