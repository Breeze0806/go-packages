# Query
这里我们讲的是select等需要返回结果集的语句的执行接口，为了使得代码更具有可读性，我把原文中的注释也挪了进来，方便阅读。
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

| Q                                                    | A                                                            |
| ---------------------------------------------------- | ------------------------------------------------------------ |
| 什么！QueryContext还会起一个携程，他不是同步过程吗？ | 其实就启动了一个携程去监听ctx是否遇到结束，如果结束关闭rows  |
| Next仅仅是判断了判断有没有下一行吗?                  | 从上述的描述中就很好了解如果返回false，产生了两种可能性：1.你的程序很幸运地读完了所有数据，出色地完成了任务;2.你的程序不幸地遇到了一些问题，如网络故障，ctx遇到结束,甚至服务器故障。往往第二种可能性被忽略导致rows.Err()未被判定，导致数据丢失等莫名奇妙的问题的产生 |
| 什么！ctx遇到结束与rows还有关系？这不可能!           | 对于QueryContext而言其生命周期是一问一答，如果没有答完（rows未读取结束），就是其生命周期没完结，所以ctx包含了接收数据的流程 |

### 重点解析
+ rows.Close必须被调用
+ rows.Next返回false并非意味着读取结束，可能在读取中遇到了异常
+ rows.Scan并非扫描数据，而是适配数据类型并输出
+ rows.Err由于rows.Next可能在读取中遇到了异常，用于捕获该错误
## 源码解析

- [QueryContext的源码分析](QueryContext/readme.md),目前基于go1.13
- [Rows的源码分析]((Rows/readme.md)),目前基于go1.13