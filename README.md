# GoSQL

GoSQL 是以Go语言为数据库模型描述语言，生成强类型化的Go源代码的工具

## 安装

使用go工具安装，GoSQL默认安装在$GOPATH

```cmd
go get github.com/YiCodes/gosql/gosql
go get github.com/YiCodes/gosql/sqlutil
```

## 使用方法

在account.go定义数据模型和操作文件，定义见[SQLCodeGen](https://github.com/YiCodes/gosql/tree/master/sqlcodegen)

```account.go
package account

import (
    "github.com/YiCodes/gosql/sqlcodegen"
)

// 定义模型
type User struct {
    UserID   string
    UserName string
    Sex      byte
}

// 定义实体
var (
    user User
)

// GetUser 获取user.UserID=userID的一条用户
func GetUser(userID string) {
    sqlcodegen.From(user)
    // select User的所有字段
    sqlcodegen.SelectAll(user)
    sqlcodegen.Where(user.UserID == userID)
    sqlcodegen.SetReturnType(sqlcodegen.ReturnRecord)
}

// GetUserList 获取所有user.Sex=0 的用户, 未调用ReturnType，默认返回多条记录（数组）
func GetUserList() {
    sqlcodegen.From(user)
    //指定select User的几个字段
    sqlcodegen.Select(user.UserID, user.UserName)
    sqlcodegen.Where(user.Sex == 0)
}

// InsertUser 插入一个用户
func InsertUser() {
    sqlcodegen.InsertAll(user)
}

// UpdateUser 更新user.UserId=userId的用户的UserName和Sex
func UpdateUser(userID string, userName string, sex byte) {
    sqlcodegen.From(user)
    sqlcodegen.Update(user.UserName, userName)
    sqlcodegen.Update(user.Sex, sex)
    sqlcodegen.Where(user.UserID == userID)
}

// DeleteUser 删除一个user.UserId=userId并且user.Sex=0的用户
func DeleteUser(userID string) {
    sqlcodegen.Delete(user)
    sqlcodegen.Where(user.UserID == userID && user.Sex == 0)
}
```

### 生成代码

在命令行输入

```cmd
gosql -in="account"
```

在in参数指定account目录，默认会在account目录下创建一个gen文件夹，并在其中生成的同名文件account.go。

也可以使用out参数，指定输出的目录。

### 使用生成的代码

在实际代码中引入account/gen文件夹。

```main.go
package main

import (
    "database/sql"
    "fmt"

    account "account/gen"
)

func main() {
    db, err := sql.Open("driver", "dataSourceName")

    if err != nil {
        panic(err)
    }
    defer db.Close()

    user, err := account.GetUser(db, "123")

    if err != nil {
        panic(err)
    }

    fmt.Println(user.UserName)

    user = new(account.User)
    user.UserId = "222"
    user.UserName = "peter"

    _, err = account.InsertUser(db, user)
}
```
