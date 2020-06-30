# database
database/sql是golang标准库的数据库接口框架，类似于java的jdbc接口框架。本身数据库的访问需要各数据库驱动程序实现对应接口实现。
各数据库驱动程序通过向database/sql注册对应的数据库驱动，就能完成该数据库的操作。其中
+ sql包提供了普遍的sql或者类sql数据库的逻辑,里面不仅实现实现查询，执行等基本操作，而且还有连接池，事务，甚至提供进行驱动注册。
+ driver包则定义了用于sql包逻辑的驱动程序的接口，各数据库驱动程序需要根据自身数据库的特性去实现全部或者部分接口。

## 查询
[查询的相关分析](Query/readme.md),目前基于go1.13

### 样例
可以查看example_test.go的QueryMuch以及QueryOne

## 执行

[执行的相关分析](Exec/readme.md),目前基于go1.13

### 样例

可以查看example_test.go的Exec