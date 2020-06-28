# Rows

主要用于返回query数据的句柄,用于读取网络数据并适配的封装。

## 相关接口

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

## Columns
Columns通过Rows的Columns() []string来获取列名
## Next
Next函数通过Rows的Next(dest []Value) error读取网络上的下一行数据，如果存在网路错误或者数据库错误，则需要通过Err返回错误并且返回false，然后通过RowsNextResultSet的HasNextResultSet() bool判断是否存在下一行数据。
## Scan
该函数仅仅通过convertAssignRows进行类型适配，对于基本类型进行适配，不是从网络上直接扫描读取下一行数据。另外，用户可以通过Scanner的 Scan(src interface{}) error可以对已经得到的数据进行类型适配控制。
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

## Err

较为重要的作用是获取Next中读取网络数据的错误以及context到期的错误。

## Close

rows关闭，先调用Rows的Close，将连接或者数据复位，再关闭stmt资源，最后释放和rows绑定的连接资源。