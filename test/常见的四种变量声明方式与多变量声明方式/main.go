package main

import "fmt"

func main() {

	println("方式一：使用var关键字声明变量 无初始值 默认值为0")
	var a int
	println("a= ", a)
	fmt.Printf("a的变量类型 %T\n", a)

	println("方式二：使用var关键字声明变量 有初始值")
	var b int = 10
	println("b= ", b)
	fmt.Printf("b的变量类型 %T\n", b)

	var bb string = "Hello Go"
	println("bb= ", bb)
	fmt.Printf("bb的变量类型 %T\n", bb)

	println("方式三:使用var 自动推导变量类型")
	var c = 20
	println("c= ", c)
	fmt.Printf("c的变量类型 %T\n", c)
	var cc = "Hello Golang"
	println("cc= ", cc)
	fmt.Printf("cc的变量类型 %T\n", cc)

	println("方式四:简短变量声明，只能在函数内部使用")
	d := 30
	println("d= ", d)
	fmt.Printf("d的变量类型 %T\n", d)
	dd := "Hello World"
	println("dd= ", dd)
	fmt.Printf("dd的变量类型 %T\n", dd)

	println("----------多变量声明----------")

	println("多变量声明单一类型")
	var x, y, z int = 1, 2, 3
	println("x=", x, "y=", y, "z=", z)
	fmt.Printf("x的变量类型 %T\n y的变量类型 %T\n z的变量类型 %T\n", x, y, z)

	println("多变量声明多种类型")
	var m, n, s = 10, 20.5, "Hello"
	println("m=", m, "n=", n, "s=", s)
	fmt.Printf("m的变量类型 %T\n n的变量类型 %T\n s的变量类型 %T\n", m, n, s)

	println("批量声明变量")
	var (
		i int = 40
		e     = 50
		f     = "Hello"
	)
	println("i=", i, "e=", e, "f=", f)
	fmt.Printf("i的变量类型 %T\n e的变量类型 %T\n f的变量类型 %T\n", i, e, f)

}
