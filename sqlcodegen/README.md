# SQLCodeGen

SQLCodeGen是[GoSQL](https://github.com/YiCodes/gosql)提供生成功能和提供用于描述数据模型的方法。

## 编写描述文件

### 创建account.go 文件

创建account文件夹，并创建account.go

在account.go中导入sqlcodegen

```account.go
import (
    "github.com/YiCodes/gosql/sqlcodegen"
)
```

在account.go中定义数据模型

```account.go
// 定义模型
type User struct {
    UserID   string
    UserName string
    Sex      byte
}
```

默认情况生成SQL语句中的数据表名与类型名称(User)相同，也可以指定不同的名字（user_info)

tableName 数据表名

```account.go
type User struct {
    sqlcodegen.TableName `tableName:"user_info"`
    UserID   string
}
// tableName 为数据表名
```

字段同样可以指定不同的名字

```account.go
type User struct {
    UserId int `name:"user_id" identity:"true"`
}

// name 为数据列名称
// identity 为自增列，若为true时，生成INSERT语句会忽略这个字段
```

### 在account.go中定义实体

```account.go
// 作于描述方法的参数使用。
var (
    user User
)
```

### INSERT 定义

```account.go
// InsertUser 插入一个用户
func InsertUser() {
    sqlcodegen.InsertAll(user)
}

// InsertAll 插入实体user的所有字段
```

### DELETE 定义

```account.go
// DeleteUser 删除一个user.UserId=userId并且user.Sex=0的用户
func DeleteUser(userID string) {
    sqlcodegen.Delete(user)
    sqlcodegen.Where(user.UserID == userID && user.Sex == 0)
}

// Delete 指定要删除的实体
// Where  删除条件
```

### UPDATE 定义

```account.go
// UpdateUser 更新user.UserId=userId的用户的UserName和Sex
func UpdateUser(userID string, userName string, sex byte) {
    sqlcodegen.From(user)
    sqlcodegen.Update(user.UserName, userName)
    sqlcodegen.Update(user.Sex, sex)
    sqlcodegen.Where(user.UserID == userID)
}

// From 指定要更新的实体
// Update 指定要更新的字段
// Where 更新条件
```

### SELECT 定义

```account.go
// GetUser 获取user.UserID=userID的一条用户
func GetUser(userID string) {
    sqlcodegen.From(user)
    // SELECT UserID, UserName, Sex
    sqlcodegen.SelectAll(user)
    sqlcodegen.Where(user.UserID == userID)
    sqlcodegen.SetReturnType(sqlcodegen.ReturnRecord)
}

// GetUserList 获取所有user.Sex=0 的用户, 未调用ReturnType，默认返回多条记录（数组）
func GetUserList() {
    sqlcodegen.From(user)
    // SELECT UserID, UserName
    sqlcodegen.Select(user.UserID, user.UserName)
    sqlcodegen.Where(user.Sex == 0)
}

func GetSortedUserList() {
    sqlcodegen.From(user)
    sqlcodegen.SelectAll(user)
    sqlcodegen.Where(user.Sex == 0)
    sqlcodegen.OrderBy(user.Sex)
    sqlcodegen.OrderByDescending(user.UserId)
}

// From 指定要查询的实体
// Where 查询条件
/* SetReturnType 设置返回值类型:
*    ReturnRecord（单条记录),
*    ReturnRecordSet(多条记录)，
*    ReturnScalar（单个值）
*/
// OrderBy 根据字段按照正序排序
// OrderByDescending 根据字段按照降序排序
```

## 生成代码

在命令行输入

```c.sh
gosql -in="account"
```

使用方法见 [GoSQL](https://github.com/YiCodes/gosql)