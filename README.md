# [8小时转go](https://www.bilibili.com/video/BV1gf4y1r79E)

## [学习笔记](https://www.yuque.com/aceld/mo95lb/dsk886)

### 从一个main函数初见golang语法

- 导包
- main函数
- 注释
- 打印输出

```go
  package main // main包是一个特殊的包
  import (
    "fmt"   // 导入fmt包，fmt包实现了格式化输入输出功能
    "time" // 导入time包，time包提供了时间的功能
  ) 

  func main() {
    /* 简单的程序 万能的hello world */
    fmt.Println("Hello Go")

    time.Sleep(2 * time.Second) // 让程序睡眠2秒钟
  }
```

## 初步设想

1. 先学习go的基础语法，并且记录学习笔记
2. 通过AI生成一些练习题目
3. 只基于笔记与刘丹冰书籍，生成练习题目，搭建一个网站（网站学习与考察golang知识点刷题学习网站）用于回顾与复习go知识点
4. 有添加笔记(此笔记会生成题目)功能

- 要求网站功能简单，易用
- 网站功能：用户注册、登录、刷题、查看错题、查看成绩
- 网站技术栈：前端使用Vue3，后端使用Gin框架，数据库使用MySQL
- 网站部署：使用Docker进行容器化部署，方便在不同环境下运行
