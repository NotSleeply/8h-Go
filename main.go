package main

import "fmt"

func main() {
	test2 := make(map[string]string)
	test2["one"] = "php"
	test2["two"] = "golang"
	test2["three"] = "java"
	for key, value := range test2 {
		fmt.Println(key, ":", value)
	}
}
