package main // main包是一个特殊的包
import (
	"fmt"  // 导入fmt包，fmt包实现了格式化输入输出功能
	"time" // 导入time包，time包提供了时间的功能
)

/*
- 导包
- main函数
- 注释
- 打印输出
*/

func main() {
	/* 简单的程序 万能的hello world */
	fmt.Println("Hello Go")

	time.Sleep(2 * time.Second) // 让程序睡眠2秒钟
}
