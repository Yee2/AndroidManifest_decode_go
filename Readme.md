# AndroidManifest decode

### 介绍
apkxmldecode 是一个使用Go语言编写的用来解析安卓安装包`AndroidManifest.xlm`的类库。

### 使用方法
````golang
package main

import (
	"archive/zip"
	"github.com/Yee2/apkxmldecode"
	"fmt"
	"os"
)

func main() {
	rd, err := zip.OpenReader("Stk.apk")
	checkErr(err)

	defer rd.Close()
	for _, file := range rd.File {
		if "AndroidManifest.xml" == file.Name {
			f, _ := file.Open()
			defer f.Close()
			result,err := apkxmldecode.New(f)
			checkErr(err)
			fmt.Println(result)
			break
		}
	}
}
func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
````
### 参考一下项目和文章：
 * [ Android逆向之旅---解析编译之后的AndroidManifest文件格式 ](http://blog.csdn.net/jiangwei0910410003/article/details/50568487)
 * [parse_androidxml](https://github.com/fourbrother/parse_androidxml)
 * [axmlprinter](https://github.com/rednaga/axmlprinter)