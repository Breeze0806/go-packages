# Query
这里我们讲的是select等需要返回结果的语句的执行接口
## 高能预警
如果你使用不是上述类型语句去执行操作，那么Query可能会一直堵塞,而QueryContext也可能因为ctx没有被cancel而堵塞。
## 常用使用方式分析
```golang
    //QueryContext先向数据库发送sql语句包，再收到来自数据库的元数据包，然后将rows和对应数据库连接绑定,包含于ctx的绑定。
    rows, err := d.db.QueryContext(ctx, query)
    if err != nil {
        return fmt.Errorf("queryContext fail. error: %v", err)
    }
    //Close 这个操作至关重要，他会释放绑定的数据库连接，关闭启动的监听ctx结束的携程等等
    // 如果没有你会发现携程泄漏，内存泄漏，连接泄漏等等奇怪的场景
    defer rows.Close()
	
    // Next 仅仅判断有没有下一行? 事实上他其实默默做了很多工作
    // 最重要的是先尝试从网络从读取下一行数据，然后转换成能够Scan的数据
    for rows.Next() {
        int a;
        string s;
        // Scan 将当前行数据适配成对应类型输出给dest，并且将适配失败的错误返回
        if err = rows.Scan(&a,&s); err != nil {
            return fmt.Errorf("scan fail. error: %v", err)
        }
    }
    // Err 返回rows在上述过程中遇到的错误，和Next高度相关
    if err = rows.Err(); err != nil {
     	return fmt.Errorf("rows has error. error: %v", err)
    }
    return nil
```
### Q & A
看了上述注释，你可能会有以下一些问题：
Q: 什么！QueryContext还会起一个携程，他不是同步过程吗？
A: 其实就启动了一个携程去监听ctx是否遇到结束，如果结束关闭rows
Q: Next仅仅是判断了判断有没有下一行吗?
A: 从上述的描述中就很好了解如果返回false，产生了两种可能性：
+ 你的程序很幸运地读完了所有数据，出色地完成了任务。
+ 你的程序不幸地遇到了一些问题，如网络故障，ctx遇到结束,甚至服务器故障。
往往第二种可能性被忽略导致rows.Err()未被判定，导致数据丢失等莫名奇妙的问题的产生
Q: 什么！ctx遇到结束与rows还有关系？这不可能!
A: 对于QueryContext而言其生命周期是一问一答，如果没有答完（rows未读取结束），就是其生命周期没完结，所以ctx包含了接收数据的流程
### 重点解析
+ rows.Close必须被调用
+ rows.Next返回false并非意味着读取结束，可能在读取中遇到了异常
+ rows.Scan并非扫描数据，而是适配数据类型并输出
+ rows.Err由于rows.Next可能在读取中遇到了异常，用于捕获该错误
## 源码解析
### QueryContext

#### 流程说明
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


#### 查询相关
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
##### 驱动实现举例

###### 优先使用QueryerContext/Queryer
+ github.com/go-sql-driver/mysql
+ github.com/lib/pq
###### 只使用Conn/Stmt/StmtQueryContext
+ github.com/godror/godror，需要注意的是其使用了Prepare/QueryContext的方式实现了对应的库

##### args 参数相关
args可以通过NamedValueChecker以及ColumnConverter，给本次查询加入参数，单独对本次查询进行配置

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

###### 驱动实现举例
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

#### Rows

##### 相关接口

```golang
// Rows is an iterator over an executed query's results.
type Rows interface {
   // Columns returns the names of the columns. The number of
   // columns of the result is inferred from the length of the
   // slice. If a particular column name isn't known, an empty
   // string should be returned for that entry.
   Columns() []string

   // Close closes the rows iterator.
   Close() error

   // Next is called to populate the next row of data into
   // the provided slice. The provided slice will be the same
   // size as the Columns() are wide.
   //
   // Next should return io.EOF when there are no more rows.
   //
   // The dest should not be written to outside of Next. Care
   // should be taken when closing Rows not to modify
   // a buffer held in dest.
   Next(dest []Value) error
}

// RowsNextResultSet extends the Rows interface by providing a way to signal
// the driver to advance to the next result set.
type RowsNextResultSet interface {
	Rows

	// HasNextResultSet is called at the end of the current result set and
	// reports whether there is another result set after the current one.
	HasNextResultSet() bool

	// NextResultSet advances the driver to the next result set even
	// if there are remaining rows in the current result set.
	//
	// NextResultSet should return io.EOF when there are no more result sets.
	NextResultSet() error
}
```

##### Columns
Columns通过Rows的Columns() []string来获取列信息
##### Next

Next函数通过Rows的Next(dest []Value) error读取网络上的下一行数据，如果存在网路错误或者数据库错误，则需要通过Err返回错误并且返回false，然后通过RowsNextResultSet的HasNextResultSet() bool判断是否存在下一行数据。

##### Scan

该函数仅仅通过convertAssignRows进行类型调整，对于基本类型进行调整，不是从网络上直接扫描读取下一行数据。另外，用户可以通过Scanner的 Scan(src interface{}) error可以对已经得到的数据进行类型转化控制。

```golang
// Scanner is an interface used by Scan.
type Scanner interface {
   // Scan assigns a value from a database driver.
   //
   // The src value will be of one of the following types:
   //
   //    int64
   //    float64
   //    bool
   //    []byte
   //    string
   //    time.Time
   //    nil - for NULL values
   //
   // An error should be returned if the value cannot be stored
   // without loss of information.
   //
   // Reference types such as []byte are only valid until the next call to Scan
   // and should not be retained. Their underlying memory is owned by the driver.
   // If retention is necessary, copy their values before the next call to Scan.
   Scan(src interface{}) error
}
```

##### Err

较为重要的作用是获取Next中读取网络数据的错误以及context到期的错误。

##### Close

rows关闭，释放连接资源。